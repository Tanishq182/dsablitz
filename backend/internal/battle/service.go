package battle

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"dsablitz/backend/internal/questions"
)

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
	GetBattlePlayerForUpdate(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (BattlePlayer, error)
	GetBattlePlayer(ctx context.Context, battleID, userID uuid.UUID) (BattlePlayer, error)
	UpdateBattlePlayer(ctx context.Context, tx pgx.Tx, p BattlePlayer) error
	InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, isCorrect bool, responseTimeMs int) error
	GetQuestionIDAtSequenceIndex(ctx context.Context, battleID uuid.UUID, index int) (uuid.UUID, error)
	CompleteBattle(ctx context.Context, battleID uuid.UUID) error
}

type Service struct {
	repo             BattleRepository
	questionsService QuestionsService
}

func NewService(repo BattleRepository, qs QuestionsService) *Service {
	return &Service{
		repo:             repo,
		questionsService: qs,
	}
}

// StartBattle initializes a battle sequence, saves metadata, player slots, and sequence indexes.
func (s *Service) StartBattle(ctx context.Context, roomID uuid.UUID, players []BattlePlayer, seed int64) (uuid.UUID, error) {
	var battleID uuid.UUID
	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		battleID, err = s.StartBattleTx(ctx, tx, roomID, players, seed)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return battleID, nil
}

// StartBattleTx initializes a battle sequence inside the parent transaction.
func (s *Service) StartBattleTx(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, players []BattlePlayer, seed int64) (uuid.UUID, error) {
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
	b := Battle{
		ID:         battleID,
		RoomID:     roomID,
		Status:     StatusActive,
		BattleSeed: seed,
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

// GetNextQuestion resolves a player's progress pointer and returns the next sanitized question.
func (s *Service) GetNextQuestion(ctx context.Context, battleID, userID uuid.UUID) (questions.SanitizedQuestionResponse, error) {
	b, err := s.repo.GetBattle(ctx, battleID)
	if err != nil {
		return questions.SanitizedQuestionResponse{}, err
	}
	if b.Status == StatusCompleted {
		return questions.SanitizedQuestionResponse{}, errors.New("battle is completed")
	}

	player, err := s.repo.GetBattlePlayer(ctx, battleID, userID)
	if err != nil {
		return questions.SanitizedQuestionResponse{}, fmt.Errorf("get player progression pointer: %w", err)
	}

	if player.CurrentQuestionIndex >= MaxQuestionStreamSize {
		return questions.SanitizedQuestionResponse{}, errors.New("finished all questions in stream")
	}

	qID, err := s.repo.GetQuestionIDAtSequenceIndex(ctx, battleID, player.CurrentQuestionIndex)
	if err != nil {
		return questions.SanitizedQuestionResponse{}, fmt.Errorf("resolve sequence index mapping: %w", err)
	}

	return s.questionsService.GetSanitizedQuestion(ctx, qID)
}

// SubmitAnswer evaluates the submission, updates progress counters, and logs statistics inside a row lock.
func (s *Service) SubmitAnswer(ctx context.Context, battleID, userID uuid.UUID, answer questions.SubmissionAnswer, responseTimeMs int) (SubmissionResult, error) {
	b, err := s.repo.GetBattle(ctx, battleID)
	if err != nil {
		return SubmissionResult{}, err
	}
	if b.Status != StatusActive {
		return SubmissionResult{}, fmt.Errorf("cannot submit: battle is currently %s", b.Status)
	}

	var result SubmissionResult

	err = s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Lock single player progress row pessimistic serialization
		player, err := s.repo.GetBattlePlayerForUpdate(ctx, tx, battleID, userID)
		if err != nil {
			return fmt.Errorf("lock player progression: %w", err)
		}

		if player.CurrentQuestionIndex >= MaxQuestionStreamSize {
			return errors.New("finished all questions in stream")
		}

		// 2. Fetch the mapped question ID from sequence
		qID, err := s.repo.GetQuestionIDAtSequenceIndex(ctx, battleID, player.CurrentQuestionIndex)
		if err != nil {
			return fmt.Errorf("resolve sequence index: %w", err)
		}
		result.QuestionID = qID

		// 3. Request stateless validation check
		isCorrect, err := s.questionsService.ValidateAnswer(ctx, qID, answer)
		if err != nil {
			return fmt.Errorf("stateless validation query: %w", err)
		}
		result.IsCorrect = isCorrect

		// 4. Calculate progression counters & points (Option C: 2 Attempts Skip Policy)
		pointsEarned := 0
		if isCorrect {
			pointsEarned = calculateScore(true)
			player.Score += pointsEarned
			player.CorrectCount++
			player.CurrentQuestionIndex++
			player.CurrentQuestionAttempts = 0
		} else {
			player.IncorrectCount++
			player.CurrentQuestionAttempts++
			if player.CurrentQuestionAttempts >= 2 {
				player.CurrentQuestionIndex++
				player.CurrentQuestionAttempts = 0
			}
		}

		result.AttemptsMade = player.CurrentQuestionAttempts
		result.CurrentQuestionIndex = player.CurrentQuestionIndex
		result.Score = player.Score

		// 5. Log submission
		err = s.repo.InsertSubmission(ctx, tx, battleID, userID, qID, isCorrect, responseTimeMs)
		if err != nil {
			return err
		}

		// 6. Persist counters
		err = s.repo.UpdateBattlePlayer(ctx, tx, player)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return SubmissionResult{}, err
	}

	return result, nil
}

// CompleteBattle transitions battle status to completed.
func (s *Service) CompleteBattle(ctx context.Context, battleID uuid.UUID) error {
	return s.repo.CompleteBattle(ctx, battleID)
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

// calculateScore is a ScoreCalculator helper to compute points awarded.
func calculateScore(isCorrect bool) int {
	if isCorrect {
		return 1
	}
	return 0
}
