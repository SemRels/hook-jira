package plugin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeErrorBody struct {
	io.Reader
	closed bool
}

func (b *closeErrorBody) Close() error {
	b.closed = true
	return errors.New("close failed")
}

type closeErrorRoundTripper func(*http.Request) (*http.Response, error)

func (fn closeErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCreateVersionIgnoresCloseErrorAfterAcceptedResponses(t *testing.T) {
	projectBody := &closeErrorBody{Reader: strings.NewReader(`{"id":"10001"}`)}
	versionBody := &closeErrorBody{Reader: strings.NewReader(`{"id":"20001","name":"v1.0.0"}`)}
	client := NewClient(Config{BaseURL: "https://jira.example.test"})
	client.http = &http.Client{Transport: closeErrorRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/rest/api/3/project/PROJ":
			return &http.Response{StatusCode: http.StatusOK, Body: projectBody, Header: make(http.Header)}, nil
		case "/rest/api/3/version":
			return &http.Response{StatusCode: http.StatusCreated, Body: versionBody, Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	version, err := client.CreateVersion(context.Background(), "PROJ", "v1.0.0", "")
	if err != nil {
		t.Fatalf("CreateVersion() error = %v", err)
	}
	if version.ID != "20001" || !projectBody.closed || !versionBody.closed {
		t.Fatalf("version = %#v, project closed = %t, version closed = %t", version, projectBody.closed, versionBody.closed)
	}
}
