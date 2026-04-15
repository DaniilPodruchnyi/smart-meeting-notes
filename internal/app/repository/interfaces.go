package repository

import (
	"context"

	"smart-meeting-notes/internal/domain"
)

type MeetingRepository interface {
	Create(ctx context.Context, meeting *domain.Meeting) error
	GetByID(ctx context.Context, id int64) (*domain.Meeting, error)
	GetByUserID(ctx context.Context, userID int64) ([]domain.Meeting, error)
	Search(ctx context.Context, userID int64, query string) ([]domain.Meeting, error)
	SmartSearch(ctx context.Context, userID int64, queryEmbedding []float64, minScore float64, limit int) ([]domain.Meeting, error)
	UpdateTranscript(ctx context.Context, id int64, transcript string) error
	UpdateSummary(ctx context.Context, id int64, summary string) error
}

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
}
