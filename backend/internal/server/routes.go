package server

import (
	"net/http"

	"dsablitz/backend/configs"
	"dsablitz/backend/internal/auth"
	"dsablitz/backend/internal/battle"
	"dsablitz/backend/internal/platform/database"
	"dsablitz/backend/internal/questions"
	"dsablitz/backend/internal/rooms"
	"dsablitz/backend/internal/users"

	"github.com/gin-gonic/gin"
)

func registerRoutes(router *gin.Engine, config configs.Config, db *database.Manager) error {
	router.GET("/health", healthHandler)

	api := router.Group("/api/v1")
	if err := auth.RegisterRoutes(api.Group("/auth"), config, db); err != nil {
		return err
	}
	users.RegisterRoutes(api.Group("/users"))
	rooms.RegisterRoutes(api.Group("/rooms"))
	battle.RegisterRoutes(api.Group("/battle"))
	questions.RegisterRoutes(api.Group("/questions"))

	return nil
}

func healthHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"checks": gin.H{},
	})
}
