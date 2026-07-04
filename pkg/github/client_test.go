package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientFetchRepo_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/widgets":
			assertHeaders(t, request)
			fmt.Fprint(writer, `{"default_branch":"main"}`)
		case "/repos/acme/widgets/git/trees/main":
			assertHeaders(t, request)
			if request.URL.RawQuery != "recursive=1" {
				t.Fatalf("expected recursive=1 query, got %q", request.URL.RawQuery)
			}
			fmt.Fprint(writer, `{"tree":[{"path":"README.md","type":"blob"},{"path":"package.json","type":"blob"},{"path":"docs","type":"tree"}],"truncated":false}`)
		case "/repos/acme/widgets/contents/package.json":
			assertHeaders(t, request)
			content := base64.StdEncoding.EncodeToString([]byte(`{"name":"widgets"}`))
			fmt.Fprintf(writer, `{"type":"file","size":18,"encoding":"base64","content":%q}`+"\n", content)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient("secret-token")
	client.baseURL = server.URL

	snapshot, err := client.FetchRepo(context.Background(), "acme/widgets", func(paths []string) []string {
		if len(paths) != 2 {
			t.Fatalf("expected 2 blob paths, got %d", len(paths))
		}
		return []string{"package.json"}
	})
	if err != nil {
		t.Fatalf("FetchRepo returned error: %v", err)
	}

	if snapshot.Repository != "acme/widgets" {
		t.Fatalf("unexpected repository: %q", snapshot.Repository)
	}
	if snapshot.DefaultBranch != "main" {
		t.Fatalf("unexpected branch: %q", snapshot.DefaultBranch)
	}
	if snapshot.Truncated {
		t.Fatal("expected non-truncated snapshot")
	}
	if got := snapshot.Files["package.json"]; !strings.Contains(got, `"widgets"`) {
		t.Fatalf("unexpected fetched file contents: %q", got)
	}
}

func TestClientFetchRepo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL

	_, err := client.FetchRepo(context.Background(), "acme/missing", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClientFetchRepo_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-RateLimit-Remaining", "0")
		writer.Header().Set("X-RateLimit-Reset", "1234567890")
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL

	_, err := client.FetchRepo(context.Background(), "acme/widgets", nil)
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestClientFetchRepo_TruncatedTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/widgets":
			fmt.Fprint(writer, `{"default_branch":"main"}`)
		case "/repos/acme/widgets/git/trees/main":
			fmt.Fprint(writer, `{"tree":[],"truncated":true}`)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL

	snapshot, err := client.FetchRepo(context.Background(), "acme/widgets", nil)
	if err != nil {
		t.Fatalf("FetchRepo returned error: %v", err)
	}
	if !snapshot.Truncated {
		t.Fatal("expected truncated snapshot")
	}
}

func TestClientFetchRepo_SkipsBinaryAndOversizedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/widgets":
			fmt.Fprint(writer, `{"default_branch":"main"}`)
		case "/repos/acme/widgets/git/trees/main":
			fmt.Fprint(writer, `{"tree":[{"path":"image.png","type":"blob"},{"path":"large.txt","type":"blob"}],"truncated":false}`)
		case "/repos/acme/widgets/contents/large.txt":
			content := base64.StdEncoding.EncodeToString([]byte("ignored"))
			fmt.Fprintf(writer, `{"type":"file","size":%d,"encoding":"base64","content":%q}`+"\n", maxContentSizeBytes+1, content)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL

	snapshot, err := client.FetchRepo(context.Background(), "acme/widgets", func(paths []string) []string {
		return paths
	})
	if err != nil {
		t.Fatalf("FetchRepo returned error: %v", err)
	}
	if len(snapshot.Files) != 0 {
		t.Fatalf("expected no fetched files, got %d", len(snapshot.Files))
	}
}

func TestClientFetchRepo_RetriesTransientFailures(t *testing.T) {
	var treeAttempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/widgets":
			fmt.Fprint(writer, `{"default_branch":"main"}`)
		case "/repos/acme/widgets/git/trees/main":
			if treeAttempts.Add(1) == 1 {
				writer.WriteHeader(http.StatusBadGateway)
				fmt.Fprint(writer, `{"message":"temporary"}`)
				return
			}
			fmt.Fprint(writer, `{"tree":[],"truncated":false}`)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL

	_, err := client.FetchRepo(context.Background(), "acme/widgets", nil)
	if err != nil {
		t.Fatalf("FetchRepo returned error: %v", err)
	}
	if treeAttempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", treeAttempts.Load())
	}
}

func assertHeaders(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Accept") != "application/vnd.github+json" {
		t.Fatalf("unexpected Accept header: %q", request.Header.Get("Accept"))
	}
	if request.Header.Get("User-Agent") != defaultUserAgent {
		t.Fatalf("unexpected User-Agent header: %q", request.Header.Get("User-Agent"))
	}
	if request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
		t.Fatalf("unexpected API version header: %q", request.Header.Get("X-GitHub-Api-Version"))
	}
	if authorization := request.Header.Get("Authorization"); authorization != "Bearer secret-token" {
		t.Fatalf("unexpected Authorization header: %q", authorization)
	}
}
