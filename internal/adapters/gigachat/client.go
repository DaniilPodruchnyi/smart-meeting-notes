package gigachat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"smart-meeting-notes/internal/config"
)

type Client struct {
	cfg                  config.GigaChatConfig
	httpClient           *http.Client
	mu                   sync.Mutex
	accessToken          string
	accessTokenExpiresAt time.Time
}

func NewClient(cfg config.GigaChatConfig) *Client {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) endpoint(path string) string {
	base := strings.TrimRight(c.cfg.APIBaseURL, "/")
	if path == "" {
		return base
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// Chat — единственная точка вызова GigaChat под ТЗ: саммари транскрипта, /chat по контексту и т.д.
// systemPrompt — инструкция роли и формата ответа (пустая: в запросе только user, например свободный диалог).
// userMessage — контент пользователя: транскрипт, вопрос, уточнение (обязателен).
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return "", errors.New("gigachat: empty user message")
	}

	msgs := make([]Message, 0, 2)
	if s := strings.TrimSpace(systemPrompt); s != "" {
		msgs = append(msgs, Message{Role: "system", Content: s})
	}
	msgs = append(msgs, Message{Role: "user", Content: userMessage})

	reqBody := ChatCompletionRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		Temperature: c.cfg.Temperature,
		TopP:        c.cfg.TopP,
		MaxTokens:   c.cfg.MaxTokens,
		Stream:      false,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("gigachat: marshal request: %w", err)
	}

	var resp ChatCompletionResponse
	url := c.endpoint(c.cfg.ChatCompletionsPath)
	if err := c.doJSON(ctx, http.MethodPost, url, payload, &resp, "chat completions"); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("gigachat: empty choices in response")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("gigachat: empty assistant content")
	}
	return content, nil
}
