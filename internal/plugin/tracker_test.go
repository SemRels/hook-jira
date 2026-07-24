package plugin_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "github.com/SemRels/hook-jira/internal/plugin"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *jira.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return jira.NewClient(jira.Config{
		BaseURL:  srv.URL,
		Email:    "user@example.com",
		APIToken: "test-token",
	})
}

func expectAuth(t *testing.T, r *http.Request) {
	t.Helper()
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:test-token"))
	if got := r.Header.Get("Authorization"); got != want {
		t.Fatalf("unexpected auth header: %s", got)
	}
}

func TestExtractIssueKeys(t *testing.T) {
	commits := []string{
		"fix: resolve ABC-123 crash on startup",
		"feat: implement PROJ-456 dark mode",
		"chore: update deps (no jira issue)",
		"fix: ABC-123 duplicate should be deduped",
		"refactor: PROJ-789 and PROJ-456 together",
	}
	keys := jira.ExtractIssueKeys(commits)
	if len(keys) != 3 {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestCreateVersionSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/project/PROJ", func(w http.ResponseWriter, r *http.Request) {
		expectAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"id": "10001"}); err != nil {
			t.Errorf("encode project response: %v", err)
		}
	})
	mux.HandleFunc("/rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		expectAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]any{"id": "20001", "name": "v1.0.0"}); err != nil {
			t.Errorf("encode version response: %v", err)
		}
	})
	client := newTestClient(t, mux)

	version, err := client.CreateVersion(context.Background(), "PROJ", "v1.0.0", "Release v1.0.0")
	if err != nil {
		t.Fatalf("CreateVersion error: %v", err)
	}
	if version.ID != "20001" || version.Name != "v1.0.0" {
		t.Fatalf("unexpected version: %+v", version)
	}
}

func TestCreateVersionInvalidProjectID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/project/PROJ", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"id": "not-a-number"}); err != nil {
			t.Errorf("encode project response: %v", err)
		}
	})
	client := newTestClient(t, mux)

	_, err := client.CreateVersion(context.Background(), "PROJ", "v1.0.0", "")
	if err == nil || !strings.Contains(err.Error(), "parse project id") {
		t.Fatalf("expected project ID parse error, got %v", err)
	}
}

func TestCreateVersionProjectError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/project/PROJ", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("missing project"))
	})
	client := newTestClient(t, mux)

	err := func() error {
		_, err := client.CreateVersion(context.Background(), "PROJ", "v1.0.0", "")
		return err
	}()
	if err == nil || !strings.Contains(err.Error(), "get project") {
		t.Fatalf("expected project error, got %v", err)
	}
}

func TestReleaseVersionSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/version/20001", func(w http.ResponseWriter, r *http.Request) {
		expectAuth(t, r)
		w.WriteHeader(http.StatusOK)
	})
	client := newTestClient(t, mux)

	if err := client.ReleaseVersion(context.Background(), "20001"); err != nil {
		t.Fatalf("ReleaseVersion error: %v", err)
	}
}

func TestReleaseVersionAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/version/20001", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad version"))
	})
	client := newTestClient(t, mux)

	err := client.ReleaseVersion(context.Background(), "20001")
	if err == nil || !strings.Contains(err.Error(), "release version") {
		t.Fatalf("expected api error, got %v", err)
	}
}

func TestTransitionIssueSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/PROJ-123/transitions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			expectAuth(t, r)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "31", "name": "Done"}}}); err != nil {
				t.Errorf("encode transitions response: %v", err)
			}
		case http.MethodPost:
			expectAuth(t, r)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	client := newTestClient(t, mux)

	if err := client.TransitionIssue(context.Background(), "PROJ-123", "Done"); err != nil {
		t.Fatalf("TransitionIssue error: %v", err)
	}
}

func TestTransitionIssueNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/PROJ-123/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "11", "name": "To Do"}}}); err != nil {
				t.Errorf("encode transitions response: %v", err)
			}
		}
	})
	client := newTestClient(t, mux)

	err := client.TransitionIssue(context.Background(), "PROJ-123", "Released")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestTransitionIssueAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/PROJ-123/transitions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "31", "name": "Done"}}}); err != nil {
				t.Errorf("encode transitions response: %v", err)
			}
		case http.MethodPost:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		}
	})
	client := newTestClient(t, mux)

	err := client.TransitionIssue(context.Background(), "PROJ-123", "Done")
	if err == nil || !strings.Contains(err.Error(), "transition issue") {
		t.Fatalf("expected api error, got %v", err)
	}
}
