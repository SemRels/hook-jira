package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	plugin "github.com/SemRels/hook-jira/internal/plugin"
)

const pluginSchemaVersion = 1

type jiraClient interface {
	CreateVersion(context.Context, string, string, string) (*plugin.Version, error)
	ReleaseVersion(context.Context, string) error
	TransitionIssue(context.Context, string, string) error
}

var newClient = func(cfg plugin.Config) jiraClient {
	return plugin.NewClient(cfg)
}

func run(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stderr, "plugin_schema_version=%d\n", pluginSchemaVersion)
	baseURL := getenv("SEMREL_PLUGIN_BASE_URL")
	email := getenv("SEMREL_PLUGIN_EMAIL")
	apiToken := getenv("SEMREL_PLUGIN_API_TOKEN")
	projectKey := getenv("SEMREL_PLUGIN_PROJECT_KEY")
	transitionName := getenv("SEMREL_PLUGIN_TRANSITION_NAME")
	version := firstNonEmpty(getenv("SEMREL_VERSION"), getenv("SEMREL_TAG_NAME"), getenv("SEMREL_NEXT_VERSION"))
	changelog := getenv("SEMREL_CHANGELOG")

	if baseURL == "" || email == "" || apiToken == "" || projectKey == "" {
		_, _ = fmt.Fprintln(stderr, "hook-jira: SEMREL_PLUGIN_BASE_URL, SEMREL_PLUGIN_EMAIL, SEMREL_PLUGIN_API_TOKEN, and SEMREL_PLUGIN_PROJECT_KEY are required")
		return 1
	}
	if version == "" {
		_, _ = fmt.Fprintln(stderr, "hook-jira: SEMREL_VERSION, SEMREL_TAG_NAME, or SEMREL_NEXT_VERSION is required")
		return 1
	}
	if getenv("SEMREL_DRY_RUN") == "true" {
		return 0
	}

	client := newClient(plugin.Config{BaseURL: baseURL, Email: email, APIToken: apiToken})
	created, err := client.CreateVersion(ctx, projectKey, version, changelog)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "hook-jira:", err)
		return 1
	}
	if created == nil || created.ID == "" {
		_, _ = fmt.Fprintln(stderr, "hook-jira: created version is missing an id")
		return 1
	}
	if err := client.ReleaseVersion(ctx, created.ID); err != nil {
		_, _ = fmt.Fprintln(stderr, "hook-jira:", err)
		return 1
	}

	if transitionName != "" {
		for _, issueKey := range plugin.ExtractIssueKeys([]string{changelog}) {
			if err := client.TransitionIssue(ctx, issueKey, transitionName); err != nil {
				_, _ = fmt.Fprintln(stderr, "hook-jira:", err)
				return 1
			}
		}
	}
	return 0
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	os.Exit(run(ctx, os.Getenv, os.Stderr))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
