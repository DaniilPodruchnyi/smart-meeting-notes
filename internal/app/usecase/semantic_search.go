package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"smart-meeting-notes/internal/domain"
)

// SemanticSearchService оркестрирует эмбеддинг запроса и поиск в SemanticMeetingStore.
type SemanticSearchService struct {
	embedder    Embedder
	store       SemanticMeetingStore
	queryPrefix string // префикс для retrieval-запросов (GIGACHAT_EMBEDDINGS_QUERY_PREFIX)
}

func NewSemanticSearchService(embedder Embedder, store SemanticMeetingStore, embeddingsQueryPrefix string) *SemanticSearchService {
	return &SemanticSearchService{
		embedder:    embedder,
		store:       store,
		queryPrefix: embeddingsQueryPrefix,
	}
}

// Search готовый вызов для «умного» /find: строка запроса → вектор → top встреч пользователя.
func (s *SemanticSearchService) Search(ctx context.Context, telegramUserID int64, query string, limit int) ([]domain.MeetingSemanticHit, error) {
	if s.embedder == nil || s.store == nil {
		return nil, errors.New("usecase: semantic search dependencies not set")
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("usecase: empty search query")
	}
	if limit <= 0 {
		limit = 10
	}
	if s.queryPrefix != "" {
		q = s.queryPrefix + q
	}

	vecs, err := s.embedder.Embed(ctx, []string{q})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || vecs[0] == nil {
		return nil, errors.New("usecase: empty query embedding")
	}
	return s.store.SearchSemantic(ctx, telegramUserID, vecs[0], limit)
}

// IndexMeetingTranscript считает эмбеддинг полного текста транскрипта (без query-prefix) и сохраняет в store.
func (s *SemanticSearchService) IndexMeetingTranscript(ctx context.Context, meetingID string, telegramUserID int64, transcript string) error {
	if s.embedder == nil || s.store == nil {
		return errors.New("usecase: semantic search dependencies not set")
	}
	t := strings.TrimSpace(transcript)
	if t == "" {
		return errors.New("usecase: empty transcript")
	}
	if strings.TrimSpace(meetingID) == "" {
		return errors.New("usecase: empty meeting id")
	}

	vecs, err := s.embedder.Embed(ctx, []string{t})
	if err != nil {
		return err
	}
	if len(vecs) == 0 || vecs[0] == nil {
		return errors.New("usecase: empty transcript embedding")
	}

	sum := sha256.Sum256([]byte(t))
	hash := hex.EncodeToString(sum[:])
	return s.store.UpsertMeetingEmbedding(ctx, meetingID, telegramUserID, vecs[0], hash)
}
