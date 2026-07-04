package battle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"dsablitz/backend/internal/questions"
)

// Mock Clock
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

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

type userIDKey struct {
	battleID   uuid.UUID
	userID     uuid.UUID
	questionID uuid.UUID
}

type userIDKey2 struct {
	battleID uuid.UUID
	userID   uuid.UUID
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
	submissionsForQuestion map[userIDKey][]questions.SubmissionAnswer
	lastSubmissionTime     map[userIDKey2]time.Time
	clock                  Clock
}

func newMockBattleRepository() *mockBattleRepository {
	return &mockBattleRepository{
		players:                make(map[uuid.UUID]BattlePlayer),
		sequence:               []uuid.UUID{},
		submissionsForQuestion: make(map[userIDKey][]questions.SubmissionAnswer),
		lastSubmissionTime:     make(map[userIDKey2]time.Time),
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

func (m *mockBattleRepository) GetBattleTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Battle, error) {
	return m.GetBattle(ctx, id)
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

func (m *mockBattleRepository) InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, answer questions.SubmissionAnswer, isCorrect bool, scoreAwarded int, responseTimeMs int) error {
	m.submission = append(m.submission, struct {
		bID       uuid.UUID
		uID       uuid.UUID
		qID       uuid.UUID
		isCorrect bool
	}{bID, uID, qID, isCorrect})

	key := userIDKey{battleID: bID, userID: uID, questionID: qID}
	m.submissionsForQuestion[key] = append(m.submissionsForQuestion[key], answer)

	key2 := userIDKey2{battleID: bID, userID: uID}
	if m.clock != nil {
		m.lastSubmissionTime[key2] = m.clock.Now()
	} else {
		m.lastSubmissionTime[key2] = time.Now()
	}
	return nil
}

func (m *mockBattleRepository) GetSubmissionsForQuestion(ctx context.Context, tx pgx.Tx, battleID, userID, questionID uuid.UUID) ([]questions.SubmissionAnswer, error) {
	key := userIDKey{battleID: battleID, userID: userID, questionID: questionID}
	return m.submissionsForQuestion[key], nil
}

func (m *mockBattleRepository) GetLastSubmission(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (time.Time, error) {
	key2 := userIDKey2{battleID: battleID, userID: userID}
	return m.lastSubmissionTime[key2], nil
}

func (m *mockBattleRepository) GetPlayerQuestionState(ctx context.Context, battleID, userID uuid.UUID) (PlayerQuestionState, error) {
	b, err := m.GetBattle(ctx, battleID)
	if err != nil {
		return PlayerQuestionState{}, err
	}
	p, err := m.GetBattlePlayer(ctx, battleID, userID)
	if err != nil {
		return PlayerQuestionState{}, err
	}
	qID := uuid.Nil
	if p.CurrentQuestionIndex >= 0 && p.CurrentQuestionIndex < len(m.sequence) {
		qID = m.sequence[p.CurrentQuestionIndex]
	}
	return PlayerQuestionState{
		BattleStatus:         b.Status,
		EndedAt:              b.EndedAt,
		CurrentQuestionIndex: p.CurrentQuestionIndex,
		QuestionCount:        len(m.sequence),
		QuestionID:           qID,
	}, nil
}

func (m *mockBattleRepository) GetQuestionIDAtSequenceIndex(ctx context.Context, battleID uuid.UUID, index int) (uuid.UUID, error) {
	if index < 0 || index >= len(m.sequence) {
		return uuid.Nil, errors.New("out of bounds")
	}
	return m.sequence[index], nil
}

func (m *mockBattleRepository) GetBattlePlayersTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID) ([]BattlePlayer, error) {
	var players []BattlePlayer
	for _, p := range m.players {
		if p.BattleID == battleID {
			players = append(players, p)
		}
	}
	if len(players) == 2 && players[0].UserID.String() > players[1].UserID.String() {
		players[0], players[1] = players[1], players[0]
	}
	return players, nil
}

func (m *mockBattleRepository) UpdateBattlePlayerResult(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID, result string) error {
	return nil
}

func (m *mockBattleRepository) UpdateRoomStatusDirect(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, status string) error {
	return nil
}

func (m *mockBattleRepository) CompleteBattleWithResultTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, winnerUserID *uuid.UUID, endedAt time.Time) error {
	if m.battle.ID == battleID {
		m.battle.Status = StatusCompleted
		m.battle.WinnerUserID = winnerUserID
		m.battle.EndedAt = &endedAt
		return nil
	}
	return ErrNotFound
}

func (m *mockBattleRepository) GetExpiredActiveBattles(ctx context.Context, now time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if m.battle.Status == StatusActive && m.battle.EndedAt != nil && m.battle.EndedAt.Before(now) {
		ids = append(ids, m.battle.ID)
	}
	return ids, nil
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

	clock := &mockClock{now: time.Now()}
	repo.clock = clock
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	roomID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()

	players := []BattlePlayer{
		{UserID: userID1, RatingBefore: 1200},
		{UserID: userID2, RatingBefore: 1300},
	}

	battleID, err := service.StartBattle(ctx, roomID, players, 42, 300)
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
			{ID: q1, IsActive: true, QuestionType: questions.TypeMCQ, Difficulty: 2},
			{ID: q2, IsActive: true, QuestionType: questions.TypeMCQ, Difficulty: 3},
		},
	}

	clock := &mockClock{now: time.Now()}
	repo.clock = clock
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	// Initializing mock database rows
	battleID := uuid.New()
	endedAt := clock.now.Add(300 * time.Second)
	repo.battle = Battle{
		ID:            battleID,
		Status:        StatusActive,
		EndedAt:       &endedAt,
		QuestionCount: MaxQuestionStreamSize,
	}
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

	// 1. Submit incorrect answer on attempt 1 (Index = 1)
	qs.validationMatch = false
	res, err := service.SubmitAnswer(ctx, battleID, userID, 1, questions.SubmissionAnswer{TextAnswer: "Wrong"}, 500)
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

	// 2. Submit incorrect answer on attempt 2 (Index = 2) (should skip)
	res, err = service.SubmitAnswer(ctx, battleID, userID, 2, questions.SubmissionAnswer{TextAnswer: "WrongAgain"}, 500)
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

	// 3. Submit correct answer on next question (index 1) (Index = 3)
	qs.validationMatch = true
	res, err = service.SubmitAnswer(ctx, battleID, userID, 3, questions.SubmissionAnswer{TextAnswer: "Right"}, 500)
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

	clock := &RealClock{}
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	battleID := uuid.New()
	repo.battle = Battle{ID: battleID, Status: StatusCompleted}
	userID := uuid.New()
	repo.players[userID] = BattlePlayer{
		BattleID: battleID,
		UserID:   userID,
	}

	_, err := service.SubmitAnswer(ctx, battleID, userID, 1, questions.SubmissionAnswer{TextAnswer: "test"}, 100)
	if !errors.Is(err, ErrBattleFinished) {
		t.Errorf("expected ErrBattleFinished, got: %v", err)
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
	clock := &RealClock{}
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	roomID := uuid.New()
	userID1 := uuid.New()
	players := []BattlePlayer{
		{UserID: userID1, RatingBefore: 1200},
	}

	battleID, err := service.StartBattleTx(ctx, nil, roomID, players, 42, 300)
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

func TestBattleService_SubmitAnswer_Expired(t *testing.T) {
	repo := newMockBattleRepository()
	q1 := uuid.New()

	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true, QuestionType: questions.TypeMCQ},
		},
	}

	clock := &mockClock{now: time.Now()}
	repo.clock = clock
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	battleID := uuid.New()
	endedAt := clock.now.Add(10 * time.Second)
	repo.battle = Battle{
		ID:            battleID,
		Status:        StatusActive,
		EndedAt:       &endedAt,
		QuestionCount: 1,
	}
	userID := uuid.New()
	repo.players[userID] = BattlePlayer{
		BattleID:                battleID,
		UserID:                  userID,
		CurrentQuestionIndex:    0,
		CurrentQuestionAttempts: 0,
	}
	repo.sequence = []uuid.UUID{q1}

	// Advance clock past expiration
	clock.now = clock.now.Add(15 * time.Second)

	_, err := service.SubmitAnswer(ctx, battleID, userID, 1, questions.SubmissionAnswer{TextAnswer: "Right"}, 100)
	if !errors.Is(err, ErrBattleExpired) {
		t.Errorf("expected ErrBattleExpired, got: %v", err)
	}
}

func TestBattleService_SubmitAnswer_Duplicate(t *testing.T) {
	repo := newMockBattleRepository()
	q1 := uuid.New()

	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true, QuestionType: questions.TypeMCQ, Difficulty: 2},
		},
	}

	clock := &mockClock{now: time.Now()}
	repo.clock = clock
	service := NewService(repo, qs, clock, MVPScoreCalculator{})
	ctx := context.Background()

	battleID := uuid.New()
	endedAt := clock.now.Add(300 * time.Second)
	repo.battle = Battle{
		ID:            battleID,
		Status:        StatusActive,
		EndedAt:       &endedAt,
		QuestionCount: 1,
	}
	userID := uuid.New()
	repo.players[userID] = BattlePlayer{
		BattleID:                battleID,
		UserID:                  userID,
		CurrentQuestionIndex:    0,
		CurrentQuestionAttempts: 0,
	}
	repo.sequence = []uuid.UUID{q1}

	// First submission (Expected = 1)
	qs.validationMatch = false
	_, err := service.SubmitAnswer(ctx, battleID, userID, 1, questions.SubmissionAnswer{TextAnswer: "Wrong"}, 100)
	if err != nil {
		t.Fatalf("unexpected error on first submit: %v", err)
	}

	// Try immediate submission of the same index/answer (Index = 1, expected = 2) (should fail due to index check)
	_, err = service.SubmitAnswer(ctx, battleID, userID, 1, questions.SubmissionAnswer{TextAnswer: "Wrong"}, 100)
	if !errors.Is(err, ErrDuplicateSubmission) {
		t.Errorf("expected ErrDuplicateSubmission (index check), got: %v", err)
	}

	// Submit correct index (Index = 2) but with the exact same answer (should fail due to same answer check)
	_, err = service.SubmitAnswer(ctx, battleID, userID, 2, questions.SubmissionAnswer{TextAnswer: "Wrong"}, 100)
	if !errors.Is(err, ErrDuplicateSubmission) {
		t.Errorf("expected ErrDuplicateSubmission (same answer check), got: %v", err)
	}

	// Submit a different answer (Index = 2)
	_, err = service.SubmitAnswer(ctx, battleID, userID, 2, questions.SubmissionAnswer{TextAnswer: "AnotherWrong"}, 100)
	if err != nil {
		t.Errorf("expected success for different answer, got error: %v", err)
	}
}

func TestBattleService_SubmitAnswer_Invalid(t *testing.T) {
	repo := newMockBattleRepository()
	service := NewService(repo, &mockQuestionsService{}, &RealClock{}, MVPScoreCalculator{})
	ctx := context.Background()

	_, err := service.SubmitAnswer(ctx, uuid.New(), uuid.New(), 1, questions.SubmissionAnswer{}, 100)
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Errorf("expected ErrInvalidSubmission, got: %v", err)
	}
}

func TestBattleService_CompleteBattle(t *testing.T) {
	repo := newMockBattleRepository()
	clock := &mockClock{now: time.Now()}
	service := NewService(repo, &mockQuestionsService{}, clock, MVPScoreCalculator{})
	ctx := context.Background()

	battleID := uuid.New()
	roomID := uuid.New()
	repo.battle = Battle{
		ID:     battleID,
		RoomID: roomID,
		Status: StatusActive,
	}

	userID1 := uuid.New()
	userID2 := uuid.New()

	repo.players[userID1] = BattlePlayer{
		BattleID: battleID,
		UserID:   userID1,
		Score:    5,
	}
	repo.players[userID2] = BattlePlayer{
		BattleID: battleID,
		UserID:   userID2,
		Score:    3,
	}

	// First complete call
	err := service.CompleteBattle(ctx, battleID)
	if err != nil {
		t.Fatalf("failed to complete battle: %v", err)
	}

	if repo.battle.Status != StatusCompleted {
		t.Errorf("expected status to be completed, got %v", repo.battle.Status)
	}
	if repo.battle.WinnerUserID == nil || *repo.battle.WinnerUserID != userID1 {
		t.Errorf("expected winner to be user 1 (%s), got: %v", userID1, repo.battle.WinnerUserID)
	}

	// Idempotent secondary call
	err = service.CompleteBattle(ctx, battleID)
	if err != nil {
		t.Fatalf("failed to complete battle idempotently: %v", err)
	}
}

func TestBattleService_ExpireActiveBattles(t *testing.T) {
	repo := newMockBattleRepository()
	now := time.Now()
	clock := &mockClock{now: now}
	service := NewService(repo, &mockQuestionsService{}, clock, MVPScoreCalculator{})
	ctx := context.Background()

	battleID := uuid.New()
	roomID := uuid.New()
	endedAt := now.Add(-5 * time.Minute)
	repo.battle = Battle {
		ID:     battleID,
		RoomID: roomID,
		Status: StatusActive,
		EndedAt: &endedAt,
	}

	userID1 := uuid.New()
	userID2 := uuid.New()
	repo.players[userID1] = BattlePlayer{
		BattleID: battleID,
		UserID:   userID1,
		Score:    5,
	}
	repo.players[userID2] = BattlePlayer{
		BattleID: battleID,
		UserID:   userID2,
		Score:    3,
	}

	count, err := service.ExpireActiveBattles(ctx)
	if err != nil {
		t.Fatalf("failed to expire active battles: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 expired battle, got: %d", count)
	}

	if repo.battle.Status != StatusCompleted {
		t.Errorf("expected battle status to transition to completed, got: %v", repo.battle.Status)
	}
}

