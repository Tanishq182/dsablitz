package battle

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"dsablitz/backend/internal/questions"
)

// Mock Questions Service
type mockQuestionsService struct {
	activeQuestions []questions.Question
	validationMatch bool
}

func (m *mockQuestionsService) GetSanitizedQuestion(ctx context.Context, id uuid.UUID) (questions.SanitizedQuestionResponse, error) {
	for _, q := range m.activeQuestions {
		if q.ID == id {
			return questions.SanitizeQuestion(q), nil
		}
	}
	return questions.SanitizedQuestionResponse{}, errors.New("not found")
}

func (m *mockQuestionsService) ValidateAnswer(ctx context.Context, questionID uuid.UUID, answer questions.SubmissionAnswer) (bool, error) {
	return m.validationMatch, nil
}

func (m *mockQuestionsService) GetActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]questions.Question, error) {
	return m.activeQuestions, nil
}

// Mock Battle Repository
type mockBattleRepository struct {
	battle     Battle
	players    map[uuid.UUID]BattlePlayer
	sequence   []uuid.UUID
	submission []struct {
		bID       uuid.UUID
		uID       uuid.UUID
		qID       uuid.UUID
		isCorrect bool
	}
}

func newMockBattleRepository() *mockBattleRepository {
	return &mockBattleRepository{
		players:  make(map[uuid.UUID]BattlePlayer),
		sequence: []uuid.UUID{},
	}
}

func (m *mockBattleRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func (m *mockBattleRepository) InsertBattle(ctx context.Context, tx pgx.Tx, b Battle) error {
	m.battle = b
	return nil
}

func (m *mockBattleRepository) InsertBattlePlayers(ctx context.Context, tx pgx.Tx, players []BattlePlayer) error {
	for _, p := range players {
		m.players[p.UserID] = p
	}
	return nil
}

func (m *mockBattleRepository) InsertBattleSequence(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, sequence []uuid.UUID) error {
	m.sequence = sequence
	return nil
}

func (m *mockBattleRepository) GetBattle(ctx context.Context, id uuid.UUID) (Battle, error) {
	if m.battle.ID == id {
		return m.battle, nil
	}
	return Battle{}, ErrNotFound
}

func (m *mockBattleRepository) GetBattlePlayerForUpdate(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (BattlePlayer, error) {
	p, ok := m.players[userID]
	if ok && p.BattleID == battleID {
		return p, nil
	}
	return BattlePlayer{}, ErrNotFound
}

func (m *mockBattleRepository) GetBattlePlayer(ctx context.Context, battleID, userID uuid.UUID) (BattlePlayer, error) {
	p, ok := m.players[userID]
	if ok && p.BattleID == battleID {
		return p, nil
	}
	return BattlePlayer{}, ErrNotFound
}

func (m *mockBattleRepository) UpdateBattlePlayer(ctx context.Context, tx pgx.Tx, p BattlePlayer) error {
	m.players[p.UserID] = p
	return nil
}

func (m *mockBattleRepository) InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, isCorrect bool, responseTimeMs int) error {
	m.submission = append(m.submission, struct {
		bID       uuid.UUID
		uID       uuid.UUID
		qID       uuid.UUID
		isCorrect bool
	}{bID, uID, qID, isCorrect})
	return nil
}

func (m *mockBattleRepository) GetQuestionIDAtSequenceIndex(ctx context.Context, battleID uuid.UUID, index int) (uuid.UUID, error) {
	if index < 0 || index >= len(m.sequence) {
		return uuid.Nil, errors.New("out of bounds")
	}
	return m.sequence[index], nil
}

func (m *mockBattleRepository) CompleteBattle(ctx context.Context, battleID uuid.UUID) error {
	if m.battle.ID == battleID {
		m.battle.Status = StatusCompleted
		return nil
	}
	return ErrNotFound
}

func TestBattleService_StartBattle(t *testing.T) {
	repo := newMockBattleRepository()
	q1 := uuid.New()
	q2 := uuid.New()

	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true},
			{ID: q2, IsActive: true},
		},
	}

	service := NewService(repo, qs)
	ctx := context.Background()

	roomID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()

	players := []BattlePlayer{
		{UserID: userID1, RatingBefore: 1200},
		{UserID: userID2, RatingBefore: 1300},
	}

	battleID, err := service.StartBattle(ctx, roomID, players, 42)
	if err != nil {
		t.Fatalf("failed to start battle: %v", err)
	}

	if battleID == uuid.Nil {
		t.Fatal("expected non-nil battle ID")
	}

	// Verify sequence size is exactly 200
	if len(repo.sequence) != MaxQuestionStreamSize {
		t.Errorf("expected sequence length %d, got %d", MaxQuestionStreamSize, len(repo.sequence))
	}

	// Verify players initialized
	if len(repo.players) != 2 {
		t.Errorf("expected 2 initialized players, got %d", len(repo.players))
	}

	player1 := repo.players[userID1]
	if player1.BattleID != battleID || player1.Score != 0 || player1.CurrentQuestionIndex != 0 {
		t.Errorf("player 1 initialized incorrectly: %+v", player1)
	}
}

func TestBattleService_SubmitAnswer_OptionCPolicy(t *testing.T) {
	repo := newMockBattleRepository()
	q1 := uuid.New()
	q2 := uuid.New()

	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true, QuestionType: questions.TypeMCQ},
			{ID: q2, IsActive: true, QuestionType: questions.TypeMCQ},
		},
	}

	service := NewService(repo, qs)
	ctx := context.Background()

	// Initializing mock database rows
	battleID := uuid.New()
	repo.battle = Battle{ID: battleID, Status: StatusActive}
	userID := uuid.New()
	repo.players[userID] = BattlePlayer{
		BattleID:                battleID,
		UserID:                  userID,
		CurrentQuestionIndex:    0,
		CurrentQuestionAttempts: 0,
		Score:                   0,
	}
	repo.sequence = make([]uuid.UUID, MaxQuestionStreamSize)
	repo.sequence[0] = q1
	repo.sequence[1] = q2

	// 1. Submit incorrect answer on attempt 1
	qs.validationMatch = false
	res, err := service.SubmitAnswer(ctx, battleID, userID, questions.SubmissionAnswer{TextAnswer: "Wrong"}, 500)
	if err != nil {
		t.Fatalf("unexpected submission error: %v", err)
	}
	if res.IsCorrect {
		t.Error("expected submission to be incorrect")
	}
	// Player remains on index 0, attempts incremented to 1
	p := repo.players[userID]
	if p.CurrentQuestionIndex != 0 || p.CurrentQuestionAttempts != 1 || p.Score != 0 {
		t.Errorf("expected index 0 and 1 attempt, got index %d, attempts %d, score %d", p.CurrentQuestionIndex, p.CurrentQuestionAttempts, p.Score)
	}

	// 2. Submit incorrect answer on attempt 2 (should skip)
	res, err = service.SubmitAnswer(ctx, battleID, userID, questions.SubmissionAnswer{TextAnswer: "WrongAgain"}, 500)
	if err != nil {
		t.Fatalf("unexpected submission error: %v", err)
	}
	if res.IsCorrect {
		t.Error("expected second submission to be incorrect")
	}
	// Player skips to index 1, attempts reset to 0, score remains 0
	p = repo.players[userID]
	if p.CurrentQuestionIndex != 1 || p.CurrentQuestionAttempts != 0 || p.Score != 0 {
		t.Errorf("expected skip to index 1 and 0 attempts, got index %d, attempts %d, score %d", p.CurrentQuestionIndex, p.CurrentQuestionAttempts, p.Score)
	}

	// 3. Submit correct answer on next question (index 1)
	qs.validationMatch = true
	res, err = service.SubmitAnswer(ctx, battleID, userID, questions.SubmissionAnswer{TextAnswer: "Right"}, 500)
	if err != nil {
		t.Fatalf("unexpected submission error: %v", err)
	}
	if !res.IsCorrect {
		t.Error("expected submission to be correct")
	}
	// Player advances to index 2, attempts reset to 0, score incremented to 1
	p = repo.players[userID]
	if p.CurrentQuestionIndex != 2 || p.CurrentQuestionAttempts != 0 || p.Score != 1 {
		t.Errorf("expected advance to index 2 and score 1, got index %d, attempts %d, score %d", p.CurrentQuestionIndex, p.CurrentQuestionAttempts, p.Score)
	}
}

func TestBattleService_SubmitAnswer_CompletedBattle(t *testing.T) {
	repo := newMockBattleRepository()
	qs := &mockQuestionsService{}

	service := NewService(repo, qs)
	ctx := context.Background()

	battleID := uuid.New()
	repo.battle = Battle{ID: battleID, Status: StatusCompleted}
	userID := uuid.New()

	_, err := service.SubmitAnswer(ctx, battleID, userID, questions.SubmissionAnswer{TextAnswer: "test"}, 100)
	if err == nil {
		t.Error("expected submission in completed battle to fail, got nil error")
	}
}

func TestBattleService_StartBattleTx(t *testing.T) {
	repo := newMockBattleRepository()
	q1 := uuid.New()
	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true},
		},
	}
	service := NewService(repo, qs)
	ctx := context.Background()

	roomID := uuid.New()
	userID1 := uuid.New()
	players := []BattlePlayer{
		{UserID: userID1, RatingBefore: 1200},
	}

	battleID, err := service.StartBattleTx(ctx, nil, roomID, players, 42)
	if err != nil {
		t.Fatalf("failed to start battle with tx: %v", err)
	}
	if battleID == uuid.Nil {
		t.Fatal("expected non-nil battle ID")
	}
	if repo.battle.ID != battleID {
		t.Errorf("expected battle ID %s in repo, got %s", battleID, repo.battle.ID)
	}
	if repo.battle.Status != StatusActive {
		t.Errorf("expected battle status to be %s, got %s", StatusActive, repo.battle.Status)
	}
}
