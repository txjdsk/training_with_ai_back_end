package llm

import (
	"context"
	"errors"
	"net/http"
	"time"

	config "training_with_ai/configs"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client      *openai.Client
	model       string
	temperature float32
	maxTokens   int
}

func NewClient(cfg config.LLMConfig) (*Client, error) {
	if cfg.APIKey == "" || cfg.BaseURL == "" || cfg.Model == "" {
		return nil, errors.New("llm config is incomplete")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	openaiCfg := openai.DefaultConfig(cfg.APIKey)
	openaiCfg.BaseURL = cfg.BaseURL
	openaiCfg.HTTPClient = &http.Client{Timeout: timeout}
	client := openai.NewClientWithConfig(openaiCfg)

	return &Client{
		client:      client,
		model:       cfg.Model,
		temperature: float32(cfg.Temperature),
		maxTokens:   cfg.MaxTokens,
	}, nil
}

func (c *Client) OptimizePrompt(ctx context.Context, original string, requirement string) (string, error) {
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You rewrite prompts. Keep intent, improve clarity and usefulness. Output only the rewritten prompt.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Original prompt:\n" + original + "\n\nRequirement:\n" + requirement,
				},
			},
			Temperature: c.temperature,
			MaxTokens:   c.maxTokens,
		},
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}

	return resp.Choices[0].Message.Content, nil
}
