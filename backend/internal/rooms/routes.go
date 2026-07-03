package rooms

import (
	"errors"
	"net/http"

	"dsablitz/backend/internal/auth"

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
		protected.POST("", handler.CreateRoom)
		protected.POST("/:code/join", handler.JoinRoom)
		protected.POST("/:code/ready", handler.ToggleReady)
		protected.POST("/:code/leave", handler.LeaveRoom)
		protected.POST("/:code/start-battle", handler.StartBattle)
	}
}

type CreateRoomRequest struct {
	DurationSeconds int `json:"duration_seconds" binding:"required"`
}

func (h *Handler) CreateRoom(ctx *gin.Context) {
	var req CreateRoomRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	room, err := h.service.CreateRoom(ctx.Request.Context(), userID, req.DurationSeconds)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusCreated, room)
}

func (h *Handler) JoinRoom(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		writeError(ctx, http.StatusBadRequest, "room code is required")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	room, err := h.service.JoinRoom(ctx.Request.Context(), userID, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(ctx, http.StatusNotFound, "room not found")
			return
		}
		writeError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, room)
}

type ToggleReadyRequest struct {
	Ready bool `json:"ready"`
}

func (h *Handler) ToggleReady(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		writeError(ctx, http.StatusBadRequest, "room code is required")
		return
	}

	var req ToggleReadyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	room, err := h.service.ToggleReady(ctx.Request.Context(), userID, code, req.Ready)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(ctx, http.StatusNotFound, "room not found")
			return
		}
		writeError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, room)
}

func (h *Handler) LeaveRoom(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		writeError(ctx, http.StatusBadRequest, "room code is required")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.service.LeaveRoom(ctx.Request.Context(), userID, code)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) StartBattle(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		writeError(ctx, http.StatusBadRequest, "room code is required")
		return
	}

	userID, ok := getAuthenticatedUserID(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "unauthorized")
		return
	}

	battleID, err := h.service.StartBattle(ctx.Request.Context(), userID, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(ctx, http.StatusNotFound, "room not found")
			return
		}
		writeError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"battle_id": battleID})
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
