package claudeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	defaultModel   = "claude-sonnet-4-6" //TODO check against haiku?
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
}

type messageRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system"`
	Messages  []requestUserPrompt `json:"messages"`
}

type requestUserPrompt struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messageResponse struct {
	Content []responseContent `json:"content"`
}

type responseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
		model:      defaultModel,
		baseURL:    defaultBaseURL,
	}
}

// Complete sends a single request and returns the text content from Claude.
func (client *Client) Complete(ctx context.Context, systemPrompt, userPayload string) (string, error) {
	if client == nil {
		return "", errors.New("missing anthropic api key")
	}
	if client.apiKey == "" {
		return "", errors.New("missing anthropic api key")
	}

	requestBody, err := json.Marshal(messageRequest{
		Model:     client.model,
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages:  []requestUserPrompt{{Role: "user", Content: userPayload}},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(client.baseURL, "/") + "/v1/messages"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("x-api-key", client.apiKey)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("content-type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return "", fmt.Errorf("anthropic api status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded messageResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	for _, item := range decoded.Content {
		if item.Type == "text" {
			return item.Text, nil
		}
	}
	return "", errors.New("no text content in anthropic response")
}
