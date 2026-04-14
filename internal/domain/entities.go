package domain

import "time"

type User struct {
	ID         int64
	TelegramID int64
	CreatedAt  time.Time
}

type Meeting struct {
	ID            int64
	UserID        int64
	CreatedAt     time.Time
	AudioFileID   string
	TranscriptRaw string
	Transcript    string
	Summary       string
}
