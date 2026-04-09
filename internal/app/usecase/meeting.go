package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/app/repository"
	"smart-meeting-notes/internal/domain"
	"smart-meeting-notes/internal/queue"
)

// MeetingService сервис для управления встречами
type MeetingService struct {
	meetingRepo  repository.MeetingRepository
	userRepo     repository.UserRepository
	saluteClient SaluteSpeechClient
	gigaClient   GigaChatClient
	queue        *queue.Queue
	logger       *zap.Logger
	sendToUser   func(chatID int64, text string)
}

// SaluteSpeechClient интерфейс для транскрипции аудио
type SaluteSpeechClient interface {
	Transcribe(ctx context.Context, audio []byte, contentType string) (string, error)
}

// GigaChatClient интерфейс для работы с GigaChat
type GigaChatClient interface {
	Chat(ctx context.Context, prompt string) (string, error)
}

// NewMeetingService создает новый экземпляр сервиса встреч
func NewMeetingService(
	meetingRepo repository.MeetingRepository,
	userRepo repository.UserRepository,
	saluteClient SaluteSpeechClient,
	gigaClient GigaChatClient,
	q *queue.Queue,
	logger *zap.Logger,
	sendToUser func(chatID int64, text string),
) *MeetingService {
	return &MeetingService{
		meetingRepo:  meetingRepo,
		userRepo:     userRepo,
		saluteClient: saluteClient,
		gigaClient:   gigaClient,
		queue:        q,
		logger:       logger,
		sendToUser:   sendToUser,
	}
}

// RegisterUser регистрирует нового пользователя
func (s *MeetingService) RegisterUser(ctx context.Context, telegramID int64) error {
	if s.userRepo == nil {
		return nil
	}

	existing, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err == nil && existing != nil {
		return nil
	}

	user := &domain.User{TelegramID: telegramID}
	return s.userRepo.Create(ctx, user)
}

// ProcessMessage обрабатывает сообщение из очереди
func (s *MeetingService) ProcessMessage(ctx context.Context, msg queue.Message) error {
	s.logger.Debug("обработка сообщения", zap.String("type", string(msg.Type)), zap.Int64("chat_id", msg.ChatID))

	switch msg.Type {
	case queue.MessageTypeRegister:
		return s.handleRegister(ctx, msg)
	case queue.MessageTypeTranscript:
		return s.handleTranscriptCommand(ctx, msg)
	case queue.MessageTypeChat:
		return s.handleChatCommand(ctx, msg)
	default:
		s.queue.SendToUser(msg.ChatID, queue.Message{
			Type:    queue.MessageTypeError,
			ChatID:  msg.ChatID,
			Payload: "Неизвестная команда",
		})
	}
	return nil
}

// handleRegister обрабатывает команду регистрации
func (s *MeetingService) handleRegister(ctx context.Context, msg queue.Message) error {
	err := s.RegisterUser(ctx, msg.ChatID)
	if err != nil {
		s.queue.SendToUser(msg.ChatID, queue.Message{
			Type:    queue.MessageTypeError,
			ChatID:  msg.ChatID,
			Payload: "Ошибка регистрации: " + err.Error(),
		})
		return err
	}
	s.queue.SendToUser(msg.ChatID, queue.Message{
		Type:    queue.MessageTypeTranscript,
		ChatID:  msg.ChatID,
		Payload: "Вы успешно зарегистрированы!",
	})
	return nil
}

// handleTranscriptCommand обрабатывает команды связанные с транскриптами
func (s *MeetingService) handleTranscriptCommand(ctx context.Context, msg queue.Message) error {
	payload := msg.Payload

	if strings.HasPrefix(payload, "list") {
		return s.handleList(ctx, msg.ChatID)
	}
	if strings.HasPrefix(payload, "find ") {
		query := strings.TrimPrefix(payload, "find ")
		return s.handleFind(ctx, msg.ChatID, query)
	}
	if strings.HasPrefix(payload, "get ") {
		meetingID := strings.TrimPrefix(payload, "get ")
		return s.handleGet(ctx, msg.ChatID, meetingID)
	}

	return s.handleAudio(ctx, msg.ChatID, msg.Data)
}

// handleList обрабатывает команду /list
func (s *MeetingService) handleList(ctx context.Context, chatID int64) error {
	if s.userRepo == nil || s.meetingRepo == nil {
		s.sendError(chatID, "Репозиторий не инициализирован")
		return nil
	}

	user, err := s.userRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		s.sendError(chatID, "Вы не зарегистрированы. Используйте /start")
		return err
	}

	meetings, err := s.meetingRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		s.sendError(chatID, "Ошибка при получении списка: "+err.Error())
		return err
	}

	if len(meetings) == 0 {
		s.sendToUser(chatID, "У вас пока нет сохраненных встреч.")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Список встреч:\n\n")
	for _, m := range meetings {
		sb.WriteString(fmt.Sprintf("ID: %d | Дата: %s\n", m.ID, m.CreatedAt.Format("2006-01-02 15:04")))
	}

	s.sendToUser(chatID, sb.String())
	return nil
}

// handleGet обрабатывает команду /get
func (s *MeetingService) handleGet(ctx context.Context, chatID int64, meetingID string) error {
	s.logger.Info("handleGet called", zap.Int64("chat_id", chatID), zap.String("meeting_id", meetingID))

	if s.meetingRepo == nil {
		s.sendError(chatID, "Репозиторий встреч недоступен")
		return nil
	}

	id, err := strconv.ParseInt(meetingID, 10, 64)
	if err != nil {
		s.sendError(chatID, "Неверный ID встречи")
		return err
	}

	meeting, err := s.meetingRepo.GetByID(ctx, id)
	s.logger.Info("GetByID result", zap.Int64("id", id), zap.Any("meeting", meeting), zap.Error(err))
	if err != nil {
		s.sendError(chatID, "Встреча не найдена")
		return err
	}

	transcript := meeting.Transcript
	s.logger.Info("sending transcript", zap.Int64("chat_id", chatID), zap.Int("len", len(transcript)))

	if len([]rune(transcript)) > 4000 {
		runes := []rune(transcript)
		part1 := string(runes[:4000])
		part2 := string(runes[4000:])
		s.sendToUser(chatID, part1)
		s.sendToUser(chatID, part2)
	} else {
		s.sendToUser(chatID, transcript)
	}
	return nil
}

// handleFind обрабатывает команду /find
func (s *MeetingService) handleFind(ctx context.Context, chatID int64, query string) error {
	if s.userRepo == nil || s.meetingRepo == nil {
		s.sendError(chatID, "Репозиторий не инициализирован")
		return nil
	}

	user, err := s.userRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		s.sendError(chatID, "Вы не зарегистрированы")
		return err
	}

	meetings, err := s.meetingRepo.Search(ctx, user.ID, query)
	if err != nil {
		s.sendError(chatID, "Ошибка при поиске: "+err.Error())
		return err
	}

	if len(meetings) == 0 {
		s.sendToUser(chatID, "Встречи не найдены.")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Найдено встреч: ")
	sb.WriteString(strconv.Itoa(len(meetings)))
	sb.WriteString("\n\n")
	for _, m := range meetings {
		sb.WriteString(fmt.Sprintf("ID: %d | Дата: %s\n", m.ID, m.CreatedAt.Format("2006-01-02 15:04")))
	}

	s.sendToUser(chatID, sb.String())
	return nil
}

// sendError отправляет сообщение об ошибке пользователю
func (s *MeetingService) sendError(chatID int64, text string) {
	if s.sendToUser != nil {
		s.sendToUser(chatID, "Ошибка: "+text)
	} else {
		s.logger.Error("ошибка отправки", zap.String("text", text))
	}
}

// handleChatCommand обрабатывает команду /chat
func (s *MeetingService) handleChatCommand(ctx context.Context, msg queue.Message) error {
	text := msg.Payload

	if s.gigaClient == nil {
		s.sendToUser(msg.ChatID, "GigaChat недоступен.")
		return nil
	}

	s.logger.Info("handleChatCommand started", zap.Int64("chat_id", msg.ChatID), zap.String("text", text))

	var prompt string
	if s.meetingRepo != nil && s.userRepo != nil {
		user, err := s.userRepo.GetByTelegramID(ctx, msg.ChatID)
		s.logger.Info("user lookup", zap.Int64("chat_id", msg.ChatID), zap.Any("user", user), zap.Error(err))
		if err == nil && user != nil {
			meetings, err := s.meetingRepo.GetByUserID(ctx, user.ID)
			s.logger.Info("meetings lookup", zap.Int64("user_id", user.ID), zap.Int("count", len(meetings)), zap.Error(err))
			if err == nil && len(meetings) > 0 {
				latest := meetings[0]
				s.logger.Info("using meeting", zap.Int64("meeting_id", latest.ID), zap.Int("transcript_len", len(latest.Transcript)))
				prompt = "Ты - ассистент для анализа заметок со встречи. Вот транскрипция последней встречи:\n\n" + latest.Transcript + "\n\nВопрос пользователя: " + text
			}
		}
	}
	if prompt == "" {
		prompt = text
	}

	s.logger.Info("prompt prepared", zap.Int64("chat_id", msg.ChatID), zap.Int("prompt_len", len(prompt)))

	s.logger.Info("calling gigaChat.Chat")
	response, err := s.gigaClient.Chat(ctx, prompt)
	s.logger.Info("gigaChat.Chat returned", zap.Error(err))
	if err != nil {
		s.sendError(msg.ChatID, err.Error())
		return err
	}

	s.sendToUser(msg.ChatID, response)
	return nil
}

// handleAudio обрабатывает голосовое/аудио сообщение
func (s *MeetingService) handleAudio(ctx context.Context, chatID int64, audioData []byte) error {
	if s.userRepo == nil || s.meetingRepo == nil || s.saluteClient == nil {
		s.sendError(chatID, "Сервис транскрипции недоступен.")
		return nil
	}

	s.sendToUser(chatID, "Конвертирую аудио...")

	converted, contentType, audioEncoding, err := convertAudioToWav(audioData)
	if err != nil {
		s.sendError(chatID, "Ошибка конвертации: "+err.Error())
		return err
	}

	if len(converted) == 0 {
		s.sendError(chatID, "Конвертация вернула пустой файл.")
		return nil
	}

	s.sendToUser(chatID, "Начинаю транскрипцию...")

	user, err := s.userRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		s.sendError(chatID, "Вы не зарегистрированы. Используйте /start")
		return err
	}

	// Формируем contentType для SaluteSpeech API
	fullContentType := contentType
	if audioEncoding == "PCM_S16LE" {
		fullContentType = "audio/pcm;rate=16000"
	} else if audioEncoding != "" && strings.Contains(contentType, "pcm") {
		fullContentType = fmt.Sprintf("audio/pcm;rate=16000;codecs=%s", audioEncoding)
	} else if audioEncoding != "" {
		fullContentType = contentType + ";codecs=" + audioEncoding
	}

	transcript, err := s.saluteClient.Transcribe(ctx, converted, fullContentType)
	if err != nil {
		s.sendError(chatID, "Ошибка при транскрипции: "+err.Error())
		return err
	}

	if s.meetingRepo != nil {
		meeting := &domain.Meeting{
			UserID:     user.ID,
			Transcript: transcript,
		}
		if err := s.meetingRepo.Create(ctx, meeting); err != nil {
			s.logger.Error("ошибка сохранения встречи", zap.Error(err))
		}
	}

	transcriptText := transcript
	if len([]rune(transcriptText)) > 4000 {
		runes := []rune(transcriptText)
		part1 := string(runes[:4000])
		part2 := string(runes[4000:])
		s.sendToUser(chatID, "Транскрипция:\n\n"+part1)
		s.sendToUser(chatID, part2)
	} else {
		s.sendToUser(chatID, "Транскрипция:\n\n"+transcriptText)
	}

	return nil
}

// ParseCommand разбирает текст команды на имя и аргументы
func (s *MeetingService) ParseCommand(text string) (string, []string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	return strings.TrimPrefix(parts[0], "/"), parts[1:]
}
