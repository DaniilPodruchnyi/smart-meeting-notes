package salutespeech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var logger Logger

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

func SetLogger(l Logger) {
	logger = l
}

func logInfo(msg string, keysAndValues ...interface{}) {
	if logger != nil {
		logger.Info(msg, keysAndValues...)
	}
}

func logError(msg string, keysAndValues ...interface{}) {
	if logger != nil {
		logger.Error(msg, keysAndValues...)
	}
}

func (c *Client) transcribe(ctx context.Context, audio []byte, contentType string) (string, error) {
	logInfo("starting transcribe", "audio_size", len(audio), "content_type", contentType)

	if len(audio) == 0 {
		return "", errors.New("salutespeech: empty audio payload")
	}
	if contentType == "" {
		contentType = "audio/wav"
	}

	requestFileID, err := c.upload(ctx, audio, contentType)
	logInfo("uploaded", "request_file_id", requestFileID, "error", err)
	if err != nil {
		return "", err
	}

	taskID, err := c.createRecognitionTask(ctx, requestFileID)
	if err != nil {
		return "", err
	}

	responseFileID, err := c.waitTaskDone(ctx, taskID)
	if err != nil {
		return "", err
	}

	return c.downloadResult(ctx, responseFileID)
}

func (c *Client) upload(ctx context.Context, audio []byte, contentType string) (string, error) {
	var env apiEnvelope[uploadResult]
	if err := c.doAPI(ctx, http.MethodPost, c.endpoint(c.cfg.UploadPath), audio, contentType, &env, "upload"); err != nil {
		return "", err
	}
	if env.Result.RequestFileID == "" {
		return "", errors.New("salutespeech: request_file_id is empty")
	}
	return env.Result.RequestFileID, nil
}

func (c *Client) createRecognitionTask(ctx context.Context, requestFileID string) (string, error) {
	logInfo("creating task", "request_file_id", requestFileID)

	payload, err := json.Marshal(recognizeRequest{
		RequestFileID: requestFileID,
		Options: recognizeOptions{
			Model:         c.cfg.Model,
			Language:      c.cfg.Language,
			AudioEncoding: c.cfg.AudioEncoding,
			SampleRate:    c.cfg.SampleRate,
			ChannelsCount: c.cfg.ChannelsCount,
		},
	})
	if err != nil {
		return "", fmt.Errorf("salutespeech: marshal recognize request: %w", err)
	}

	var env apiEnvelope[recognizeResult]
	if err := c.doAPI(ctx, http.MethodPost, c.endpoint(c.cfg.RecognizePath), payload, "application/json", &env, "create task"); err != nil {
		logError("create task error", "error", err)
		return "", err
	}

	logInfo("task created", "task_id", env.Result.ID, "status", env.Result.Status)
	if env.Result.ID == "" {
		return "", errors.New("salutespeech: task id is empty")
	}
	return env.Result.ID, nil
}

func (c *Client) waitTaskDone(ctx context.Context, taskID string) (string, error) {
	pollInterval := c.cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

	pollTimeout := c.cfg.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = 2 * time.Minute
	}

	deadline := time.Now().Add(pollTimeout)
	for {
		if time.Now().After(deadline) {
			return "", errors.New("salutespeech: task polling timeout exceeded")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}

		responseFileID, status, err := c.getTaskStatus(ctx, taskID)
		if err != nil {
			return "", err
		}

		switch strings.ToUpper(status) {
		case "DONE":
			if responseFileID == "" {
				return "", errors.New("salutespeech: response_file_id is empty for DONE task")
			}
			return responseFileID, nil
		case "ERROR", "CANCELED", "CANCELLED":
			return "", fmt.Errorf("salutespeech: task ended with status %s", status)
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) getTaskStatus(ctx context.Context, taskID string) (responseFileID, status string, err error) {
	logInfo("checking status", "task_id", taskID)

	u, err := url.Parse(c.endpoint(c.cfg.StatusPath))
	if err != nil {
		return "", "", fmt.Errorf("salutespeech: parse status endpoint: %w", err)
	}
	q := u.Query()
	q.Set("id", taskID)
	u.RawQuery = q.Encode()

	var env apiEnvelope[statusResult]
	if err := c.doAPI(ctx, http.MethodGet, u.String(), nil, "application/json", &env, "status request"); err != nil {
		logError("status error", "error", err)
		return "", "", err
	}

	logInfo("status result", "task_id", taskID, "status", env.Result.Status, "response_file_id", env.Result.ResponseFileID, "error", env.Error)

	if env.Error != "" {
		return "", "", fmt.Errorf("salutespeech API error: %s", env.Error)
	}

	return env.Result.ResponseFileID, env.Result.Status, nil
}

func (c *Client) downloadResult(ctx context.Context, responseFileID string) (string, error) {
	u, err := url.Parse(c.endpoint(c.cfg.DownloadPath))
	if err != nil {
		return "", fmt.Errorf("salutespeech: parse download endpoint: %w", err)
	}
	q := u.Query()
	q.Set("response_file_id", responseFileID)
	u.RawQuery = q.Encode()

	req, err := c.authorizedRequest(ctx, http.MethodGet, u.String(), nil, "application/json")
	if err != nil {
		return "", err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("salutespeech: download request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("salutespeech: read download response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("salutespeech: download request bad status: %s, body: %s", res.Status, strings.TrimSpace(string(body)))
	}

	// Формат ответа в download может отличаться:
	// 1) {"status":200,"result":...}
	// 2) сразу JSON-массив/объект c результатом распознавания.
	var envAny apiEnvelope[any]
	if err := json.Unmarshal(body, &envAny); err == nil && envAny.Result != nil {
		switch v := envAny.Result.(type) {
		case string:
			return extractTextFromResponse(v), nil
		default:
			raw, mErr := json.Marshal(v)
			if mErr != nil {
				return "", fmt.Errorf("salutespeech: marshal result payload: %w", mErr)
			}
			return extractTextFromResponse(string(raw)), nil
		}
	}

	return extractTextFromResponse(string(body)), nil
}

// extractTextFromResponse извлекает текст из ответа SaluteSpeech API
func extractTextFromResponse(response string) string {
	response = strings.TrimSpace(response)

	// Проверяем, начинается ли с массива
	if strings.HasPrefix(response, "[") {
		// Это массив результатов
		var results []struct {
			Results []struct {
				Text       string `json:"text"`
				Normalized string `json:"normalized_text"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(response), &results); err == nil {
			var text strings.Builder
			for _, r := range results {
				for _, item := range r.Results {
					if item.Text != "" {
						text.WriteString(item.Text)
						text.WriteString(" ")
					}
					if item.Normalized != "" && item.Text == "" {
						text.WriteString(item.Normalized)
						text.WriteString(" ")
					}
				}
			}
			if text.Len() > 0 {
				return strings.TrimSpace(text.String())
			}
		}
	} else if strings.HasPrefix(response, "{") {
		// Это объект
		var result struct {
			Results []struct {
				Text       string `json:"text"`
				Normalized string `json:"normalized_text"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(response), &result); err == nil {
			var text strings.Builder
			for _, r := range result.Results {
				if r.Text != "" {
					text.WriteString(r.Text)
					text.WriteString(" ")
				}
				if r.Normalized != "" && r.Text == "" {
					text.WriteString(r.Normalized)
					text.WriteString(" ")
				}
			}
			if text.Len() > 0 {
				return strings.TrimSpace(text.String())
			}
		}
	}

	// Если не удалось распарсить, возвращаем как есть
	return response
}
