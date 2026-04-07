package salutespeech

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/config"
)

type Client struct {
	cfg                  config.SaluteSpeechConfig
	httpClient           *http.Client
	mu                   sync.Mutex
	accessToken          string
	accessTokenExpiresAt time.Time
	logger               *zap.Logger
}

func NewClient(cfg config.SaluteSpeechConfig, lg *zap.Logger) *Client {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	c := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
		logger: lg,
	}

	if lg != nil {
		SetLogger(&zapLogger{lg})
	}

	return c
}

type zapLogger struct {
	l *zap.Logger
}

func (z *zapLogger) Info(msg string, keysAndValues ...interface{}) {
	z.l.Sugar().Infow(msg, keysAndValues...)
}

func (z *zapLogger) Error(msg string, keysAndValues ...interface{}) {
	z.l.Sugar().Errorw(msg, keysAndValues...)
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
