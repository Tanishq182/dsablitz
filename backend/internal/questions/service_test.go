package questions

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type mockRepository struct {
	questions []Question
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		questions: []Question{},
	}
}

func (m *mockRepository) FindQuestionByID(ctx context.Context, id uuid.UUID) (Question, error) {
	for _, q := range m.questions {
		if q.ID == id {
			return q, nil
		}
	}
	return Question{}, ErrNotFound
}

func (m *mockRepository) FindActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]Question, error) {
	var result []Question
	for _, q := range m.questions {
		if !q.IsActive {
			continue
		}
		if difficulty > 0 && q.Difficulty != difficulty {
			continue
		}
		if len(tags) > 0 {
			match := false
			for _, t := range tags {
				for _, qt := range q.Tags {
					if qt == t {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, q)
	}
	return result, nil
}

func (m *mockRepository) InsertOrUpdateQuestion(ctx context.Context, q Question) error {
	for i, existing := range m.questions {
		if existing.ID == q.ID {
			m.questions[i] = q
			return nil
		}
	}
	m.questions = append(m.questions, q)
	return nil
}

func TestService_CacheAndLookup(t *testing.T) {
	repo := newMockRepository()
	id1 := uuid.New()
	id2 := uuid.New()

	repo.questions = []Question{
		{ID: id1, Title: "Q1", Difficulty: 1, IsActive: true},
		{ID: id2, Title: "Q2", Difficulty: 2, IsActive: true},
	}

	service := NewService(repo)
	ctx := context.Background()

	// 1. Initial lookup should fallback to DB because cache is empty
	q, err := service.GetQuestionByID(ctx, id1)
	if err != nil {
		t.Fatalf("failed to fetch question: %v", err)
	}
	if q.Title != "Q1" {
		t.Errorf("expected Q1, got %s", q.Title)
	}

	// 2. Load cache
	err = service.LoadCache(ctx)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}

	// 3. Clear database list to prove we read from cache
	repo.questions = nil

	// Lookup should succeed from in-memory cache
	q, err = service.GetQuestionByID(ctx, id1)
	if err != nil {
		t.Fatalf("failed to fetch cached question: %v", err)
	}
	if q.Title != "Q1" {
		t.Errorf("expected Q1, got %s", q.Title)
	}

	// 4. Test sanitization DTO mapping
	sanitized, err := service.GetSanitizedQuestion(ctx, id2)
	if err != nil {
		t.Fatalf("failed to fetch sanitized question: %v", err)
	}
	if sanitized.Title != "Q2" {
		t.Errorf("expected Q2, got %s", sanitized.Title)
	}
}

func TestService_ValidateAnswer(t *testing.T) {
	repo := newMockRepository()
	id1 := uuid.New()

	repo.questions = []Question{
		{
			ID:            id1,
			QuestionType:  TypeMCQ,
			CorrectAnswer: "O(N)",
			IsActive:      true,
		},
	}

	service := NewService(repo)
	ctx := context.Background()

	// Load Cache
	_ = service.LoadCache(ctx)

	// Test validation check
	isCorrect, err := service.ValidateAnswer(ctx, id1, SubmissionAnswer{TextAnswer: "O(N)"})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if !isCorrect {
		t.Error("expected validation to succeed")
	}

	isCorrect, err = service.ValidateAnswer(ctx, id1, SubmissionAnswer{TextAnswer: "O(1)"})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if isCorrect {
		t.Error("expected validation to fail")
	}

	// Miss case
	_, err = service.ValidateAnswer(ctx, uuid.New(), SubmissionAnswer{TextAnswer: "O(N)"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
