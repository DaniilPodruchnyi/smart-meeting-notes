package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"smart-meeting-notes/internal/domain"
)

type UserRepo struct {
	repo *Repository[domain.User]
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{repo: newRepository[domain.User](db)}
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (telegram_id, created_at) VALUES ($1, $2) RETURNING id`
	return r.repo.create(ctx, query, &user.ID, user.TelegramID, time.Now())
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	query := `SELECT id, telegram_id, created_at FROM users WHERE telegram_id = $1`
	return r.repo.getOne(ctx, query, scanUserRow, telegramID)
}

func scanUserRow(row pgx.Row) (*domain.User, error) {
	user := &domain.User{}
	if err := row.Scan(&user.ID, &user.TelegramID, &user.CreatedAt); err != nil {
		return nil, err
	}
	return user, nil
}
