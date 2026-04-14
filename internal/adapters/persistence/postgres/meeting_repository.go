package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"smart-meeting-notes/internal/domain"
)

type MeetingRepo struct {
	repo *Repository[domain.Meeting]
}

func NewMeetingRepo(db *DB) *MeetingRepo {
	return &MeetingRepo{repo: newRepository[domain.Meeting](db)}
}

func (r *MeetingRepo) Create(ctx context.Context, meeting *domain.Meeting) error {
	query := `INSERT INTO meetings (user_id, created_at, audio_file_id, transcript, summary) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.repo.create(ctx, query, &meeting.ID, meeting.UserID, time.Now(), meeting.AudioFileID, meeting.Transcript, meeting.Summary)
}

func (r *MeetingRepo) GetByID(ctx context.Context, id int64) (*domain.Meeting, error) {
	query := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE id = $1`
	return r.repo.getOne(ctx, query, scanMeetingRow, id)
}

func (r *MeetingRepo) GetByUserID(ctx context.Context, userID int64) ([]domain.Meeting, error) {
	query := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE user_id = $1 ORDER BY created_at DESC`
	return r.repo.getMany(ctx, query, scanMeetingRows, userID)
}

func (r *MeetingRepo) Search(ctx context.Context, userID int64, query string) ([]domain.Meeting, error) {
	sqlQuery := `SELECT id, user_id, created_at, audio_file_id, transcript, summary FROM meetings WHERE user_id = $1 AND transcript ILIKE $2 ORDER BY created_at DESC`
	return r.repo.getMany(ctx, sqlQuery, scanMeetingRows, userID, "%"+query+"%")
}

func (r *MeetingRepo) UpdateTranscript(ctx context.Context, id int64, transcript string) error {
	query := `UPDATE meetings SET transcript = $1 WHERE id = $2`
	_, err := r.repo.db.Exec(ctx, query, transcript, id)
	return err
}

func (r *MeetingRepo) UpdateSummary(ctx context.Context, id int64, summary string) error {
	query := `UPDATE meetings SET summary = $1 WHERE id = $2`
	_, err := r.repo.db.Exec(ctx, query, summary, id)
	return err
}

func scanMeetingRow(row pgx.Row) (*domain.Meeting, error) {
	meeting := &domain.Meeting{}
	if err := row.Scan(&meeting.ID, &meeting.UserID, &meeting.CreatedAt, &meeting.AudioFileID, &meeting.Transcript, &meeting.Summary); err != nil {
		return nil, err
	}
	return meeting, nil
}

func scanMeetingRows(rows pgx.Rows) (domain.Meeting, error) {
	var meeting domain.Meeting
	if err := rows.Scan(&meeting.ID, &meeting.UserID, &meeting.CreatedAt, &meeting.AudioFileID, &meeting.Transcript, &meeting.Summary); err != nil {
		return domain.Meeting{}, err
	}
	return meeting, nil
}
