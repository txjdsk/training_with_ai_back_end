package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	config "training_with_ai/configs"
	"training_with_ai/internal/pkg/logger"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

type Client struct {
	client         *openai.Client
	defaultModel   string
	rewriteModel   string
	critiqueModel  string
	customerModel  string
	embeddingModel string
	temperature    float32
	maxTokens      int
}

type CritiqueResult struct {
	Critique        string `json:"critique"`
	ReferenceAnswer string `json:"reference_answer"`
	SentimentLabel  string `json:"sentiment_label"`
}

func NewClient(cfg config.LLMConfig) (*Client, error) {
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil, errors.New("llm config is incomplete")
	}

	defaultModel := cfg.Model
	if defaultModel == "" {
		defaultModel = cfg.RewriteModel
	}
	if defaultModel == "" {
		defaultModel = cfg.CritiqueModel
	}
	if defaultModel == "" {
		defaultModel = cfg.EmbeddingModel
	}
	if defaultModel == "" {
		return nil, errors.New("llm model is missing")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	openaiCfg := openai.DefaultConfig(cfg.APIKey)
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	openaiCfg.BaseURL = baseURL
	openaiCfg.HTTPClient = &http.Client{Timeout: timeout}
	client := openai.NewClientWithConfig(openaiCfg)

	return &Client{
		client:         client,
		defaultModel:   defaultModel,
		rewriteModel:   firstNonEmpty(cfg.RewriteModel, defaultModel),
		critiqueModel:  firstNonEmpty(cfg.CritiqueModel, defaultModel),
		customerModel:  firstNonEmpty(cfg.CustomerModel, defaultModel),
		embeddingModel: cfg.EmbeddingModel,
		temperature:    float32(cfg.Temperature),
		maxTokens:      cfg.MaxTokens,
	}, nil
}

func (c *Client) OptimizePrompt(ctx context.Context, original string, requirement string) (string, error) {
	request := openai.ChatCompletionRequest{
		Model: c.rewriteModel,
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
	}
	logChatRequest("optimize_prompt", request)
	resp, err := c.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logLLMError("optimize_prompt", err)
		return "", err
	}
	logChatResponse("optimize_prompt", resp)
	if len(resp.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *Client) RewriteQuery(ctx context.Context, dialogue string) (string, error) {
	request := openai.ChatCompletionRequest{
		Model: c.rewriteModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "提炼该对话涉及的礼仪问题类型，生成仅用于检索礼仪知识库的精准语句，仅返回检索语句。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: dialogue,
			},
		},
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}
	logChatRequest("rewrite_query", request)
	resp, err := c.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logLLMError("rewrite_query", err)
		return "", err
	}
	logChatResponse("rewrite_query", resp)
	if len(resp.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (c *Client) Critique(ctx context.Context, dialogue string, etiquetteText string) (*CritiqueResult, error) {
	systemPrompt := strings.Join([]string{
		"按礼仪规范点评该客服应答，输出点评语、参考回答，并给出情感分析标签。",
		"仅输出 JSON，字段包括: critique, reference_answer, sentiment_label。",
		"sentiment_label 只能是: 积极, 一般积极, 一般负面, 负面。",
	}, "\n")

	userPrompt := "对话内容:\n" + dialogue + "\n\n礼仪规范:\n" + etiquetteText
	request := openai.ChatCompletionRequest{
		Model: c.critiqueModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}
	logChatRequest("critique", request)
	resp, err := c.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logLLMError("critique", err)
		return nil, err
	}
	logChatResponse("critique", resp)
	if len(resp.Choices) == 0 {
		return nil, errors.New("llm response has no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = trimCodeFence(content)

	var result CritiqueResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateCustomerReply(ctx context.Context, systemPrompt string, history string, userMsg string) (string, error) {
	systemContent := strings.TrimSpace(systemPrompt)
	if systemContent == "" {
		systemContent = "You are a customer in a service conversation. Reply naturally based on the context."
	}

	userContent := "对话历史:\n" + history + "\n\n当前店员回复:\n" + userMsg + "\n\n请输出顾客的下一句回复。"
	request := openai.ChatCompletionRequest{
		Model: c.customerModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemContent,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userContent,
			},
		},
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}
	logChatRequest("customer_reply", request)
	resp, err := c.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logLLMError("customer_reply", err)
		return "", err
	}
	logChatResponse("customer_reply", resp)
	if len(resp.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	request := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(c.embeddingModel),
		Input: []string{text},
	}
	logEmbeddingRequest("embedding", request)
	resp, err := c.client.CreateEmbeddings(ctx, request)
	if err != nil {
		logLLMError("embedding", err)
		return nil, err
	}
	logEmbeddingResponse("embedding", resp)
	if len(resp.Data) == 0 {
		return nil, errors.New("embedding response has no data")
	}
	return resp.Data[0].Embedding, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func trimCodeFence(content string) string {
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimPrefix(content, "json")
		content = strings.TrimPrefix(content, "\n")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func logLLMError(action string, err error) {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		logger.Error("llm api error",
			zap.String("action", action),
			zap.Int("status_code", apiErr.HTTPStatusCode),
			zap.String("type", apiErr.Type),
			zap.String("message", apiErr.Message),
			zap.String("code", fmt.Sprint(apiErr.Code)),
		)
		return
	}
	logger.Error("llm request error", zap.String("action", action), zap.Error(err))
}

func logChatRequest(action string, req openai.ChatCompletionRequest) {
	messages := truncateMessages(req.Messages)
	logger.Debugw("llm request",
		"action", action,
		"model", req.Model,
		"messages", messages,
		"temperature", req.Temperature,
		"max_tokens", req.MaxTokens,
	)
}

func logChatResponse(action string, resp openai.ChatCompletionResponse) {
	choices := truncateChoices(resp.Choices)
	logger.Debugw("llm response",
		"action", action,
		"id", resp.ID,
		"model", resp.Model,
		"choices", choices,
		"usage", resp.Usage,
	)
}

func logEmbeddingRequest(action string, req openai.EmbeddingRequest) {
	input := truncateInputValue(req.Input)
	logger.Debugw("llm request",
		"action", action,
		"model", req.Model,
		"input", input,
	)
}

func logEmbeddingResponse(action string, resp openai.EmbeddingResponse) {
	logger.Debugw("llm response",
		"action", action,
		"model", resp.Model,
		"data", resp.Data,
		"usage", resp.Usage,
	)
}

func truncateString(value string) string {
	if len(value) <= 1000 {
		return value
	}
	head := value[:500]
	tail := value[len(value)-500:]
	return head + "..." + tail
}

func truncateMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	result := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		msgCopy := msg
		msgCopy.Content = truncateString(msgCopy.Content)
		result = append(result, msgCopy)
	}
	return result
}

type choiceLog struct {
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	Content      string `json:"content"`
}

func truncateChoices(choices []openai.ChatCompletionChoice) []choiceLog {
	if len(choices) == 0 {
		return []choiceLog{}
	}
	result := make([]choiceLog, 0, len(choices))
	for _, choice := range choices {
		result = append(result, choiceLog{
			Index:        choice.Index,
			FinishReason: fmt.Sprint(choice.FinishReason),
			Content:      truncateString(choice.Message.Content),
		})
	}
	return result
}

func truncateInputValue(input any) any {
	switch value := input.(type) {
	case []string:
		if len(value) == 0 {
			return value
		}
		result := make([]string, 0, len(value))
		for _, item := range value {
			result = append(result, truncateString(item))
		}
		return result
	case string:
		return truncateString(value)
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			result = append(result, truncateString(fmt.Sprint(item)))
		}
		return result
	default:
		return truncateString(fmt.Sprint(value))
	}
}
