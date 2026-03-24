package usecase

import "context"

type PingService struct{}

func NewPingService() *PingService {
	return &PingService{}
}

func (s *PingService) Ping(ctx context.Context) (string, error) {
	_ = ctx // для будущих usecase с контекстом/транзакциями
	return "pong", nil
}
