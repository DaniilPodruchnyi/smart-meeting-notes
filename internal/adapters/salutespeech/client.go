package salutespeech

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"smart-meeting-notes/internal/config"
)

type Client struct {
	cfg                  config.SaluteSpeechConfig
	httpClient           *http.Client
	mu                   sync.Mutex
	accessToken          string
	accessTokenExpiresAt time.Time
}

func NewClient(cfg config.SaluteSpeechConfig) *Client {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Transcribe проходит 4 шага async распознавания:
// upload -> create task -> polling status -> download result.
func (c *Client) Transcribe(ctx context.Context, audio []byte, contentType string) (string, error) {
	return c.transcribe(ctx, audio, contentType)
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
