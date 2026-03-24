package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"smart-meeting-notes/internal/config"
)

// New создает logger на базе zap.
func New(cfg config.Config) (*zap.Logger, error) {
	level := strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	if level == "" {
		level = "info"
	}

	var zapLevel zapcore.Level
	if err := zapLevel.Set(level); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zapLevel)

	// ProductionConfig уже включает JSON-логирование и корректные уровни.
	return zapCfg.Build()
}
