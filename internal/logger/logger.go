package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"smart-meeting-notes/internal/config"
)

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

	return zapCfg.Build()
}
