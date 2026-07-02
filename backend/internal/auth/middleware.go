package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "auth.user_id"

func JWTMiddleware(tokens *TokenManager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rawToken, err := ctx.Cookie(accessCookieName)
		if err != nil {
			writeError(ctx, http.StatusUnauthorized, ErrUnauthorized.Error())
			return
		}

		claims, err := tokens.VerifyAccessToken(rawToken)
		if err != nil {
			writeError(ctx, http.StatusUnauthorized, ErrInvalidToken.Error())
			return
		}

		ctx.Set(userIDContextKey, claims.Subject)
		ctx.Next()
	}
}

func UserIDFromContext(ctx *gin.Context) (string, bool) {
	value, ok := ctx.Get(userIDContextKey)
	if !ok {
		return "", false
	}

	userID, ok := value.(string)
	return userID, ok && userID != ""
}
