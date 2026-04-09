package llm

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

	config "training_with_ai/configs"
	"training_with_ai/internal/pkg/logger"

	"go.uber.org/zap"
)

type Client struct {
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	defaultModel   string
	rewriteModel   string
	critiqueModel  string
	customerModel  string
	embeddingModel string
	temperature    float32
	maxTokens      int
	rewriteMax     int
	critiqueMax    int
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float32       `json:"temperature,omitempty"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	EnableThinking *bool         `json:"enable_thinking,omitempty"`
}

type chatMessageResponse struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatChoice struct {
	Index        int                 `json:"index"`
	FinishReason string              `json:"finish_reason"`
	Message      chatMessageResponse `json:"message"`
}

type chatCompletionResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   any          `json:"usage"`
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type embeddingResponse struct {
	Model string          `json:"model"`
	Data  []embeddingData `json:"data"`
	Usage any             `json:"usage"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type CritiqueResult struct {
	Critique        string `json:"critique"`
	PolishReply     string `json:"polish_reply"`
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

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	return &Client{
		baseURL:        baseURL,
		apiKey:         cfg.APIKey,
		httpClient:     &http.Client{Timeout: timeout},
		defaultModel:   defaultModel,
		rewriteModel:   firstNonEmpty(cfg.RewriteModel, defaultModel),
		critiqueModel:  firstNonEmpty(cfg.CritiqueModel, defaultModel),
		customerModel:  firstNonEmpty(cfg.CustomerModel, defaultModel),
		embeddingModel: cfg.EmbeddingModel,
		temperature:    float32(cfg.Temperature),
		maxTokens:      cfg.MaxTokens,
		rewriteMax:     cfg.RewriteMax,
		critiqueMax:    cfg.CritiqueMax,
	}, nil
}

func (c *Client) OptimizePrompt(ctx context.Context, original string, requirement string) (string, error) {
	request := chatCompletionRequest{
		Model: c.rewriteModel,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "你负责重写提示词。按照要求以最小修改原则修改提示词。仅输出重写后的提示词即可。",
			},
			{
				Role:    "user",
				Content: "原始提示词：\n" + original + "\n\n要求：\n" + requirement,
			},
		},
		Temperature:    c.temperature,
		MaxTokens:      c.maxTokens,
		EnableThinking: boolPtr(false),
	}
	logChatRequest("optimize_prompt", request)
	resp, err := c.createChatCompletion(ctx, request)
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
	request := chatCompletionRequest{
		Model: c.rewriteModel,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "提炼该对话涉及的礼仪问题类型，生成仅用于检索礼仪知识库的精准语句，仅返回检索语句。",
			},
			{
				Role:    "user",
				Content: dialogue,
			},
		},
		Temperature:    c.temperature,
		MaxTokens:      firstPositiveInt(c.rewriteMax, c.maxTokens),
		EnableThinking: boolPtr(false),
	}
	logChatRequest("rewrite_query", request)
	resp, err := c.createChatCompletion(ctx, request)
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
		"请根据提供的【对话内容】和【礼仪规范】，执行以下任务\n*注：“斧正”特指对客服当前应答的修改润色。*" +
			`1. 点评：客观点评客服当前应答的优缺点。
2. 斧正：将客服当前的应答修改得更好。
3. 建议：提供客服接下来可以说的一句优秀回复作为参考。

请严格使用以下格式输出，严禁使用JSON：
<review>这里是点评内容</review>
<polish>这里是斧正后的应答</polish>
<answer>这里是接下来的参考回复</answer>`,
	}, "\n")

	userPrompt := "对话内容:\n" + dialogue + "\n\n礼仪规范:\n" + etiquetteText
	request := chatCompletionRequest{
		Model: c.critiqueModel,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature:    c.temperature,
		MaxTokens:      firstPositiveInt(c.critiqueMax, c.maxTokens),
		EnableThinking: boolPtr(false),
	}
	logChatRequest("critique", request)
	resp, err := c.createChatCompletion(ctx, request)
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
	reviewText, okReview := extractTaggedText(content, "review")
	polishText, okPolish := extractTaggedText(content, "polish")
	answerText, okAnswer := extractTaggedText(content, "answer")
	if !okReview || !okPolish || !okAnswer {
		return nil, fmt.Errorf("critique response missing review/polish/answer tags")
	}
	result := CritiqueResult{
		Critique:        reviewText,
		PolishReply:     polishText,
		ReferenceAnswer: answerText,
		SentimentLabel:  "",
	}
	return &result, nil
}

func (c *Client) GenerateCustomerReply(ctx context.Context, systemPrompt string, history string, userMsg string) (string, error) {
	systemContent := strings.TrimSpace(systemPrompt)
	if systemContent == "" {
		systemContent = "You are a customer in a service conversation. Reply naturally based on the context."
	}

	userContent := "对话历史:\n" + history + "\n\n当前店员回复:\n" + userMsg + "\n\n请输出顾客的下一句回复。"
	request := chatCompletionRequest{
		Model: c.customerModel,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemContent,
			},
			{
				Role:    "user",
				Content: userContent,
			},
		},
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		//EnableThinking: boolPtr(false),
	}
	logChatRequest("customer_reply", request)
	resp, err := c.createChatCompletion(ctx, request)
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
	request := embeddingRequest{
		Model: c.embeddingModel,
		Input: []string{text},
	}
	logEmbeddingRequest("embedding", request)
	resp, err := c.createEmbeddings(ctx, request)
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

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
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

func extractTaggedText(content string, tag string) (string, bool) {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(content, openTag)
	if start == -1 {
		return "", false
	}
	start += len(openTag)
	end := strings.Index(content[start:], closeTag)
	if end == -1 {
		return "", false
	}
	text := content[start : start+end]
	return strings.TrimSpace(text), true
}

func logLLMError(action string, err error) {
	logger.Error("llm request error", zap.String("action", action), zap.Error(err))
}

func logChatRequest(action string, req chatCompletionRequest) {
	messages := truncateMessages(req.Messages)
	logger.Debugw("llm request",
		"action", action,
		"model", req.Model,
		"messages", messages,
		"temperature", req.Temperature,
		"max_tokens", req.MaxTokens,
	)
}

func logChatResponse(action string, resp chatCompletionResponse) {
	choices := truncateChoices(resp.Choices)
	logger.Debugw("llm response",
		"action", action,
		"id", resp.ID,
		"model", resp.Model,
		"choices", choices,
		"usage", resp.Usage,
	)
}

func logEmbeddingRequest(action string, req embeddingRequest) {
	input := truncateInputValue(req.Input)
	logger.Debugw("llm request",
		"action", action,
		"model", req.Model,
		"input", input,
	)
}

func logEmbeddingResponse(action string, resp embeddingResponse) {
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

func truncateMessages(messages []chatMessage) []chatMessage {
	if len(messages) == 0 {
		return messages
	}
	result := make([]chatMessage, 0, len(messages))
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

func truncateChoices(choices []chatChoice) []choiceLog {
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

func (c *Client) createChatCompletion(ctx context.Context, req chatCompletionRequest) (chatCompletionResponse, error) {
	url := c.baseURL + "/chat/completions"
	body, err := json.Marshal(req)
	if err != nil {
		return chatCompletionResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return chatCompletionResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return chatCompletionResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return chatCompletionResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return chatCompletionResponse{}, fmt.Errorf("llm api error: status=%d type=%s code=%v message=%s", resp.StatusCode, apiErr.Error.Type, apiErr.Error.Code, apiErr.Error.Message)
		}
		return chatCompletionResponse{}, fmt.Errorf("llm api error: status=%d body=%s", resp.StatusCode, truncateString(string(respBody)))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return chatCompletionResponse{}, err
	}
	return result, nil
}

func (c *Client) createEmbeddings(ctx context.Context, req embeddingRequest) (embeddingResponse, error) {
	url := c.baseURL + "/embeddings"
	body, err := json.Marshal(req)
	if err != nil {
		return embeddingResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return embeddingResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return embeddingResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return embeddingResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return embeddingResponse{}, fmt.Errorf("llm api error: status=%d type=%s code=%v message=%s", resp.StatusCode, apiErr.Error.Type, apiErr.Error.Code, apiErr.Error.Message)
		}
		return embeddingResponse{}, fmt.Errorf("llm api error: status=%d body=%s", resp.StatusCode, truncateString(string(respBody)))
	}

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return embeddingResponse{}, err
	}
	return result, nil
}

func boolPtr(value bool) *bool {
	return &value
}
