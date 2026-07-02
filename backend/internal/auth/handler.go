package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	accessCookieName  = "access_token"
	refreshCookieName = "refresh_token"

	accessCookiePath  = "/"
	refreshCookiePath = "/api/v1/auth"
)

type Handler struct {
    service      *Service
    tokens       *TokenManager
    cookieSecure bool
}

func NewHandler(
    service *Service,
    tokens *TokenManager,
    cookieSecure bool,
) *Handler {
    return &Handler{
        service:      service,
        tokens:       tokens,
        cookieSecure: cookieSecure,
    }
}
func (h *Handler) Signup(ctx *gin.Context) {
	var request SignupRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.Signup(ctx.Request.Context(), request, clientInfo(ctx))
	if err != nil {
		writeAuthError(ctx, err)
		return
	}

	h.setAuthCookies(ctx, result)
	ctx.JSON(http.StatusCreated, AuthResponse{User: result.User})
}

func (h *Handler) Login(ctx *gin.Context) {
	var request LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.Login(ctx.Request.Context(), request, clientInfo(ctx))
	if err != nil {
		writeAuthError(ctx, err)
		return
	}

	h.setAuthCookies(ctx, result)
	ctx.JSON(http.StatusOK, AuthResponse{User: result.User})
}

func (h *Handler) Refresh(ctx *gin.Context) {
	refreshToken, err := ctx.Cookie(refreshCookieName)
	if err != nil {
		writeError(ctx, http.StatusUnauthorized, ErrUnauthorized.Error())
		return
	}

	result, err := h.service.Refresh(ctx.Request.Context(), refreshToken, clientInfo(ctx))
	if err != nil {
		h.clearAuthCookies(ctx)
		writeAuthError(ctx, err)
		return
	}

	h.setAuthCookies(ctx, result)
	ctx.JSON(http.StatusOK, AuthResponse{User: result.User})
}

func (h *Handler) Logout(ctx *gin.Context) {
	refreshToken, _ := ctx.Cookie(refreshCookieName)
	if err := h.service.Logout(ctx.Request.Context(), refreshToken); err != nil {
		writeAuthError(ctx, err)
		return
	}

	h.clearAuthCookies(ctx)
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) Me(ctx *gin.Context) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, ErrUnauthorized.Error())
		return
	}

	user, err := h.service.Me(ctx.Request.Context(), userID)
	if err != nil {
		writeAuthError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, AuthResponse{User: user})
}

func (h *Handler) setAuthCookies(ctx *gin.Context, result AuthResult) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		accessCookieName,
		result.AccessToken,
		int(h.tokens.AccessTokenTTL().Seconds()),
		accessCookiePath,
		"",
		h.cookieSecure, // changed
		true,
	)
	ctx.SetCookie(
		refreshCookieName,
		result.RefreshToken,
		int(h.tokens.RefreshTokenTTL().Seconds()),
		refreshCookiePath,
		"",
		h.cookieSecure, // changed
		true,
	)
}

func (h *Handler) clearAuthCookies(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		accessCookieName,
		"",
		-1,
		accessCookiePath,
		"",
		h.cookieSecure, // changed
		true,
	)
	ctx.SetCookie(
		refreshCookieName,
		"",
		-1,
		refreshCookiePath,
		"",
		h.cookieSecure, // changed
		true,
	)
}
func clientInfo(ctx *gin.Context) ClientInfo {
	return ClientInfo{
		UserAgent: ctx.Request.UserAgent(),
		IPAddress: ctx.ClientIP(),
	}
}

func writeAuthError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken), errors.Is(err, ErrHandleTaken):
		writeError(ctx, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidToken):
		writeError(ctx, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserDisabled):
		writeError(ctx, http.StatusForbidden, err.Error())
	default:
		writeError(ctx, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(ctx *gin.Context, status int, message string) {
	ctx.AbortWithStatusJSON(status, ErrorResponse{Error: message})
}
