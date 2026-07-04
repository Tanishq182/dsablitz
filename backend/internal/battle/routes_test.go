package battle

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dsablitz/backend/internal/questions"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func mockAuthMiddleware(userID uuid.UUID) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set("auth.user_id", userID.String())
		ctx.Next()
	}
}

func TestRoutes_GetNextQuestion_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockBattleRepository()
	q1 := uuid.New()
	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, Title: "Reverse List", Prompt: "Prompt", IsActive: true},
		},
	}
	clock := &mockClock{now: time.Now()}
	service := NewService(repo, qs, clock, MVPScoreCalculator{})

	battleID := uuid.New()
	userID := uuid.New()
	repo.battle = Battle{
		ID:     battleID,
		Status: StatusActive,
	}
	repo.players[userID] = BattlePlayer{
		BattleID:             battleID,
		UserID:               userID,
		CurrentQuestionIndex: 0,
	}
	repo.sequence = []uuid.UUID{q1}

	router := gin.New()
	RegisterRoutes(router, service, mockAuthMiddleware(userID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/"+battleID.String()+"/question", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got: %d", w.Code)
	}

	var resp questions.SanitizedQuestionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID != q1 {
		t.Errorf("expected question ID %s, got: %s", q1, resp.ID)
	}
}

func TestRoutes_GetNextQuestion_Expired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockBattleRepository()
	clock := &mockClock{now: time.Now()}
	service := NewService(repo, &mockQuestionsService{}, clock, MVPScoreCalculator{})

	battleID := uuid.New()
	userID := uuid.New()
	endedAt := clock.Now().Add(-1 * time.Minute)
	repo.battle = Battle{
		ID:      battleID,
		Status:  StatusActive,
		EndedAt: &endedAt,
	}
	repo.players[userID] = BattlePlayer{
		BattleID:             battleID,
		UserID:               userID,
		CurrentQuestionIndex: 0,
	}

	router := gin.New()
	RegisterRoutes(router, service, mockAuthMiddleware(userID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/"+battleID.String()+"/question", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410 Gone for expired battle, got: %d", w.Code)
	}
}

func TestRoutes_SubmitAnswer_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockBattleRepository()
	q1 := uuid.New()
	qs := &mockQuestionsService{
		activeQuestions: []questions.Question{
			{ID: q1, IsActive: true},
		},
		validationMatch: true,
	}
	clock := &mockClock{now: time.Now()}
	service := NewService(repo, qs, clock, MVPScoreCalculator{})

	battleID := uuid.New()
	userID := uuid.New()
	repo.battle = Battle{
		ID:            battleID,
		Status:        StatusActive,
		QuestionCount: 1,
	}
	repo.players[userID] = BattlePlayer{
		BattleID:             battleID,
		UserID:               userID,
		CurrentQuestionIndex: 0,
	}
	repo.sequence = []uuid.UUID{q1}

	router := gin.New()
	RegisterRoutes(router, service, mockAuthMiddleware(userID))

	submitReq := SubmitAnswerRequest{
		SubmissionIndex: 1,
		Answer: questions.SubmissionAnswer{
			TextAnswer: "O(1)",
		},
		ResponseTimeMs: 1200,
	}
	body, _ := json.Marshal(submitReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/"+battleID.String()+"/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got: %d (body: %s)", w.Code, w.Body.String())
	}

	var resp SubmissionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.IsCorrect {
		t.Errorf("expected submission to be marked correct")
	}
	if resp.Score != 1 {
		t.Errorf("expected score to increment to 1, got: %d", resp.Score)
	}
}

func TestRoutes_SubmitAnswer_Duplicate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockBattleRepository()
	service := NewService(repo, &mockQuestionsService{}, &RealClock{}, MVPScoreCalculator{})

	battleID := uuid.New()
	userID := uuid.New()
	repo.battle = Battle{
		ID:            battleID,
		Status:        StatusActive,
		QuestionCount: 1,
	}
	repo.players[userID] = BattlePlayer{
		BattleID:     battleID,
		UserID:       userID,
		CorrectCount: 1, // Next expected index is 2
	}
	repo.sequence = []uuid.UUID{uuid.New()}

	router := gin.New()
	RegisterRoutes(router, service, mockAuthMiddleware(userID))

	// Submit with index 1 (already processed, since CorrectCount=1)
	submitReq := SubmitAnswerRequest{
		SubmissionIndex: 1,
		Answer: questions.SubmissionAnswer{
			TextAnswer: "O(1)",
		},
	}
	body, _ := json.Marshal(submitReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/"+battleID.String()+"/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate submission, got: %d", w.Code)
	}
}
