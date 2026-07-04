package claudeapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultModel = "claude-sonnet-4-6"

type Client struct {
	apiKey  string
	model   string
	baseURL string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		model:  defaultModel,
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

	opts := []option.RequestOption{option.WithAPIKey(client.apiKey)}
	if client.baseURL != "" {
		opts = append(opts, option.WithBaseURL(client.baseURL))
	}
	sdkClient := anthropic.NewClient(opts...)

	message, err := sdkClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     client.model,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userPayload))},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic api: %w", err)
	}

	for _, block := range message.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", errors.New("no text content in anthropic response")
}
