package salutespeech

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/config"
	"smart-meeting-notes/internal/pkg/httptls"
)

type Client struct {
	cfg                  config.SaluteSpeechConfig
	httpClient           *http.Client
	mu                   sync.Mutex
	accessToken          string
	accessTokenExpiresAt time.Time
	logger               *zap.Logger
}

type TranscriptResult struct {
	RawText        string
	TextBySpeakers string
}

func NewClient(cfg config.SaluteSpeechConfig, tlsCfg config.TLSConfig, lg *zap.Logger) *Client {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	c := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: httptls.NewTransport(tlsCfg.InsecureSkipVerify),
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
	result, err := c.transcribe(ctx, audio, contentType)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.RawText) != "" {
		return result.RawText, nil
	}
	return strings.TrimSpace(result.TextBySpeakers), nil
}

func (c *Client) TranscribeDetailed(ctx context.Context, audio []byte, contentType string) (TranscriptResult, error) {
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
