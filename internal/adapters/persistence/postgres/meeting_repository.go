package postgres

import (
	"context"
	"fmt"
	"strings"
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
	query := `
INSERT INTO meetings (user_id, created_at, audio_file_id, transcript_raw, transcript, summary, transcript_emb, summary_emb)
VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $7 = '' THEN NULL ELSE $7::vector END, CASE WHEN $8 = '' THEN NULL ELSE $8::vector END)
RETURNING id
`
	return r.repo.create(
		ctx,
		query,
		&meeting.ID,
		meeting.UserID,
		time.Now(),
		meeting.AudioFileID,
		meeting.TranscriptRaw,
		meeting.Transcript,
		meeting.Summary,
		vectorLiteral(meeting.TranscriptEmb),
		vectorLiteral(meeting.SummaryEmb),
	)
}

func (r *MeetingRepo) GetByID(ctx context.Context, id int64) (*domain.Meeting, error) {
	query := `SELECT id, user_id, created_at, audio_file_id, transcript_raw, transcript, summary FROM meetings WHERE id = $1`
	return r.repo.getOne(ctx, query, scanMeetingRow, id)
}

func (r *MeetingRepo) GetByUserID(ctx context.Context, userID int64) ([]domain.Meeting, error) {
	query := `SELECT id, user_id, created_at, audio_file_id, transcript_raw, transcript, summary FROM meetings WHERE user_id = $1 ORDER BY created_at DESC`
	return r.repo.getMany(ctx, query, scanMeetingRows, userID)
}

func (r *MeetingRepo) Search(ctx context.Context, userID int64, query string) ([]domain.Meeting, error) {
	sqlQuery := `
SELECT id, user_id, created_at, audio_file_id, transcript_raw, transcript, summary, transcript_emb, summary_emb
FROM meetings
WHERE user_id = $1
  AND (
    to_tsvector('russian', COALESCE(transcript_raw, transcript, '')) @@ plainto_tsquery('russian', $2)
    OR to_tsvector('russian', COALESCE(summary, '')) @@ plainto_tsquery('russian', $2)
  )
ORDER BY created_at DESC
`
	return r.repo.getMany(ctx, sqlQuery, scanMeetingRows, userID, query)
}

func (r *MeetingRepo) SmartSearch(ctx context.Context, userID int64, queryEmbedding []float64, minScore float64, limit int) ([]domain.Meeting, error) {
	sqlQuery := `
SELECT
	id,
	user_id,
	created_at,
	audio_file_id,
	transcript_raw,
	transcript,
	summary,
	COALESCE(1 - (summary_emb <=> $2::vector), -1) AS summary_score,
	COALESCE(1 - (transcript_emb <=> $2::vector), -1) AS transcript_score,
	GREATEST(
		COALESCE(1 - (summary_emb <=> $2::vector), -1),
		COALESCE(1 - (transcript_emb <=> $2::vector), -1)
	) AS semantic_score
FROM meetings
WHERE user_id = $1
  AND (summary_emb IS NOT NULL OR transcript_emb IS NOT NULL)
  AND GREATEST(
		COALESCE(1 - (summary_emb <=> $2::vector), -1),
		COALESCE(1 - (transcript_emb <=> $2::vector), -1)
	) >= $3
ORDER BY semantic_score DESC, created_at DESC
LIMIT $4
`
	return r.repo.getMany(ctx, sqlQuery, scanSmartMeetingRows, userID, vectorLiteral(queryEmbedding), minScore, limit)
}

func (r *MeetingRepo) UpdateTranscript(ctx context.Context, id int64, transcript string) error {
	query := `UPDATE meetings SET transcript = $1, transcript_raw = COALESCE(transcript_raw, $1) WHERE id = $2`
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
	if err := row.Scan(&meeting.ID, &meeting.UserID, &meeting.CreatedAt, &meeting.AudioFileID, &meeting.TranscriptRaw, &meeting.Transcript, &meeting.Summary); err != nil {
		return nil, err
	}
	return meeting, nil
}

func scanMeetingRows(rows pgx.Rows) (domain.Meeting, error) {
	var meeting domain.Meeting
	if err := rows.Scan(&meeting.ID, &meeting.UserID, &meeting.CreatedAt, &meeting.AudioFileID, &meeting.TranscriptRaw, &meeting.Transcript, &meeting.Summary); err != nil {
		return domain.Meeting{}, err
	}
	return meeting, nil
}

func scanSmartMeetingRows(rows pgx.Rows) (domain.Meeting, error) {
	var meeting domain.Meeting
	if err := rows.Scan(
		&meeting.ID,
		&meeting.UserID,
		&meeting.CreatedAt,
		&meeting.AudioFileID,
		&meeting.TranscriptRaw,
		&meeting.Transcript,
		&meeting.Summary,
		&meeting.SummaryScore,
		&meeting.TranscriptScore,
		&meeting.SemanticScore,
	); err != nil {
		return domain.Meeting{}, err
	}
	return meeting, nil
}

func vectorLiteral(vec []float64) string {
	if len(vec) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[")
	for i, v := range vec {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%g", v))
	}
	b.WriteString("]")
	return b.String()
}
