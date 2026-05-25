// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJiraHookExecuteTransitionsIssuesAndComments(t *testing.T) {
	t.Parallel()

	var transitionsPosted atomic.Int32
	var commentsPosted atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/2/issue/PROJ-123/transitions":
			_, _ = w.Write([]byte(`{"transitions":[{"id":"31","name":"Released"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issue/PROJ-123/transitions":
			transitionsPosted.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issue/PROJ-123/comment":
			commentsPosted.Add(1)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	hook := NewJiraHook(server.Client(), server.URL, "user@example.com", "token-123")
	result, err := hook.Execute(context.Background(), &Release{Version: "1.2.3", TagName: "v1.2.3", Repository: "SemRels/semrel", Commits: []string{"feat: release prep\n\nFixes PROJ-123"}})
	require.NoError(t, err)
	require.EqualValues(t, 1, transitionsPosted.Load())
	require.EqualValues(t, 1, commentsPosted.Load())
	require.Equal(t, "PROJ-123", result.Outputs["issues"])
}
