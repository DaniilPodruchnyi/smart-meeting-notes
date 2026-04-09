package queue

import (
	"context"
	"sync"
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
	subs     map[int64]map[MessageType]Handler
	workers  int
	msgChan  chan Message
	workerFn func(ctx context.Context, msg Message) error
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
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
	q.workerFn = workerFn
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(q.ctx)
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-q.msgChan:
			if q.workerFn != nil {
				if err := q.workerFn(ctx, msg); err != nil {
					q.sendToSubs(msg.ChatID, Message{Type: MessageTypeError, ChatID: msg.ChatID, Payload: err.Error()})
				}
			}
		}
	}
}

func (q *Queue) Subscribe(userID int64, msgType MessageType, h Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.subs[userID] == nil {
		q.subs[userID] = make(map[MessageType]Handler)
	}
	q.subs[userID][msgType] = h
}

func (q *Queue) Unsubscribe(userID int64, msgType MessageType) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.subs[userID] != nil {
		delete(q.subs[userID], msgType)
	}
}

func (q *Queue) Publish(msg Message) {
	select {
	case q.msgChan <- msg:
	default:
	}
}

func (q *Queue) sendToSubs(chatID int64, msg Message) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	handlers, ok := q.subs[chatID]
	if !ok {
		return
	}
	handler, ok := handlers[msg.Type]
	if ok {
		go handler(q.ctx, msg)
	}
}

func (q *Queue) SendToUser(chatID int64, msg Message) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	handlers, ok := q.subs[chatID]
	if !ok {
		return
	}
	handler, ok := handlers[msg.Type]
	if ok {
		handler(q.ctx, msg)
	}
}

func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()
	close(q.msgChan)
}
