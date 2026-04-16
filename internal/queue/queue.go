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

type Handler func(ctx context.Context, msg Message) error

type Queue struct {
	mu       sync.RWMutex
	subs     map[int64]map[MessageType]Handler // key: chatID
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
		subs:    make(map[int64]map[MessageType]Handler),
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
							q.sendToSubs(msg.ChatID, Message{
								Type:    MessageTypeError,
								ChatID:  msg.ChatID,
								Payload: "panic в обработчике очереди",
							})
						}
					}()

				if err := q.workerFn(ctx, msg); err != nil {
					q.sendToSubs(msg.ChatID, Message{Type: MessageTypeError, ChatID: msg.ChatID, Payload: err.Error()})
				}
				}()
			}
		}
	}
}

func (q *Queue) Subscribe(chatID int64, msgType MessageType, h Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.subs[chatID] == nil {
		q.subs[chatID] = make(map[MessageType]Handler)
	}
	q.subs[chatID][msgType] = h
}

func (q *Queue) Unsubscribe(chatID int64, msgType MessageType) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.subs[chatID] != nil {
		delete(q.subs[chatID], msgType)
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

func (q *Queue) sendToSubs(chatID int64, msg Message) {
	q.mu.RLock()
	handlers, ok := q.subs[chatID]
	if !ok {
		q.mu.RUnlock()
		return
	}
	handler, ok := handlers[msg.Type]
	q.mu.RUnlock()

	if ok {
		_ = handler(q.ctx, msg)
	}
}

func (q *Queue) SendToUser(chatID int64, msg Message) {
	q.mu.RLock()
	handlers, ok := q.subs[chatID]
	if !ok {
		q.mu.RUnlock()
		return
	}
	handler, ok := handlers[msg.Type]
	q.mu.RUnlock()

	if ok {
		_ = handler(q.ctx, msg)
	}
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
