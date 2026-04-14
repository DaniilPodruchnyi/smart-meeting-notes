package gigachat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"smart-meeting-notes/internal/config"
	"smart-meeting-notes/internal/pkg/httptls"

	"github.com/google/uuid"
)

// generateUUID генерирует UUID для RqUID заголовка
func generateUUID() string {
	return uuid.New().String()
}

// Client клиент для работы с GigaChat API
type Client struct {
	cfg         config.GigaChatConfig
	httpClient  *http.Client
	accessToken string
	expiresAt   time.Time
}

// New создает новый экземпляр GigaChat клиента
func New(cfg config.GigaChatConfig, tlsCfg config.TLSConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: httptls.NewTransport(tlsCfg.InsecureSkipVerify),
		},
	}
}

// getToken получает токен доступа
func (c *Client) getToken(ctx context.Context) (string, error) {
	// Проверяем валидность текущего токена
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-time.Minute)) {
		return c.accessToken, nil
	}

	auth := c.cfg.APIKey
	if auth == "" {
		return "", fmt.Errorf("API ключ пуст - проверь переменную GIGACHAT_API_KEY")
	}

	auth = strings.TrimSpace(auth)
	auth = strings.TrimPrefix(auth, "Basic ")
	auth = strings.TrimPrefix(auth, "basic ")

	// Проверяем что ключ не пустой после обработки
	if auth == "" {
		return "", fmt.Errorf("API ключ пустой после обработки")
	}

	// URL для получения токена GigaChat
	authURL := "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"

	// Формируем данные запроса
	form := url.Values{}
	form.Set("scope", "GIGACHAT_API_PERS")

	// Генерируем RqUID (обязательный заголовок UUID)
	rqUID := generateUUID()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Add("RqUID", rqUID)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос авторизации не удался: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != 200 {
		return "", fmt.Errorf("авторизация не удалась: %s, body: '%s'", res.Status, string(body))
	}

	if string(body) == "" {
		return "", fmt.Errorf("пустой ответ от сервера авторизации")
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("декодирование токена: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.expiresAt = time.UnixMilli(tokenResp.ExpiresAt)
	return c.accessToken, nil
}

// Message структура сообщения для чата
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest структура запроса к чату
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse структура ответа чата
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// Chat отправляет запрос к GigaChat и возвращает ответ
func (c *Client) Chat(ctx context.Context, prompt string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", fmt.Errorf("получение токена: %w", err)
	}

	apiURL := "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
	payload, _ := json.Marshal(ChatRequest{
		Model: "GigaChat",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос не удался: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != 200 {
		return "", fmt.Errorf("запрос к чату не удался: %s, body: %s", res.Status, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("декодирование ответа: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("нет ответа от GigaChat")
	}

	return chatResp.Choices[0].Message.Content, nil
}
