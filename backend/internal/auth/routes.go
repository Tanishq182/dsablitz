package auth

import (
	"dsablitz/backend/configs"
	"dsablitz/backend/internal/platform/database"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router gin.IRouter, config configs.Config, db *database.Manager) error {
	tokens, err := NewTokenManager(config.JWTSecret)
	if err != nil {
		return err
	}

	repository := NewRepository(db.Pool())
	service := NewService(repository, tokens)
	handler := NewHandler(service, tokens, config.IsProduction())
	router.POST("/signup", handler.Signup)
	router.POST("/login", handler.Login)
	router.POST("/refresh", handler.Refresh)
	router.POST("/logout", handler.Logout)
	router.GET("/me", JWTMiddleware(tokens), handler.Me)

	return nil
}
