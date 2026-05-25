// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Release contains the SemRel release data consumed by this plugin.
type Release struct {
	Version         string
	PreviousVersion string
	TagName         string
	Repository      string
	Changelog       string
	CommitSHA       string
	DryRun          bool
	Metadata        map[string]string
	Commits         []string
}

// Result captures the outcome of a plugin execution.
type Result struct {
	Name       string
	Outputs    map[string]string
	Skipped    bool
	SkipReason string
}

// Provider is the contract exposed by this plugin implementation.
type Provider interface {
	Name() string
	HealthCheck(context.Context) error
	Validate(map[string]interface{}) error
	Execute(context.Context, *Release) (*Result, error)
	ReleaseContext() []string
}

var issuePattern = regexp.MustCompile(`(?i)\b(?:fix(?:es|ed)?|close[sd]?)\s+([A-Z][A-Z0-9]+-\d+)\b`)

// JiraHook transitions release issues and leaves a release comment.
type JiraHook struct {
	BaseURL  string
	Email    string
	APIToken string
	Client   *http.Client
}

// NewJiraHook constructs a Jira hook with explicit configuration.
func NewJiraHook(client *http.Client, baseURL, email, apiToken string) *JiraHook {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &JiraHook{BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Email: strings.TrimSpace(email), APIToken: strings.TrimSpace(apiToken), Client: client}
}

// NewJiraHookFromEnv constructs a Jira hook from environment variables.
func NewJiraHookFromEnv() *JiraHook {
	return NewJiraHook(nil, os.Getenv("JIRA_BASE_URL"), os.Getenv("JIRA_EMAIL"), os.Getenv("JIRA_API_TOKEN"))
}

func (j *JiraHook) Name() string { return "hook-jira" }

func (j *JiraHook) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (j *JiraHook) Validate(map[string]interface{}) error {
	switch {
	case j.BaseURL == "":
		return fmt.Errorf("jira: JIRA_BASE_URL is required")
	case j.Email == "":
		return fmt.Errorf("jira: JIRA_EMAIL is required")
	case j.APIToken == "":
		return fmt.Errorf("jira: JIRA_API_TOKEN is required")
	}
	return nil
}

func (j *JiraHook) ReleaseContext() []string {
	return []string{"version", "repository", "commits"}
}

func (j *JiraHook) Execute(ctx context.Context, rel *Release) (*Result, error) {
	if err := j.HealthCheck(ctx); err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("jira: release is required")
	}
	issues := ExtractIssueKeys(rel.Commits)
	if len(issues) == 0 {
		return &Result{Name: j.Name(), Skipped: true, SkipReason: "no Jira issue keys found in release commits"}, nil
	}
	if rel.DryRun {
		return &Result{Name: j.Name(), Outputs: map[string]string{"issues": strings.Join(issues, ","), "dry_run": "true"}}, nil
	}
	if err := j.Validate(nil); err != nil {
		return nil, err
	}

	comment := BuildReleaseComment(rel)
	for _, issueKey := range issues {
		transitionID, err := j.findPreferredTransition(ctx, issueKey)
		if err != nil {
			return nil, err
		}
		if err := j.transitionIssue(ctx, issueKey, transitionID); err != nil {
			return nil, err
		}
		if err := j.addComment(ctx, issueKey, comment); err != nil {
			return nil, err
		}
	}

	return &Result{Name: j.Name(), Outputs: map[string]string{"issues": strings.Join(issues, ",")}}, nil
}

// ExtractIssueKeys finds Jira issue keys in release commit messages.
func ExtractIssueKeys(commits []string) []string {
	seen := map[string]struct{}{}
	var issues []string
	for _, commit := range commits {
		matches := issuePattern.FindAllStringSubmatch(commit, -1)
		for _, match := range matches {
			issue := strings.ToUpper(match[1])
			if _, exists := seen[issue]; exists {
				continue
			}
			seen[issue] = struct{}{}
			issues = append(issues, issue)
		}
	}
	sort.Strings(issues)
	return issues
}

// BuildReleaseComment renders the Jira comment body.
func BuildReleaseComment(rel *Release) string {
	version := strings.TrimSpace(rel.Version)
	if version == "" {
		version = "unknown"
	}
	tag := strings.TrimSpace(rel.TagName)
	if tag == "" {
		tag = version
	}
	comment := "Released in " + version + " (tag " + tag + ")"
	if repo := strings.TrimSpace(rel.Repository); repo != "" {
		comment += " for " + repo
	}
	return comment + "."
}

func (j *JiraHook) findPreferredTransition(ctx context.Context, issueKey string) (string, error) {
	endpoint := j.BaseURL + "/rest/api/2/issue/" + issueKey + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("jira: build transition lookup request: %w", err)
	}
	req.SetBasicAuth(j.Email, j.APIToken)

	resp, err := j.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: get transitions for %s: %w", issueKey, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("jira: transition lookup for %s returned %s", issueKey, resp.Status)
	}

	var payload struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("jira: decode transitions for %s: %w", issueKey, err)
	}

	for _, preferred := range []string{"Released", "Done"} {
		for _, transition := range payload.Transitions {
			if strings.EqualFold(strings.TrimSpace(transition.Name), preferred) {
				return transition.ID, nil
			}
		}
	}
	return "", fmt.Errorf("jira: no Released or Done transition found for %s", issueKey)
}

func (j *JiraHook) transitionIssue(ctx context.Context, issueKey, transitionID string) error {
	payload := map[string]map[string]string{"transition": {"id": transitionID}}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshal transition payload: %w", err)
	}
	endpoint := j.BaseURL + "/rest/api/2/issue/" + issueKey + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: build transition request: %w", err)
	}
	req.SetBasicAuth(j.Email, j.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.Client.Do(req)
	if err != nil {
		return fmt.Errorf("jira: transition %s: %w", issueKey, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jira: transition %s returned %s", issueKey, resp.Status)
	}
	return nil
}

func (j *JiraHook) addComment(ctx context.Context, issueKey, comment string) error {
	body, err := json.Marshal(map[string]string{"body": comment})
	if err != nil {
		return fmt.Errorf("jira: marshal comment payload: %w", err)
	}
	endpoint := j.BaseURL + "/rest/api/2/issue/" + issueKey + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: build comment request: %w", err)
	}
	req.SetBasicAuth(j.Email, j.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.Client.Do(req)
	if err != nil {
		return fmt.Errorf("jira: comment on %s: %w", issueKey, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jira: comment on %s returned %s", issueKey, resp.Status)
	}
	return nil
}
