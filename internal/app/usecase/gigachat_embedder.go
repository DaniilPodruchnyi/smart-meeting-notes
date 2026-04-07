package usecase

import (
	"context"

	"smart-meeting-notes/internal/adapters/gigachat"
)

// GigaChatEmbedder реализует Embedder через клиент GigaChat POST /embeddings.
type GigaChatEmbedder struct {
	client *gigachat.Client
}

func NewGigaChatEmbedder(client *gigachat.Client) *GigaChatEmbedder {
	return &GigaChatEmbedder{client: client}
}

func (e *GigaChatEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return e.client.Embed(ctx, texts)
}
