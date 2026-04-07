package domain

import "time"

// MeetingSemanticHit — результат семантического поиска (для /find smart + pgvector).
type MeetingSemanticHit struct {
	MeetingID string
	Score     float64 // чем выше, тем ближе по смыслу (косинус / 1-distance — как решит репозиторий)
	CreatedAt time.Time
	Snippet   string // опционально: превью текста из репозитория
}
