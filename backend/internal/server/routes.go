package server

import (
	"net/http"

	"dsablitz/backend/internal/auth"
	"dsablitz/backend/internal/battle"
	"dsablitz/backend/internal/questions"
	"dsablitz/backend/internal/rooms"
	"dsablitz/backend/internal/users"

	"github.com/gin-gonic/gin"
)

func registerRoutes(router *gin.Engine) {
	router.GET("/health", healthHandler)

	api := router.Group("/api/v1")
	auth.RegisterRoutes(api.Group("/auth"))
	users.RegisterRoutes(api.Group("/users"))
	rooms.RegisterRoutes(api.Group("/rooms"))
	battle.RegisterRoutes(api.Group("/battle"))
	questions.RegisterRoutes(api.Group("/questions"))
}

func healthHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
