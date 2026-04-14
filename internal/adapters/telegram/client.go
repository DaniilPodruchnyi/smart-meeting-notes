package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/telebot.v3"

	"smart-meeting-notes/internal/app/usecase"
	"smart-meeting-notes/internal/config"
	"smart-meeting-notes/internal/pkg/httptls"
	"smart-meeting-notes/internal/queue"
)

// Client клиент Telegram бота
type Client struct {
	bot            *telebot.Bot
	logger         *zap.Logger
	cfg            config.TelegramConfig
	q              *queue.Queue
	meetingService *usecase.MeetingService
	wg             sync.WaitGroup
	ctx            context.Context
	httpClient     *http.Client
}

// New создает новый экземпляр Telegram клиента
func New(cfg config.TelegramConfig, tlsCfg config.TLSConfig, lg *zap.Logger, q *queue.Queue, meetingSvc *usecase.MeetingService) (*Client, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("telegram: токен бота пуст")
	}

	poller := &telebot.LongPoller{Timeout: 60}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.BotToken,
		Poller: poller,
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: создание бота: %w", err)
	}

	httpClient := &http.Client{Transport: httptls.NewTransport(tlsCfg.InsecureSkipVerify)}

	return &Client{
		bot:            bot,
		logger:         lg,
		cfg:            cfg,
		q:              q,
		meetingService: meetingSvc,
		httpClient:     httpClient,
	}, nil
}

// Run запускает бота и начинает обработку обновлений
func (c *Client) Run(ctx context.Context) error {
	c.ctx = ctx
	c.bot.Use(c.middlewareLogger)

	// Регистрация обработчиков команд
	c.bot.Handle("/start", c.handleStart)
	c.bot.Handle("/list", c.handleList)
	c.bot.Handle("/get", c.handleGet)
	c.bot.Handle("/find", c.handleFind)
	c.bot.Handle("/chat", c.handleChat)

	// Обработчики сообщений
	c.bot.Handle(telebot.OnText, c.handleText)
	c.bot.Handle(telebot.OnVoice, c.handleVoice)
	c.bot.Handle(telebot.OnAudio, c.handleAudio)

	// Запуск обработчика очереди
	c.q.Start(c.processQueueMessage)

	c.logger.Info("telegram бот запущен")
	c.bot.Start()
	return nil
}

// middlewareLogger логирует входящие обновления
func (c *Client) middlewareLogger(h telebot.HandlerFunc) telebot.HandlerFunc {
	return func(ctx telebot.Context) error {
		c.logger.Debug("telegram update", zap.Int64("chat_id", ctx.Chat().ID))
		return h(ctx)
	}
}

// handleStart обрабатывает команду /start
func (c *Client) handleStart(ctx telebot.Context) error {
	c.logger.Info("пользователь начал работу", zap.Int64("chat_id", ctx.Chat().ID))

	regMsg := queue.Message{Type: queue.MessageTypeRegister, ChatID: ctx.Chat().ID, Payload: ""}
	c.q.Publish(regMsg)

	return ctx.Send("Привет! Я бот для конспектирования встреч.\n\n" +
		"Доступные команды:\n" +
		"/start - начать работу\n" +
		"/list - список встреч\n" +
		"/get <id> - получить текст встречи\n" +
		"/find <слово> - найти встречу\n" +
		"/chat <вопрос> - задать вопрос ассистенту")
}

// handleList обрабатывает команду /list
func (c *Client) handleList(ctx telebot.Context) error {
	msg := queue.Message{Type: queue.MessageTypeTranscript, ChatID: ctx.Chat().ID, Payload: "list"}
	c.q.Publish(msg)
	return ctx.Send("Загружаю список встреч...")
}

// handleGet обрабатывает команду /get
func (c *Client) handleGet(ctx telebot.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return ctx.Send("Использование: /get <id>")
	}
	msg := queue.Message{Type: queue.MessageTypeTranscript, ChatID: ctx.Chat().ID, Payload: "get " + args[0]}
	c.q.Publish(msg)
	return ctx.Send("Загружаю встречу...")
}

// handleFind обрабатывает команду /find
func (c *Client) handleFind(ctx telebot.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return ctx.Send("Использование: /find <слово>")
	}
	msg := queue.Message{Type: queue.MessageTypeTranscript, ChatID: ctx.Chat().ID, Payload: "find " + strings.Join(args, " ")}
	c.q.Publish(msg)
	return ctx.Send("Ищу встречи...")
}

// handleChat обрабатывает команду /chat
func (c *Client) handleChat(ctx telebot.Context) error {
	args := ctx.Args()
	if len(args) < 1 {
		return ctx.Send("Использование: /chat <вопрос>")
	}
	msg := queue.Message{Type: queue.MessageTypeChat, ChatID: ctx.Chat().ID, Payload: strings.Join(args, " ")}
	c.q.Publish(msg)
	return ctx.Send("Думаю над ответом...")
}

// handleText обрабатывает текстовые сообщения
func (c *Client) handleText(ctx telebot.Context) error {
	return c.handleChat(ctx)
}

// handleVoice обрабатывает голосовые сообщения
func (c *Client) handleVoice(ctx telebot.Context) error {
	c.logger.Info("получено голосовое сообщение", zap.Int64("chat_id", ctx.Chat().ID))
	msg := ctx.Message()
	if msg == nil || msg.Voice == nil {
		return ctx.Send("Не удалось получить файл")
	}

	file, err := c.bot.FileByID(msg.Voice.FileID)
	if err != nil {
		c.logger.Error("получение файла", zap.Error(err))
		return ctx.Send("Не удалось получить файл")
	}

	fileURL := file.FileURL
	if fileURL == "" {
		fileURL = "https://api.telegram.org/file/bot" + c.cfg.BotToken + "/" + file.FilePath
	}

	audioData, err := c.downloadFile(c.ctx, fileURL)
	if err != nil {
		c.logger.Error("скачивание файла", zap.Error(err))
		return ctx.Send("Не удалось скачать файл")
	}

	queueMsg := queue.Message{Type: queue.MessageTypeTranscript, ChatID: ctx.Chat().ID, Data: audioData}
	c.q.Publish(queueMsg)
	return ctx.Send("Аудио получено, обрабатываю...")
}

// handleAudio обрабатывает аудио файлы
func (c *Client) handleAudio(ctx telebot.Context) error {
	c.logger.Info("получено аудио сообщение", zap.Int64("chat_id", ctx.Chat().ID))
	msg := ctx.Message()
	if msg == nil || msg.Audio == nil {
		return ctx.Send("Не удалось получить файл")
	}

	file, err := c.bot.FileByID(msg.Audio.FileID)
	if err != nil {
		c.logger.Error("получение файла", zap.Error(err))
		return ctx.Send("Не удалось получить файл")
	}

	fileURL := file.FileURL
	if fileURL == "" {
		fileURL = "https://api.telegram.org/file/bot" + c.cfg.BotToken + "/" + file.FilePath
	}

	audioData, err := c.downloadFile(c.ctx, fileURL)
	if err != nil {
		c.logger.Error("скачивание файла", zap.Error(err))
		return ctx.Send("Не удалось скачать файл")
	}

	queueMsg := queue.Message{Type: queue.MessageTypeTranscript, ChatID: ctx.Chat().ID, Data: audioData}
	c.q.Publish(queueMsg)
	return ctx.Send("Аудио получено, обрабатываю...")
}

// downloadFile скачивает файл по URL
func (c *Client) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("неверный статус: %s", res.Status)
	}

	return io.ReadAll(res.Body)
}

// processQueueMessage обрабатывает сообщения из очереди
func (c *Client) processQueueMessage(ctx context.Context, msg queue.Message) error {
	c.logger.Debug("обработка сообщения из очереди", zap.String("type", string(msg.Type)), zap.Int64("chat_id", msg.ChatID))

	if c.meetingService != nil {
		err := c.meetingService.ProcessMessage(ctx, msg)
		if err != nil {
			c.logger.Error("ошибка обработки сообщения", zap.Error(err))
		}
		return err
	}
	return nil
}

// Stop останавливает бота
func (c *Client) Stop() {
	c.bot.Stop()
}

// SendToUser отправляет сообщение пользователю
func (c *Client) SendToUser(chatID int64, text string) error {
	_, err := c.bot.Send(&telebot.Chat{ID: chatID}, text)
	return err
}

// SetMeetingService устанавливает сервис встреч
func (c *Client) SetMeetingService(svc interface{}) {
	if ms, ok := svc.(*usecase.MeetingService); ok {
		c.meetingService = ms
	}
}
