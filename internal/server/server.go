package server

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/app/usecase"
	"smart-meeting-notes/internal/config"
	httptransport "smart-meeting-notes/internal/server/transport/http"
)

type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
}

func New(cfg config.Config, lg *zap.Logger, pingSvc *usecase.PingService) *Server {
	router := httptransport.NewRouter(pingSvc)

	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.HTTPAddress,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: lg,
	}
}

// Run запускает HTTP сервер и корректно останавливает его при отмене ctx.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("HTTP server starting", zap.String("addr", s.httpServer.Addr))
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("HTTP server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		// ListenAndServe возвращает ErrServerClosed при Shutdown.
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
