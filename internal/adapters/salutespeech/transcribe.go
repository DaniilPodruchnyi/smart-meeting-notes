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

func (c *Client) transcribe(ctx context.Context, audio []byte, contentType string) (string, error) {
	if len(audio) == 0 {
		return "", errors.New("salutespeech: empty audio payload")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	requestFileID, err := c.upload(ctx, audio, contentType)
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
		return "", err
	}
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
	u, err := url.Parse(c.endpoint(c.cfg.StatusPath))
	if err != nil {
		return "", "", fmt.Errorf("salutespeech: parse status endpoint: %w", err)
	}
	q := u.Query()
	q.Set("id", taskID)
	u.RawQuery = q.Encode()

	var env apiEnvelope[statusResult]
	if err := c.doAPI(ctx, http.MethodGet, u.String(), nil, "application/json", &env, "status request"); err != nil {
		return "", "", err
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
			return strings.TrimSpace(v), nil
		default:
			raw, mErr := json.Marshal(v)
			if mErr != nil {
				return "", fmt.Errorf("salutespeech: marshal result payload: %w", mErr)
			}
			return strings.TrimSpace(string(raw)), nil
		}
	}

	return strings.TrimSpace(string(body)), nil
}
