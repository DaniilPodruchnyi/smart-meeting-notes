package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/adapters/salutespeech"
	"smart-meeting-notes/internal/app/repository"
	"smart-meeting-notes/internal/domain"
	"smart-meeting-notes/internal/queue"
)

const (
	smartFindMinScore = 0.35
	smartFindTopK     = 5
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
	TranscribeDetailed(ctx context.Context, audio []byte, contentType string) (salutespeech.TranscriptResult, error)
}

// GigaChatClient интерфейс для работы с GigaChat
type GigaChatClient interface {
	Chat(ctx context.Context, prompt string) (string, error)
	Embedding(ctx context.Context, text string) ([]float64, error)
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
	if strings.HasPrefix(payload, "smart-find ") {
		query := strings.TrimPrefix(payload, "smart-find ")
		return s.handleSmartFind(ctx, msg.ChatID, query)
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

func (s *MeetingService) handleSmartFind(ctx context.Context, chatID int64, query string) error {
	if s.userRepo == nil || s.meetingRepo == nil {
		s.sendError(chatID, "Репозиторий не инициализирован")
		return nil
	}
	if s.gigaClient == nil {
		s.sendError(chatID, "Семантический поиск недоступен")
		return nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		s.sendError(chatID, "Пустой запрос")
		return nil
	}

	user, err := s.userRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		s.sendError(chatID, "Вы не зарегистрированы")
		return err
	}

	queryEmb, err := s.gigaClient.Embedding(ctx, query)
	if err != nil {
		s.sendError(chatID, "Не удалось построить embedding запроса: "+err.Error())
		return err
	}

	meetings, err := s.meetingRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		s.sendError(chatID, "Ошибка получения встреч: "+err.Error())
		return err
	}
	if len(meetings) == 0 {
		s.sendToUser(chatID, "Встречи не найдены.")
		return nil
	}

	type scored struct {
		meeting         domain.Meeting
		score           float64
		summaryScore    float64
		transcriptScore float64
		reason          string
		reasonSnippet   string
	}
	scoredMeetings := make([]scored, 0, len(meetings))
	meetingsWithVectors := 0
	for _, m := range meetings {
		if len(m.SummaryEmb) > 0 || len(m.TranscriptEmb) > 0 {
			meetingsWithVectors++
		}
		summaryScore := cosineSimilarity(queryEmb, m.SummaryEmb)
		transcriptScore := cosineSimilarity(queryEmb, m.TranscriptEmb)
		score := max(summaryScore, transcriptScore)
		if score < smartFindMinScore {
			continue
		}
		reason, snippet := buildSmartFindReason(m, summaryScore, transcriptScore)
		scoredMeetings = append(scoredMeetings, scored{
			meeting:         m,
			score:           score,
			summaryScore:    summaryScore,
			transcriptScore: transcriptScore,
			reason:          reason,
			reasonSnippet:   snippet,
		})
	}
	if len(scoredMeetings) == 0 {
		if meetingsWithVectors == 0 {
			s.sendToUser(chatID, "smart-find пока недоступен: у встреч нет embeddings. Проверьте доступ к GigaChat Embeddings API (в логах сейчас может быть 402 Payment Required).")
			return nil
		}
		s.sendToUser(chatID, fmt.Sprintf("По smart-find ничего релевантного не найдено (порог %.2f).", smartFindMinScore))
		return nil
	}

	sort.Slice(scoredMeetings, func(i, j int) bool { return scoredMeetings[i].score > scoredMeetings[j].score })
	limit := smartFindTopK
	if len(scoredMeetings) < limit {
		limit = len(scoredMeetings)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("smart-find: найдено %d релевантных встреч (порог %.2f)\n\n", len(scoredMeetings), smartFindMinScore))
	for i := 0; i < limit; i++ {
		item := scoredMeetings[i]
		m := item.meeting
		sb.WriteString(fmt.Sprintf("- ID %d | релевантность %.3f | %s\n", m.ID, item.score, m.CreatedAt.Format("2006-01-02 15:04")))
		sb.WriteString(fmt.Sprintf("  Причина: %s\n", item.reason))
		if item.reasonSnippet != "" {
			sb.WriteString("  Фрагмент: " + item.reasonSnippet + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Для полной транскрипции: /get <id>")
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
	if strings.TrimSpace(transcript) == "" {
		s.sendToUser(chatID, "Транскрипция для этой встречи пока пустая.")
		return nil
	}

	s.sendLongMessage(chatID, "Полная транскрипция:\n\n"+transcript)
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

	tr, err := s.saluteClient.TranscribeDetailed(ctx, converted, fullContentType)
	if err != nil {
		s.sendError(chatID, "Ошибка при транскрипции: "+err.Error())
		return err
	}
	rawTranscript := strings.TrimSpace(tr.RawText)
	roleTranscript := strings.TrimSpace(tr.TextBySpeakers)
	s.logger.Info(
		"transcription received",
		zap.Int64("chat_id", chatID),
		zap.Int("raw_len", len(rawTranscript)),
		zap.Int("roles_len", len(roleTranscript)),
		zap.Int("roles_speakers", speakerCount(roleTranscript)),
	)
	if rawTranscript == "" {
		s.sendToUser(chatID, "Не удалось распознать речь. Похоже, сообщение пустое или слишком тихое.")
		return nil
	}
	if roleTranscript == "" {
		roleTranscript = rawTranscript
	}

	// Если SaluteSpeech не разделил спикеров, пробуем сделать читаемую ролевую версию через LLM.
	needRoleFallback := !hasAtLeastTwoSpeakers(roleTranscript) || roleTranscript == rawTranscript
	if needRoleFallback && s.gigaClient != nil {
		s.logger.Info(
			"role transcript fallback started",
			zap.Int64("chat_id", chatID),
			zap.Bool("equal_to_raw", roleTranscript == rawTranscript),
			zap.Int("speaker_count_before", speakerCount(roleTranscript)),
		)
		roleTranscript, err = s.buildRoleTranscript(ctx, rawTranscript)
		if err != nil {
			s.logger.Warn("не удалось построить ролевую транскрипцию", zap.Error(err))
			roleTranscript = rawTranscript
		} else {
			s.logger.Info(
				"role transcript fallback finished",
				zap.Int64("chat_id", chatID),
				zap.Int("roles_len_after", len(roleTranscript)),
				zap.Int("speaker_count_after", speakerCount(roleTranscript)),
			)
		}
	}

	summary := ""
	summaryEmb := []float64(nil)
	transcriptEmb := []float64(nil)

	if s.gigaClient != nil {
		transcriptEmb, err = s.gigaClient.Embedding(ctx, rawTranscript)
		if err != nil {
			s.logger.Warn("не удалось получить embedding транскрипции", zap.Error(err))
		}
	}

	if s.gigaClient != nil {
		summary, err = s.buildSummary(ctx, roleTranscript)
		if err != nil {
			s.logger.Warn("не удалось получить summary", zap.Error(err))
		} else {
			s.logger.Info(
				"summary generated",
				zap.Int64("chat_id", chatID),
				zap.Int("summary_len_before_clean", len(summary)),
			)
			summaryEmb, err = s.gigaClient.Embedding(ctx, summary)
			if err != nil {
				s.logger.Warn("не удалось получить embedding summary", zap.Error(err))
			}
		}
	}

	if s.meetingRepo != nil {
		meeting := &domain.Meeting{
			UserID:        user.ID,
			TranscriptRaw: rawTranscript,
			Transcript:    roleTranscript,
			Summary:       summary,
			TranscriptEmb: transcriptEmb,
			SummaryEmb:    summaryEmb,
		}
		if err := s.meetingRepo.Create(ctx, meeting); err != nil {
			s.logger.Error("ошибка сохранения встречи", zap.Error(err))
		}

		if summary != "" {
			summary = cleanTelegramText(summary)
			s.logger.Info(
				"summary prepared for telegram",
				zap.Int64("chat_id", chatID),
				zap.Int("summary_len_after_clean", len(summary)),
			)
			s.sendLongMessage(chatID, "Краткая выжимка:\n\n"+summary+"\n\nПолная транскрипция доступна по команде /get "+strconv.FormatInt(meeting.ID, 10))
			s.logger.Info(
				"meeting stored with summary",
				zap.Int64("chat_id", chatID),
				zap.Int64("meeting_id", meeting.ID),
				zap.Int("raw_len", len(rawTranscript)),
				zap.Int("roles_len", len(roleTranscript)),
			)
			return nil
		}
	}

	// Если summary недоступно, отправляем транскрипцию как fallback.
	s.sendLongMessage(chatID, "Транскрипция:\n\n"+roleTranscript)

	return nil
}

func (s *MeetingService) buildSummary(ctx context.Context, transcript string) (string, error) {
	prompt := "Ты аналитик встреч. Ниже транскрипция с возможными метками спикеров (например, \"Спикер 1\", \"Спикер 2\"). " +
		"Сделай краткую и содержательную выжимку на русском языке.\n\n" +
		"Требования к формату:\n" +
		"1) Верни структурированный текст для Telegram.\n" +
		"2) Используй блоки в таком виде:\n" +
		"Ключевые моменты:\n- пункт\n- пункт\n\n" +
		"Решения:\n- пункт\n\n" +
		"Договоренности и следующие шаги:\n- пункт\n\n" +
		"3) Внутри блоков используй списки только через '-' (дефис).\n" +
		"4) Разрешены простые визуальные разделители типа '-----'.\n" +
		"5) Если данных мало, явно укажи это отдельным пунктом.\n\n" +
		"Транскрипция:\n" + transcript

	summary, err := s.gigaClient.Chat(ctx, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(summary), nil
}

func (s *MeetingService) sendLongMessage(chatID int64, text string) {
	runes := []rune(text)
	for len(runes) > 0 {
		chunkSize := 4000
		if len(runes) < chunkSize {
			chunkSize = len(runes)
		}
		s.sendToUser(chatID, string(runes[:chunkSize]))
		runes = runes[chunkSize:]
	}
}

func (s *MeetingService) buildRoleTranscript(ctx context.Context, rawTranscript string) (string, error) {
	prompt := "Ниже сырая транскрипция разговора. Преобразуй ее в диалог по ролям.\n" +
		"Правила:\n" +
		"1) Верни только текст диалога без markdown.\n" +
		"2) Используй формат строк: \"Спикер 1: ...\", \"Спикер 2: ...\".\n" +
		"3) Сохраняй смысл и порядок реплик, не выдумывай факты.\n" +
		"4) Если уверенности в разделении мало, сделай минимально вероятное разбиение.\n\n" +
		"Сырая транскрипция:\n" + rawTranscript

	roleTranscript, err := s.gigaClient.Chat(ctx, prompt)
	if err != nil {
		return "", err
	}
	roleTranscript = cleanTelegramText(roleTranscript)
	return strings.TrimSpace(roleTranscript), nil
}

func hasAtLeastTwoSpeakers(transcript string) bool {
	lines := strings.Split(transcript, "\n")
	speakers := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		prefix := strings.TrimSpace(line[:colon])
		if strings.HasPrefix(prefix, "Спикер") {
			speakers[prefix] = struct{}{}
		}
	}
	return len(speakers) >= 2
}

func speakerCount(transcript string) int {
	lines := strings.Split(transcript, "\n")
	speakers := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		prefix := strings.TrimSpace(line[:colon])
		if strings.HasPrefix(prefix, "Спикер") {
			speakers[prefix] = struct{}{}
		}
	}
	return len(speakers)
}

func cleanTelegramText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "### ", "")
	text = strings.ReplaceAll(text, "## ", "")
	text = strings.ReplaceAll(text, "# ", "")
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "•", "-")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
				continue
			}
			cleaned = append(cleaned, "")
			continue
		}
		line = strings.TrimPrefix(line, "* ")
		if strings.HasPrefix(line, "-") {
			line = "- " + strings.TrimSpace(strings.TrimPrefix(line, "-"))
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	var normA float64
	var normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func buildSmartFindReason(meeting domain.Meeting, summaryScore, transcriptScore float64) (string, string) {
	if summaryScore >= transcriptScore && strings.TrimSpace(meeting.Summary) != "" {
		return fmt.Sprintf("запрос ближе к выжимке встречи (score %.3f)", summaryScore), oneLineSnippet(meeting.Summary, 140)
	}
	if strings.TrimSpace(meeting.TranscriptRaw) != "" {
		return fmt.Sprintf("запрос ближе к полной транскрипции (score %.3f)", transcriptScore), oneLineSnippet(meeting.TranscriptRaw, 140)
	}
	return fmt.Sprintf("запрос ближе к полной транскрипции (score %.3f)", transcriptScore), oneLineSnippet(meeting.Transcript, 140)
}

func oneLineSnippet(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

// ParseCommand разбирает текст команды на имя и аргументы
func (s *MeetingService) ParseCommand(text string) (string, []string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	return strings.TrimPrefix(parts[0], "/"), parts[1:]
}
