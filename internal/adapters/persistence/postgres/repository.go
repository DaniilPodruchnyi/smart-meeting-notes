package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"smart-meeting-notes/internal/domain"
)

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	schema := `
	CREATE EXTENSION IF NOT EXISTS pg_trgm;

	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		telegram_id BIGINT UNIQUE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS meetings (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id),
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		audio_file_id VARCHAR(255),
		transcript TEXT,
		summary TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_meetings_user_id ON meetings(user_id);
	CREATE INDEX IF NOT EXISTS idx_meetings_transcript_trgm ON meetings USING gin (transcript gin_trgm_ops);
	`
	_, err := db.Exec(schema)
	return err
}

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (telegram_id, created_at) VALUES ($1, $2) RETURNING id`
	return r.db.QueryRowContext(ctx, query, user.TelegramID, time.Now()).Scan(&user.ID)
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	user := &domain.User{}
	query := `SELECT id, telegram_id, created_at FROM users WHERE telegram_id = $1`
	err := r.db.QueryRowContext(ctx, query, telegramID).Scan(&user.ID, &user.TelegramID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

type MeetingRepo struct {
	db *DB
}

func NewMeetingRepo(db *DB) *MeetingRepo {
	return &MeetingRepo{db: db}
}

func (r *MeetingRepo) Create(ctx context.Context, meeting *domain.Meeting) error {
	query := `INSERT INTO meetings (user_id, created_at, audio_file_id, transcript, summary) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.db.QueryRowContext(ctx, query, meeting.UserID, time.Now(), meeting.AudioFileID, meeting.Transcript, meeting.Summary).Scan(&meeting.ID)
}

func (r *MeetingRepo) GetByID(ctx context.Context, id int64) (*domain.Meeting, error) {
	m := &domain.Meeting{}
	query := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&m.ID, &m.UserID, &m.CreatedAt, &m.AudioFileID, &m.Transcript, &m.Summary)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *MeetingRepo) GetByUserID(ctx context.Context, userID int64) ([]domain.Meeting, error) {
	query := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meetings []domain.Meeting
	for rows.Next() {
		var m domain.Meeting
		if err := rows.Scan(&m.ID, &m.UserID, &m.CreatedAt, &m.AudioFileID, &m.Transcript, &m.Summary); err != nil {
			return nil, err
		}
		meetings = append(meetings, m)
	}
	return meetings, nil
}

func (r *MeetingRepo) Search(ctx context.Context, userID int64, query string) ([]domain.Meeting, error) {
	sqlQuery := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE user_id = $1 AND transcript ILIKE $2 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, sqlQuery, userID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meetings []domain.Meeting
	for rows.Next() {
		var m domain.Meeting
		if err := rows.Scan(&m.ID, &m.UserID, &m.CreatedAt, &m.AudioFileID, &m.Transcript, &m.Summary); err != nil {
			return nil, err
		}
		meetings = append(meetings, m)
	}
	return meetings, nil
}

func (r *MeetingRepo) UpdateTranscript(ctx context.Context, id int64, transcript string) error {
	query := `UPDATE meetings SET transcript = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, transcript, id)
	return err
}

func (r *MeetingRepo) UpdateSummary(ctx context.Context, id int64, summary string) error {
	query := `UPDATE meetings SET summary = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, summary, id)
	return err
}
