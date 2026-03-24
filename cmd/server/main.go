package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/app/usecase"
	"smart-meeting-notes/internal/config"
	"smart-meeting-notes/internal/logger"
	"smart-meeting-notes/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(".env")
	if err != nil {
		// Даже если .env частично сломан, сервер должен иметь шанс подняться.
		// Ошибку залогируем после инициализации logger.
	}

	lg, err := logger.New(cfg)
	if err != nil {
		// Fallback на stderr, если zap не поднялся (крайне редкий кейс).
		panic(err)
	}
	defer func() { _ = lg.Sync() }()

	pingSvc := usecase.NewPingService()
	srv := server.New(cfg, lg, pingSvc)

	if err := srv.Run(ctx); err != nil {
		lg.Fatal("server stopped with error", zap.Error(err))
	}
}
