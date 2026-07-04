package battle

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"dsablitz/backend/internal/questions"
)

// Clock defines a time provider interface for deterministic timer evaluation and testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the system clock.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// ScoreCalculator defines an interface for scoring calculations.
type ScoreCalculator interface {
	Calculate(isCorrect bool, attempts int, difficulty int) int
}

// MVPScoreCalculator implements a basic +1/0 scoring scheme.
type MVPScoreCalculator struct{}

func (MVPScoreCalculator) Calculate(isCorrect bool, attempts int, difficulty int) int {
	if isCorrect {
		return 1
	}
	return 0
}

// QuestionsService defines the read-only question adapter required by the battle module.
type QuestionsService interface {
	GetSanitizedQuestion(ctx context.Context, id uuid.UUID) (questions.SanitizedQuestionResponse, error)
	ValidateAnswer(ctx context.Context, questionID uuid.UUID, answer questions.SubmissionAnswer) (bool, error)
	GetActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]questions.Question, error)
}

// BattleRepository defines the interface for database queries in the battle module.
type BattleRepository interface {
	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
	InsertBattle(ctx context.Context, tx pgx.Tx, b Battle) error
	InsertBattlePlayers(ctx context.Context, tx pgx.Tx, players []BattlePlayer) error
	InsertBattleSequence(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, sequence []uuid.UUID) error
	GetBattle(ctx context.Context, id uuid.UUID) (Battle, error)
	GetBattleTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Battle, error)
	GetBattlePlayerForUpdate(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (BattlePlayer, error)
	GetBattlePlayer(ctx context.Context, battleID, userID uuid.UUID) (BattlePlayer, error)
	UpdateBattlePlayer(ctx context.Context, tx pgx.Tx, p BattlePlayer) error
	InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, answer questions.SubmissionAnswer, isCorrect bool, scoreAwarded int, responseTimeMs int) error
	GetSubmissionsForQuestion(ctx context.Context, tx pgx.Tx, battleID, userID, questionID uuid.UUID) ([]questions.SubmissionAnswer, error)
	GetLastSubmission(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (time.Time, error)
	GetPlayerQuestionState(ctx context.Context, battleID, userID uuid.UUID) (PlayerQuestionState, error)
	GetQuestionIDAtSequenceIndex(ctx context.Context, battleID uuid.UUID, index int) (uuid.UUID, error)
	GetBattlePlayersTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID) ([]BattlePlayer, error)
	UpdateBattlePlayerResult(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID, result string) error
	UpdateRoomStatusDirect(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, status string) error
	CompleteBattleWithResultTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, winnerUserID *uuid.UUID, endedAt time.Time) error
	GetExpiredActiveBattles(ctx context.Context, now time.Time) ([]uuid.UUID, error)
}

type Service struct {
	repo             BattleRepository
	questionsService QuestionsService
	clock            Clock
	scoreCalculator  ScoreCalculator
}

func NewService(repo BattleRepository, qs QuestionsService, clock Clock, scoreCalculator ScoreCalculator) *Service {
	return &Service{
		repo:             repo,
		questionsService: qs,
		clock:            clock,
		scoreCalculator:  scoreCalculator,
	}
}

// StartBattle initializes a battle sequence, saves metadata, player slots, and sequence indexes.
func (s *Service) StartBattle(ctx context.Context, roomID uuid.UUID, players []BattlePlayer, seed int64, durationSeconds int) (uuid.UUID, error) {
	var battleID uuid.UUID
	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		battleID, err = s.StartBattleTx(ctx, tx, roomID, players, seed, durationSeconds)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return battleID, nil
}

// StartBattleTx initializes a battle sequence inside the parent transaction.
func (s *Service) StartBattleTx(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, players []BattlePlayer, seed int64, durationSeconds int) (uuid.UUID, error) {
	// 1. Get active questions from Questions module
	activeQuestions, err := s.questionsService.GetActiveQuestionsByFilters(ctx, 0, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("fetch active questions for stream: %w", err)
	}
	if len(activeQuestions) == 0 {
		return uuid.Nil, errors.New("no active questions available to start battle")
	}

	// 2. Generate deterministic sequence
	sequence := s.GenerateSequence(activeQuestions, seed)

	// 3. Initialize Battle models
	battleID := uuid.New()
	now := s.clock.Now()
	ended := now.Add(time.Duration(durationSeconds) * time.Second)

	b := Battle{
		ID:              battleID,
		RoomID:          roomID,
		Status:          StatusActive,
		DurationSeconds: durationSeconds,
		QuestionCount:   len(sequence),
		StartedAt:       &now,
		EndedAt:         &ended,
		BattleSeed:      seed,
	}

	for i := range players {
		players[i].ID = uuid.New()
		players[i].BattleID = battleID
		players[i].CurrentQuestionIndex = 0
		players[i].CurrentQuestionAttempts = 0
		players[i].Score = 0
		players[i].CorrectCount = 0
		players[i].IncorrectCount = 0
	}

	// 4. Execute coordinated writes inside the parent transaction
	if err := s.repo.InsertBattle(ctx, tx, b); err != nil {
		return uuid.Nil, err
	}
	if err := s.repo.InsertBattlePlayers(ctx, tx, players); err != nil {
		return uuid.Nil, err
	}
	if err := s.repo.InsertBattleSequence(ctx, tx, battleID, sequence); err != nil {
		return uuid.Nil, err
	}

	return battleID, nil
}

func (s *Service) checkBattleActive(ctx context.Context, battleID uuid.UUID, status BattleStatus, endedAt *time.Time, completeOnExpiry bool) error {
	if status == StatusCompleted {
		return ErrBattleFinished
	}
	if status != StatusActive {
		return fmt.Errorf("battle status is %s", status)
	}
	if endedAt != nil && !s.clock.Now().Before(*endedAt) {
		if completeOnExpiry {
			_ = s.CompleteBattle(ctx, battleID)
		}
		return ErrBattleExpired
	}
	return nil
}

// GetNextQuestion resolves a player's progress pointer and returns the next sanitized question.
func (s *Service) GetNextQuestion(ctx context.Context, battleID, userID uuid.UUID) (questions.SanitizedQuestionResponse, error) {
	state, err := s.repo.GetPlayerQuestionState(ctx, battleID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return questions.SanitizedQuestionResponse{}, err
		}
		return questions.SanitizedQuestionResponse{}, fmt.Errorf("get player question state: %w", err)
	}

	if err := s.checkBattleActive(ctx, battleID, state.BattleStatus, state.EndedAt, true); err != nil {
		return questions.SanitizedQuestionResponse{}, err
	}

	if state.CurrentQuestionIndex >= state.QuestionCount || state.QuestionID == uuid.Nil {
		return questions.SanitizedQuestionResponse{}, ErrQuestionExhausted
	}

	return s.questionsService.GetSanitizedQuestion(ctx, state.QuestionID)
}

// SubmitAnswer evaluates the submission, updates progress counters, and logs statistics inside a row lock.
func (s *Service) SubmitAnswer(ctx context.Context, battleID, userID uuid.UUID, submissionIndex int, answer questions.SubmissionAnswer, responseTimeMs int) (SubmissionResult, error) {
	// Check answer format
	if answer.TextAnswer == "" && answer.NumericAnswer == nil && len(answer.OrderAnswer) == 0 {
		return SubmissionResult{}, ErrInvalidSubmission
	}

	// 1. Fast read-only check before transaction to avoid unnecessary locking if already completed or expired
	b, err := s.repo.GetBattle(ctx, battleID)
	if err == nil {
		if err := s.checkBattleActive(ctx, battleID, b.Status, b.EndedAt, true); err != nil {
			return SubmissionResult{}, err
		}
	}

	var result SubmissionResult

	err = s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Lock single player progress row pessimistic serialization
		player, err := s.repo.GetBattlePlayerForUpdate(ctx, tx, battleID, userID)
		if err != nil {
			return fmt.Errorf("lock player progression: %w", err)
		}

		// 2. Load battle
		b, err := s.repo.GetBattleTx(ctx, tx, battleID)
		if err != nil {
			return fmt.Errorf("load battle: %w", err)
		}

		// 3. Check expiration
		if err := s.checkBattleActive(ctx, battleID, b.Status, b.EndedAt, false); err != nil {
			return err
		}

		// 4. Resolve current question
		if player.CurrentQuestionIndex >= b.QuestionCount {
			return ErrQuestionExhausted
		}
		qID, err := s.repo.GetQuestionIDAtSequenceIndex(ctx, battleID, player.CurrentQuestionIndex)
		if err != nil {
			return ErrQuestionExhausted
		}
		result.QuestionID = qID

		// 5. Check for Duplicate Submission
		expectedIndex := player.CorrectCount + player.IncorrectCount + 1
		if submissionIndex < expectedIndex {
			return ErrDuplicateSubmission
		}
		if submissionIndex > expectedIndex {
			return fmt.Errorf("%w: expected submission index %d, got %d", ErrInvalidSubmission, expectedIndex, submissionIndex)
		}

		// Check for identical answers on the current question
		subs, err := s.repo.GetSubmissionsForQuestion(ctx, tx, battleID, userID, qID)
		if err != nil {
			return fmt.Errorf("check submissions for duplicate: %w", err)
		}
		for _, sAns := range subs {
			if answersEqual(sAns, answer) {
				return ErrDuplicateSubmission
			}
		}

		// Fetch question details for difficulty (safe, server-side only)
		q, err := s.questionsService.GetSanitizedQuestion(ctx, qID)
		if err != nil {
			return fmt.Errorf("get sanitized question: %w", err)
		}

		// Stateless evaluation
		isCorrect, err := s.questionsService.ValidateAnswer(ctx, qID, answer)
		if err != nil {
			return fmt.Errorf("validate answer: %w", err)
		}

		result.IsCorrect = isCorrect

		// Apply Option C progression policy
		if isCorrect {
			scoreAwarded := s.scoreCalculator.Calculate(true, int(player.CurrentQuestionAttempts)+1, int(q.Difficulty))
			player.Score += scoreAwarded
			player.CorrectCount++
			player.CurrentQuestionIndex++
			player.CurrentQuestionAttempts = 0
			result.Score = player.Score
		} else {
			player.IncorrectCount++
			player.CurrentQuestionAttempts++
			if player.CurrentQuestionAttempts >= 2 {
				// Skip question
				player.CurrentQuestionIndex++
				player.CurrentQuestionAttempts = 0
			}
			result.Score = int(player.Score)
		}

		result.AttemptsMade = int(player.CurrentQuestionAttempts)
		result.CurrentQuestionIndex = int(player.CurrentQuestionIndex)

		// Log submission
		scoreAwarded := 0
		if isCorrect {
			scoreAwarded = s.scoreCalculator.Calculate(true, int(player.CurrentQuestionAttempts), int(q.Difficulty))
		}
		err = s.repo.InsertSubmission(ctx, tx, battleID, userID, qID, answer, isCorrect, scoreAwarded, responseTimeMs)
		if err != nil {
			return fmt.Errorf("log submission: %w", err)
		}

		// Update scorecard
		err = s.repo.UpdateBattlePlayer(ctx, tx, player)
		if err != nil {
			return fmt.Errorf("update battle player: %w", err)
		}

		return nil
	})

	if errors.Is(err, ErrBattleExpired) {
		_ = s.CompleteBattle(ctx, battleID)
		return SubmissionResult{}, ErrBattleExpired
	}

	return result, err
}

// CompleteBattle transitions battle status to completed inside a transaction context.
func (s *Service) CompleteBattle(ctx context.Context, battleID uuid.UUID) error {
	return s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Load battle metadata
		b, err := s.repo.GetBattleTx(ctx, tx, battleID)
		if err != nil {
			return fmt.Errorf("get battle tx: %w", err)
		}
		if b.Status == StatusCompleted {
			return nil // Idempotent noop
		}

		// 2. Load players under FOR UPDATE (lock ordering is guaranteed by ORDER BY user_id ASC in repository query)
		players, err := s.repo.GetBattlePlayersTx(ctx, tx, battleID)
		if err != nil {
			return fmt.Errorf("load battle players tx: %w", err)
		}

		// Re-read battle status after locking players to ensure idempotency under concurrency
		b, err = s.repo.GetBattleTx(ctx, tx, battleID)
		if err != nil {
			return fmt.Errorf("re-read battle tx: %w", err)
		}
		if b.Status == StatusCompleted {
			return nil // Idempotent noop
		}

		// 3. Determine winner based on score
		var winnerUserID *uuid.UUID
		endedAt := s.clock.Now()

		if len(players) == 2 {
			p1 := players[0]
			p2 := players[1]

			var res1, res2 string
			if p1.Score > p2.Score {
				winnerUserID = &p1.UserID
				res1 = "win"
				res2 = "loss"
			} else if p2.Score > p1.Score {
				winnerUserID = &p2.UserID
				res1 = "loss"
				res2 = "win"
			} else {
				res1 = "draw"
				res2 = "draw"
			}

			if err := s.repo.UpdateBattlePlayerResult(ctx, tx, battleID, p1.UserID, res1); err != nil {
				return fmt.Errorf("update player 1 result: %w", err)
			}
			if err := s.repo.UpdateBattlePlayerResult(ctx, tx, battleID, p2.UserID, res2); err != nil {
				return fmt.Errorf("update player 2 result: %w", err)
			}
		} else if len(players) == 1 {
			// Single player lobby edge case
			winnerUserID = &players[0].UserID
			if err := s.repo.UpdateBattlePlayerResult(ctx, tx, battleID, players[0].UserID, "win"); err != nil {
				return fmt.Errorf("update player result: %w", err)
			}
		}

		// 4. Update room status directly to 'ready'
		if b.RoomID != uuid.Nil {
			if err := s.repo.UpdateRoomStatusDirect(ctx, tx, b.RoomID, "ready"); err != nil {
				return fmt.Errorf("update room status direct to ready: %w", err)
			}
		}

		// 5. Complete the battle with winner ID
		if err := s.repo.CompleteBattleWithResultTx(ctx, tx, battleID, winnerUserID, endedAt); err != nil {
			return fmt.Errorf("complete battle status: %w", err)
		}

		return nil
	})
}

// GenerateSequence generates a deterministic reshuffled question stream of size MaxQuestionStreamSize.
func (s *Service) GenerateSequence(activeQuestions []questions.Question, seed int64) []uuid.UUID {
	r := rand.New(rand.NewSource(seed))
	sequence := make([]uuid.UUID, 0, MaxQuestionStreamSize)

	for len(sequence) < MaxQuestionStreamSize {
		poolCopy := make([]questions.Question, len(activeQuestions))
		copy(poolCopy, activeQuestions)

		r.Shuffle(len(poolCopy), func(i, j int) {
			poolCopy[i], poolCopy[j] = poolCopy[j], poolCopy[i]
		})

		for _, q := range poolCopy {
			if len(sequence) < MaxQuestionStreamSize {
				sequence = append(sequence, q.ID)
			}
		}
	}
	return sequence
}

func answersEqual(a, b questions.SubmissionAnswer) bool {
	if a.TextAnswer != b.TextAnswer {
		return false
	}
	if (a.NumericAnswer == nil) != (b.NumericAnswer == nil) {
		return false
	}
	if a.NumericAnswer != nil && b.NumericAnswer != nil && *a.NumericAnswer != *b.NumericAnswer {
		return false
	}
	if len(a.OrderAnswer) != len(b.OrderAnswer) {
		return false
	}
	for i := range a.OrderAnswer {
		if a.OrderAnswer[i] != b.OrderAnswer[i] {
			return false
		}
	}
	return true
}

// ExpireActiveBattles scans and completes all expired battles.
func (s *Service) ExpireActiveBattles(ctx context.Context) (int, error) {
	now := s.clock.Now()
	ids, err := s.repo.GetExpiredActiveBattles(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("get expired active battles: %w", err)
	}

	var count int
	for _, id := range ids {
		if err := s.CompleteBattle(ctx, id); err != nil {
			// Log the error and continue to avoid blocking other expirations
			log.Printf("Failed to complete expired battle %s: %v", id, err)
			continue
		}
		count++
	}

	return count, nil
}

