package queue

import (
	"context"
	"sync"
	"sync/atomic"
)

type MessageType string

const (
	MessageTypeRegister   MessageType = "register"
	MessageTypeTranscript MessageType = "transcript"
	MessageTypeSummary    MessageType = "summary"
	MessageTypeChat       MessageType = "chat"
	MessageTypeError      MessageType = "error"
)

type Message struct {
	Type      MessageType
	UserID    int64
	ChatID    int64
	Payload   string
	Data      []byte
	MeetingID int64
}

type Queue struct {
	workers  int
	msgChan  chan Message
	workerFn func(ctx context.Context, msg Message) error
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc

	startOnce sync.Once
	stopOnce  sync.Once

	stopped      atomic.Bool
	droppedCount atomic.Uint64
}

func New(ctx context.Context, workers int) *Queue {
	ctx, cancel := context.WithCancel(ctx)
	return &Queue{
		workers: workers,
		msgChan: make(chan Message, 100),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (q *Queue) Start(workerFn func(ctx context.Context, msg Message) error) {
	q.startOnce.Do(func() {
		q.workerFn = workerFn
		for i := 0; i < q.workers; i++ {
			q.wg.Add(1)
			go q.worker(q.ctx)
		}
	})
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	defer func() {
		if recover() != nil {
			// Не даем панике уронить весь пул воркеров.
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-q.msgChan:
			if !ok {
				return
			}
			if q.workerFn != nil {
				func() {
					defer func() {
						if recover() != nil {
							// Не даем панике в обработчике уронить воркер.
						}
					}()

					_ = q.workerFn(ctx, msg)
				}()
			}
		}
	}
}

func (q *Queue) Publish(msg Message) {
	if q.stopped.Load() {
		q.droppedCount.Add(1)
		return
	}

	select {
	case q.msgChan <- msg:
	default:
		q.droppedCount.Add(1)
	}
}

// SendToUser отправляет сообщение напрямую в общий обработчик очереди.
// Используется сервисом для обратной связи пользователю без публикации в канал.
func (q *Queue) SendToUser(chatID int64, msg Message) {
	if q.workerFn == nil {
		return
	}
	msg.ChatID = chatID
	_ = q.workerFn(q.ctx, msg)
}

func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		q.stopped.Store(true)
		q.cancel()
		close(q.msgChan)
		q.wg.Wait()
	})
}

func (q *Queue) DroppedMessages() uint64 {
	return q.droppedCount.Load()
}
