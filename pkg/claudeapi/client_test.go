package claudeapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// minimalMessageResponse returns a minimal valid Anthropic Messages API response body.
func minimalMessageResponse(text string) string {
	return fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":5},"content":[{"type":"text","text":%q}]}`, text)
}

func TestClientComplete_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("unexpected api key header: %q", request.Header.Get("x-api-key"))
		}
		if request.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("unexpected anthropic-version header: %q", request.Header.Get("anthropic-version"))
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, minimalMessageResponse(`{"technologies":[]}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL

	result, err := client.Complete(context.Background(), "system", "payload")
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if !strings.Contains(result, "technologies") {
		t.Fatalf("unexpected completion: %q", result)
	}
}

func TestClientComplete_MissingAPIKey(t *testing.T) {
	client := NewClient("")
	_, err := client.Complete(context.Background(), "system", "payload")
	if err == nil {
		t.Fatal("expected missing API key error")
	}
}

func TestClientComplete_HandlesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(writer, `{"error":{"type":"invalid_request_error","message":"bad request"}}`)
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL

	_, err := client.Complete(context.Background(), "system", "payload")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected status error, got %v", err)
	}
}
