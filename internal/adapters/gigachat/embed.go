package gigachat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Embed возвращает вектор для каждого элемента texts.
// Пустые строки после Trim дают nil-срез на соответствующей позиции; если все пустые — ошибка.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	type slot struct {
		orig int
		text string
	}
	var batch []slot
	out := make([][]float64, len(texts))

	for i, t := range texts {
		s := strings.TrimSpace(t)
		if s == "" {
			continue
		}
		batch = append(batch, slot{orig: i, text: s})
	}
	if len(batch) == 0 {
		return nil, errors.New("gigachat: no non-empty texts to embed")
	}

	payloadTexts := make([]string, len(batch))
	for i := range batch {
		payloadTexts[i] = batch[i].text
	}

	model := strings.TrimSpace(c.cfg.EmbeddingsModel)
	if model == "" {
		model = "Embeddings"
	}

	body, err := json.Marshal(EmbeddingsRequest{
		Model: model,
		Input: payloadTexts,
	})
	if err != nil {
		return nil, fmt.Errorf("gigachat: marshal embeddings request: %w", err)
	}

	var resp EmbeddingsResponse
	url := c.endpoint(c.cfg.EmbeddingsPath)
	if err := c.doJSON(ctx, http.MethodPost, url, body, &resp, "embeddings"); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("gigachat: empty embeddings data")
	}

	byIndex := make(map[int][]float64, len(resp.Data))
	for _, d := range resp.Data {
		if len(d.Embedding) == 0 {
			return nil, fmt.Errorf("gigachat: empty embedding vector at index %d", d.Index)
		}
		byIndex[d.Index] = d.Embedding
	}

	for i := range batch {
		vec, ok := byIndex[i]
		if !ok {
			return nil, fmt.Errorf("gigachat: missing embedding for batch index %d", i)
		}
		out[batch[i].orig] = vec
	}

	return out, nil
}
