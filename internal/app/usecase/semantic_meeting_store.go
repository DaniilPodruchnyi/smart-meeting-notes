package usecase

import (
	"context"

	"smart-meeting-notes/internal/domain"
)

// SemanticMeetingStore — хранение и поиск эмбеддингов встреч (реализация: PostgreSQL + pgvector).
type SemanticMeetingStore interface {
	// SearchSemantic — top-N встреч пользователя по близости embedding к query-вектору.
	SearchSemantic(ctx context.Context, telegramUserID int64, queryEmbedding []float64, limit int) ([]domain.MeetingSemanticHit, error)

	// UpsertMeetingEmbedding — сохранить/обновить вектор для встречи (после транскрипции).
	UpsertMeetingEmbedding(ctx context.Context, meetingID string, telegramUserID int64, embedding []float64, sourceTextHash string) error
}
