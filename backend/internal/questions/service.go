package questions

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// QuestionsRepository defines the database interactions for the questions module.
type QuestionsRepository interface {
	FindQuestionByID(ctx context.Context, id uuid.UUID) (Question, error)
	FindActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]Question, error)
	InsertOrUpdateQuestion(ctx context.Context, q Question) error
}

type Service struct {
	repo  QuestionsRepository
	mu    sync.RWMutex
	cache map[uuid.UUID]Question
}

func NewService(repo QuestionsRepository) *Service {
	return &Service{
		repo:  repo,
		cache: make(map[uuid.UUID]Question),
	}
}

// LoadCache populates the in-memory question bank from the database.
func (s *Service) LoadCache(ctx context.Context) error {
	qs, err := s.repo.FindActiveQuestionsByFilters(ctx, 0, nil)
	if err != nil {
		return fmt.Errorf("load active questions cache: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = make(map[uuid.UUID]Question)
	for _, q := range qs {
		s.cache[q.ID] = q
	}
	return nil
}

// GetQuestionByID retrieves a question from the in-memory cache, falling back to database query if missed.
func (s *Service) GetQuestionByID(ctx context.Context, id uuid.UUID) (Question, error) {
	s.mu.RLock()
	q, ok := s.cache[id]
	s.mu.RUnlock()
	if ok {
		return q, nil
	}
	return s.repo.FindQuestionByID(ctx, id)
}

// GetSanitizedQuestion retrieves a question by ID and filters sensitive keys for client delivery.
func (s *Service) GetSanitizedQuestion(ctx context.Context, id uuid.UUID) (SanitizedQuestionResponse, error) {
	q, err := s.GetQuestionByID(ctx, id)
	if err != nil {
		return SanitizedQuestionResponse{}, err
	}
	return SanitizeQuestion(q), nil
}

// ValidateAnswer evaluates the correctness of a user submission without database I/O overhead.
func (s *Service) ValidateAnswer(ctx context.Context, questionID uuid.UUID, answer SubmissionAnswer) (bool, error) {
	q, err := s.GetQuestionByID(ctx, questionID)
	if err != nil {
		return false, fmt.Errorf("find validation target: %w", err)
	}
	return ValidateAnswer(q.QuestionType, q.CorrectAnswer, answer)
}

// GetActiveQuestionsByFilters returns active questions from the database matching the criteria.
func (s *Service) GetActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]Question, error) {
	return s.repo.FindActiveQuestionsByFilters(ctx, difficulty, tags)
}

// SanitizeQuestion converts a full Question entity into a client-safe DTO.
func SanitizeQuestion(q Question) SanitizedQuestionResponse {
	return SanitizedQuestionResponse{
		ID:           q.ID,
		QuestionType: q.QuestionType,
		Difficulty:   q.Difficulty,
		Title:        q.Title,
		Prompt:       q.Prompt,
		Options:      q.Options,
		TimeLimitSec: q.TimeLimitSec,
		Tags:         q.Tags,
	}
}
