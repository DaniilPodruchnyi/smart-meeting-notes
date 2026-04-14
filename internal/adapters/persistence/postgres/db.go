package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"smart-meeting-notes/internal/config"
)

type DB struct {
	*pgxpool.Pool
}

func New(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	maxConnLifetime, err := time.ParseDuration(cfg.MaxConnLifetime)
	if err != nil {
		return nil, fmt.Errorf("parse max conn lifetime: %w", err)
	}

	maxConnIdleTime, err := time.ParseDuration(cfg.MaxConnIdleTime)
	if err != nil {
		return nil, fmt.Errorf("parse max conn idle time: %w", err)
	}

	healthCheckPeriod, err := time.ParseDuration(cfg.HealthCheckTime)
	if err != nil {
		return nil, fmt.Errorf("parse health check period: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = maxConnLifetime
	poolCfg.MaxConnIdleTime = maxConnIdleTime
	poolCfg.HealthCheckPeriod = healthCheckPeriod

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) Migrate(ctx context.Context) error {
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
		transcript_raw TEXT,
		transcript TEXT,
		summary TEXT,
		transcript_emb DOUBLE PRECISION[],
		summary_emb DOUBLE PRECISION[]
	);

	ALTER TABLE meetings ADD COLUMN IF NOT EXISTS transcript_raw TEXT;
	ALTER TABLE meetings ADD COLUMN IF NOT EXISTS transcript_emb DOUBLE PRECISION[];
	ALTER TABLE meetings ADD COLUMN IF NOT EXISTS summary_emb DOUBLE PRECISION[];
	UPDATE meetings SET transcript_raw = transcript WHERE transcript_raw IS NULL;

	CREATE INDEX IF NOT EXISTS idx_meetings_user_id ON meetings(user_id);
	CREATE INDEX IF NOT EXISTS idx_meetings_transcript_trgm ON meetings USING gin (transcript gin_trgm_ops);
	CREATE INDEX IF NOT EXISTS idx_meetings_transcript_raw_trgm ON meetings USING gin (transcript_raw gin_trgm_ops);
	`
	_, err := db.Exec(ctx, schema)
	return err
}
