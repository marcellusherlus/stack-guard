package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "https://api.github.com"
	defaultUserAgent    = "stack-guard"
	githubAPIVersion    = "2022-11-28" //Todo might be worth to use the 2026 version
	maxFetchedFiles     = 25
	maxContentSizeBytes = 1 << 20
)

var (
	ErrNotFound = errors.New("repository not found or inaccessible (private or missing token)")
	retryDelay  = 10 * time.Millisecond
)

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type RepoSnapshot struct {
	Repository    string
	DefaultBranch string
	Tree          []TreeEntry
	Truncated     bool
	Files         map[string]string
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type repositoryResponse struct {
	DefaultBranch string `json:"default_branch"`
}

type treeResponse struct {
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type contentResponse struct {
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	Type     string `json:"type"`
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      strings.TrimSpace(token),
		baseURL:    defaultBaseURL,
	}
}

// FetchRepo loads the repository metadata, full tree, and selected text files.
func (client *Client) FetchRepo(ctx context.Context, repository string, selectFiles func(paths []string) []string) (RepoSnapshot, error) {
	defaultBranch, err := client.fetchDefaultBranch(ctx, repository)
	if err != nil {
		return RepoSnapshot{}, err
	}

	tree, truncated, err := client.fetchTree(ctx, repository, defaultBranch)
	if err != nil {
		return RepoSnapshot{}, err
	}

	snapshot := RepoSnapshot{
		Repository:    repository,
		DefaultBranch: defaultBranch,
		Tree:          tree,
		Truncated:     truncated,
		Files:         make(map[string]string),
	}

	if selectFiles == nil {
		return snapshot, nil
	}

	blobPaths := blobPaths(tree)
	selected := selectFiles(blobPaths)
	if len(selected) > maxFetchedFiles {
		selected = selected[:maxFetchedFiles]
	}

	for _, filePath := range selected {
		if isBinaryPath(filePath) {
			continue
		}

		content, fetched, err := client.fetchContent(ctx, repository, filePath)
		if err != nil {
			return RepoSnapshot{}, err
		}
		if !fetched {
			continue
		}
		snapshot.Files[filePath] = content
	}

	return snapshot, nil
}

func (client *Client) fetchDefaultBranch(ctx context.Context, repository string) (string, error) {
	var response repositoryResponse
	requestPath := path.Join("repos", escapeRepository(repository))
	if err := client.getJSON(ctx, requestPath, nil, &response); err != nil {
		return "", fmt.Errorf("fetch repo metadata for %q: %w", repository, err)
	}
	return response.DefaultBranch, nil
}

func (client *Client) fetchTree(ctx context.Context, repository, branch string) ([]TreeEntry, bool, error) {
	var response treeResponse
	requestPath := path.Join("repos", escapeRepository(repository), "git", "trees", url.PathEscape(branch))
	query := url.Values{"recursive": []string{"1"}}
	if err := client.getJSON(ctx, requestPath, query, &response); err != nil {
		return nil, false, fmt.Errorf("fetch repo tree for %q: %w", repository, err)
	}
	return response.Tree, response.Truncated, nil
}

func (client *Client) fetchContent(ctx context.Context, repository, filePath string) (string, bool, error) {
	var response contentResponse
	requestPath := path.Join("repos", escapeRepository(repository), "contents", escapePath(filePath))
	if err := client.getJSON(ctx, requestPath, nil, &response); err != nil {
		return "", false, fmt.Errorf("fetch file content %q for %q: %w", filePath, repository, err)
	}

	if response.Type != "file" {
		return "", false, nil
	}
	if response.Size > maxContentSizeBytes {
		return "", false, nil
	}
	if response.Encoding != "base64" {
		return "", false, fmt.Errorf("unsupported content encoding %q", response.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
	if err != nil {
		return "", false, fmt.Errorf("decode base64 content: %w", err)
	}

	return string(decoded), true, nil
}

func (client *Client) getJSON(ctx context.Context, requestPath string, query url.Values, target any) error {
	requestURL := client.baseURL + "/" + strings.TrimPrefix(requestPath, "/")
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	var lastErr error
	for attempt := range 2 {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		client.applyHeaders(request)

		response, err := client.httpClient.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("perform request: %w", err)
			if attempt == 0 && ctx.Err() == nil {
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		func() {
			defer response.Body.Close()
			lastErr = decodeResponse(response, target)
		}()
		if lastErr == nil {
			return nil
		}
		if attempt == 0 && shouldRetryStatus(response.StatusCode) && ctx.Err() == nil {
			time.Sleep(retryDelay)
			continue
		}
		return lastErr
	}

	return lastErr
}

func (client *Client) applyHeaders(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}
}

func decodeResponse(response *http.Response, target any) error {
	if response.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0" {
		resetAt := response.Header.Get("X-RateLimit-Reset")
		if resetAt != "" {
			return fmt.Errorf("github api rate limit exceeded; resets at %s, set --token: %w", resetAt, ErrNotFound)
		}
		return errors.New("github api rate limit exceeded, set --token")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("github api returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func blobPaths(tree []TreeEntry) []string {
	paths := make([]string, 0, len(tree))
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		paths = append(paths, entry.Path)
	}
	return paths
}

func shouldRetryStatus(statusCode int) bool {
	return statusCode >= 500 && statusCode <= 599
}

func escapeRepository(repository string) string {
	parts := strings.Split(repository, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return path.Join(parts...)
}

func escapePath(filePath string) string {
	parts := strings.Split(filePath, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return path.Join(parts...)
}

func isBinaryPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	for _, suffix := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".zip", ".jar", ".exe", ".dll", ".so", ".dylib", ".class"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
