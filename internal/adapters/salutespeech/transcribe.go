package salutespeech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

type sttResult struct {
	Text       string `json:"text"`
	Normalized string `json:"normalized_text"`
}

type sttSpeakerInfo struct {
	SpeakerID int `json:"speaker_id"`
}

type sttChunk struct {
	Results     []sttResult    `json:"results"`
	SpeakerInfo sttSpeakerInfo `json:"speaker_info"`
}

func (c *Client) transcribe(ctx context.Context, audio []byte, contentType string) (TranscriptResult, error) {
	logInfo("starting transcribe", "audio_size", len(audio), "content_type", contentType)

	if len(audio) == 0 {
		return TranscriptResult{}, errors.New("salutespeech: empty audio payload")
	}
	if contentType == "" {
		contentType = "audio/wav"
	}

	requestFileID, err := c.upload(ctx, audio, contentType)
	logInfo("uploaded", "request_file_id", requestFileID, "error", err)
	if err != nil {
		return TranscriptResult{}, err
	}

	taskID, err := c.createRecognitionTask(ctx, requestFileID)
	if err != nil {
		return TranscriptResult{}, err
	}

	responseFileID, err := c.waitTaskDone(ctx, taskID)
	if err != nil {
		return TranscriptResult{}, err
	}

	result, err := c.downloadResult(ctx, responseFileID)
	if err != nil {
		return TranscriptResult{}, err
	}
	logInfo(
		"transcribe result prepared",
		"raw_len", len(result.RawText),
		"roles_len", len(result.TextBySpeakers),
		"speaker_count", countSpeakers(result.TextBySpeakers),
	)
	return result, nil
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

	options := recognizeOptions{
		Model:         c.cfg.Model,
		Language:      c.cfg.Language,
		AudioEncoding: c.cfg.AudioEncoding,
		SampleRate:    c.cfg.SampleRate,
		ChannelsCount: c.cfg.ChannelsCount,
	}
	if c.cfg.SpeakerSeparationEnabled {
		options.SpeakerSeparationOptions = &speakerSeparationOptions{
			Enable:                true,
			EnableOnlyMainSpeaker: false,
			Count:                 clampSpeakerCount(c.cfg.SpeakerMaxCount),
		}
	}

	payload, err := json.Marshal(recognizeRequest{
		RequestFileID: requestFileID,
		Options:       options,
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

func clampSpeakerCount(n int) int {
	if n < 1 {
		return 1
	}
	if n > 10 {
		return 10
	}
	return n
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

func (c *Client) downloadResult(ctx context.Context, responseFileID string) (TranscriptResult, error) {
	u, err := url.Parse(c.endpoint(c.cfg.DownloadPath))
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("salutespeech: parse download endpoint: %w", err)
	}
	q := u.Query()
	q.Set("response_file_id", responseFileID)
	u.RawQuery = q.Encode()

	req, err := c.authorizedRequest(ctx, http.MethodGet, u.String(), nil, "application/json")
	if err != nil {
		return TranscriptResult{}, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("salutespeech: download request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("salutespeech: read download response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return TranscriptResult{}, fmt.Errorf("salutespeech: download request bad status: %s, body: %s", res.Status, strings.TrimSpace(string(body)))
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
				return TranscriptResult{}, fmt.Errorf("salutespeech: marshal result payload: %w", mErr)
			}
			return extractTextFromResponse(string(raw)), nil
		}
	}

	return extractTextFromResponse(string(body)), nil
}

// extractTextFromResponse извлекает сразу два вида транскрипции:
// raw text и текст, сгруппированный по спикерам.
func extractTextFromResponse(response string) TranscriptResult {
	response = strings.TrimSpace(response)
	if response == "" {
		return TranscriptResult{}
	}

	parsedKnownJSON := false

	// Проверяем, начинается ли с массива
	if strings.HasPrefix(response, "[") {
		// Это массив результатов; для него чаще всего доступны speaker_id по сегментам.
		var results []sttChunk
		if err := json.Unmarshal([]byte(response), &results); err == nil {
			parsedKnownJSON = true
			return TranscriptResult{
				RawText:        formatRawTranscript(results),
				TextBySpeakers: formatTranscriptBySpeakers(results),
			}
		}
	} else if strings.HasPrefix(response, "{") {
		// Это объект
		var result sttChunk
		if err := json.Unmarshal([]byte(response), &result); err == nil {
			parsedKnownJSON = true
			chunks := []sttChunk{result}
			return TranscriptResult{
				RawText:        formatRawTranscript(chunks),
				TextBySpeakers: formatTranscriptBySpeakers(chunks),
			}
		}
	}

	// Если это JSON-ответ, но не удалось извлечь распознанный текст, возвращаем пустую строку.
	// Это лучше, чем отправлять пользователю сырой технический payload.
	if parsedKnownJSON || strings.HasPrefix(response, "[") || strings.HasPrefix(response, "{") {
		return TranscriptResult{}
	}

	// Если не удалось распарсить, считаем это сырым текстом.
	return TranscriptResult{RawText: response}
}

func formatTranscriptBySpeakers(chunks []sttChunk) string {
	var lines []string
	var currentSpeaker string
	var currentText strings.Builder

	flush := func() {
		text := strings.TrimSpace(currentText.String())
		if text == "" {
			currentText.Reset()
			return
		}
		if currentSpeaker == "" {
			lines = append(lines, text)
		} else {
			lines = append(lines, currentSpeaker+": "+text)
		}
		currentText.Reset()
	}

	for _, chunk := range chunks {
		segmentText := extractSegmentText(chunk.Results)
		if segmentText == "" {
			continue
		}

		speaker := speakerLabel(chunk.SpeakerInfo.SpeakerID)
		if speaker != currentSpeaker {
			flush()
			currentSpeaker = speaker
		}
		if currentText.Len() > 0 {
			currentText.WriteString(" ")
		}
		currentText.WriteString(segmentText)
	}
	flush()

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatRawTranscript(chunks []sttChunk) string {
	var text strings.Builder
	for _, chunk := range chunks {
		segmentText := extractSegmentText(chunk.Results)
		if segmentText == "" {
			continue
		}
		if text.Len() > 0 {
			text.WriteString(" ")
		}
		text.WriteString(segmentText)
	}
	return strings.TrimSpace(text.String())
}

func extractSegmentText(results []sttResult) string {
	var text strings.Builder
	for _, item := range results {
		switch {
		case item.Text != "":
			text.WriteString(item.Text)
			text.WriteString(" ")
		case item.Normalized != "":
			text.WriteString(item.Normalized)
			text.WriteString(" ")
		}
	}
	return strings.TrimSpace(text.String())
}

func speakerLabel(speakerID int) string {
	if speakerID < 0 {
		return "[неопределен]"
	}
	return "Спикер " + strconv.Itoa(speakerID+1)
}

func countSpeakers(transcriptByRoles string) int {
	lines := strings.Split(transcriptByRoles, "\n")
	unique := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		label := strings.TrimSpace(line[:colon])
		if strings.HasPrefix(label, "Спикер") {
			unique[label] = struct{}{}
		}
	}
	return len(unique)
}
