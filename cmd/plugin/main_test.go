package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	plugin "github.com/SemRels/hook-jira/internal/plugin"
)

type fakeClient struct {
	createdProject string
	createdVersion string
	releasedID     string
	transitions    []string
	createErr      error
	releaseErr     error
	transitionErr  error
	version        *plugin.Version
}

func (f *fakeClient) CreateVersion(_ context.Context, projectKey, name, _ string) (*plugin.Version, error) {
	f.createdProject = projectKey
	f.createdVersion = name
	if f.version != nil || f.createErr != nil {
		return f.version, f.createErr
	}
	return &plugin.Version{ID: "42", Name: name}, nil
}

func (f *fakeClient) ReleaseVersion(_ context.Context, versionID string) error {
	f.releasedID = versionID
	return f.releaseErr
}

func (f *fakeClient) TransitionIssue(_ context.Context, issueKey, transitionName string) error {
	f.transitions = append(f.transitions, issueKey+":"+transitionName)
	return f.transitionErr
}

func env(kv map[string]string) func(string) string {
	return func(key string) string { return kv[key] }
}

func TestRunSuccess(t *testing.T) {

	fake := &fakeClient{}
	old := newClient
	newClient = func(cfg plugin.Config) jiraClient {
		if cfg.BaseURL != "https://jira.example.com" {
			t.Fatalf("unexpected base url: %s", cfg.BaseURL)
		}
		return fake
	}
	defer func() { newClient = old }()

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{
		"SEMREL_PLUGIN_BASE_URL":        "https://jira.example.com",
		"SEMREL_PLUGIN_EMAIL":           "bot@example.com",
		"SEMREL_PLUGIN_API_TOKEN":       "token",
		"SEMREL_PLUGIN_PROJECT_KEY":     "PROJ",
		"SEMREL_PLUGIN_TRANSITION_NAME": "Done",
		"SEMREL_VERSION":                "v1.2.3",
		"SEMREL_CHANGELOG":              "fix: PROJ-123\nfeat: PROJ-456",
	}), &stderr)

	if code != 0 || stderr.String() != "plugin_schema_version=1\n" {
		t.Fatalf("unexpected result: code=%d stderr=%q", code, stderr.String())
	}
	if fake.createdProject != "PROJ" || fake.createdVersion != "v1.2.3" || fake.releasedID != "42" {
		t.Fatalf("unexpected client calls: %+v", fake)
	}
	if len(fake.transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %v", fake.transitions)
	}
}

func TestRunDryRun(t *testing.T) {

	called := false
	old := newClient
	newClient = func(plugin.Config) jiraClient {
		called = true
		return &fakeClient{}
	}
	defer func() { newClient = old }()

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{
		"SEMREL_PLUGIN_BASE_URL":    "https://jira.example.com",
		"SEMREL_PLUGIN_EMAIL":       "bot@example.com",
		"SEMREL_PLUGIN_API_TOKEN":   "token",
		"SEMREL_PLUGIN_PROJECT_KEY": "PROJ",
		"SEMREL_VERSION":            "v1.2.3",
		"SEMREL_DRY_RUN":            "true",
	}), &stderr)
	if code != 0 || called {
		t.Fatalf("unexpected result: code=%d called=%v", code, called)
	}
}

func TestRunValidationError(t *testing.T) {

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{}), &stderr)
	if code != 1 || stderr.Len() == 0 {
		t.Fatalf("unexpected result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunCreateVersionError(t *testing.T) {

	old := newClient
	newClient = func(plugin.Config) jiraClient {
		return &fakeClient{createErr: errors.New("boom")}
	}
	defer func() { newClient = old }()

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{
		"SEMREL_PLUGIN_BASE_URL":    "https://jira.example.com",
		"SEMREL_PLUGIN_EMAIL":       "bot@example.com",
		"SEMREL_PLUGIN_API_TOKEN":   "token",
		"SEMREL_PLUGIN_PROJECT_KEY": "PROJ",
		"SEMREL_VERSION":            "v1.2.3",
	}), &stderr)
	if code != 1 || stderr.Len() == 0 {
		t.Fatalf("unexpected result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunMissingVersionID(t *testing.T) {

	old := newClient
	newClient = func(plugin.Config) jiraClient {
		return &fakeClient{version: &plugin.Version{Name: "v1.2.3"}}
	}
	defer func() { newClient = old }()

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{
		"SEMREL_PLUGIN_BASE_URL":    "https://jira.example.com",
		"SEMREL_PLUGIN_EMAIL":       "bot@example.com",
		"SEMREL_PLUGIN_API_TOKEN":   "token",
		"SEMREL_PLUGIN_PROJECT_KEY": "PROJ",
		"SEMREL_VERSION":            "v1.2.3",
	}), &stderr)
	if code != 1 || stderr.Len() == 0 {
		t.Fatalf("unexpected result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunReleaseError(t *testing.T) {
	old := newClient
	newClient = func(plugin.Config) jiraClient {
		return &fakeClient{releaseErr: errors.New("boom")}
	}
	defer func() { newClient = old }()

	var stderr bytes.Buffer
	code := run(context.Background(), env(map[string]string{
		"SEMREL_PLUGIN_BASE_URL":    "https://jira.example.com",
		"SEMREL_PLUGIN_EMAIL":       "bot@example.com",
		"SEMREL_PLUGIN_API_TOKEN":   "token",
		"SEMREL_PLUGIN_PROJECT_KEY": "PROJ",
		"SEMREL_VERSION":            "v1.2.3",
	}), &stderr)
	if code != 1 || stderr.Len() == 0 {
		t.Fatalf("unexpected result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "v1.2.3", "v1.2.4"); got != "v1.2.3" {
		t.Fatalf("unexpected value: %s", got)
	}
}
