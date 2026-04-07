package usecase

import "context"

// Embedder — векторизация текста (GigaChat / другие модели).
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}
