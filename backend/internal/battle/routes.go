package battle

import (
	"errors"
	"net/http"

	"dsablitz/backend/internal/auth"
	"dsablitz/backend/internal/questions"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router gin.IRouter, service *Service, jwtMiddleware gin.HandlerFunc) {
	handler := NewHandler(service)

	protected := router.Group("")
	protected.Use(jwtMiddleware)
	{
		protected.GET("/:id/question", handler.GetNextQuestion)
		protected.POST("/:id/submit", handler.SubmitAnswer)
	}
}

func (h *Handler) GetNextQuestion(ctx *gin.Context) {
	battleID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid battle ID")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	q, err := h.service.GetNextQuestion(ctx.Request.Context(), battleID, userID)
	if err != nil {
		if errors.Is(err, ErrBattleFinished) {
			writeError(ctx, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrBattleExpired) {
			writeError(ctx, http.StatusGone, err.Error())
			return
		}
		if errors.Is(err, ErrQuestionExhausted) {
			ctx.Status(http.StatusNoContent)
			return
		}
		if errors.Is(err, ErrNotFound) {
			writeError(ctx, http.StatusNotFound, "battle or player not found")
			return
		}
		writeError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, q)
}

type SubmitAnswerRequest struct {
	SubmissionIndex int                        `json:"submission_index" binding:"required"`
	Answer          questions.SubmissionAnswer `json:"answer"`
	ResponseTimeMs  int                        `json:"response_time_ms"`
}

func (h *Handler) SubmitAnswer(ctx *gin.Context) {
	battleID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid battle ID")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req SubmitAnswerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.service.SubmitAnswer(ctx.Request.Context(), battleID, userID, req.SubmissionIndex, req.Answer, req.ResponseTimeMs)
	if err != nil {
		if errors.Is(err, ErrBattleFinished) {
			writeError(ctx, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrBattleExpired) {
			writeError(ctx, http.StatusGone, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateSubmission) {
			writeError(ctx, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidSubmission) {
			writeError(ctx, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrQuestionExhausted) {
			ctx.Status(http.StatusNoContent)
			return
		}
		if errors.Is(err, ErrNotFound) {
			writeError(ctx, http.StatusNotFound, "battle or player not found")
			return
		}
		writeError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func getAuthenticatedUserID(ctx *gin.Context) (uuid.UUID, bool) {
	userIDStr, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	uID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return uID, true
}

func writeError(ctx *gin.Context, code int, msg string) {
	ctx.JSON(code, gin.H{"error": msg})
}
