package server

import (
	"context"
	"fmt"
	"net/http"

	"dsablitz/backend/configs"
	"dsablitz/backend/internal/auth"
	"dsablitz/backend/internal/battle"
	"dsablitz/backend/internal/platform/database"
	"dsablitz/backend/internal/questions"
	"dsablitz/backend/internal/rooms"
	"dsablitz/backend/internal/users"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type battleCoordinatorAdapter struct {
	battleService *battle.Service
}

func (a *battleCoordinatorAdapter) StartBattle(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, players []rooms.BattlePlayer, seed int64) (uuid.UUID, error) {
	battlePlayers := make([]battle.BattlePlayer, len(players))
	for i, p := range players {
		battlePlayers[i] = battle.BattlePlayer{
			UserID:       p.UserID,
			SeatNumber:   p.SeatNumber,
			RatingBefore: p.RatingBefore,
		}
	}
	return a.battleService.StartBattleTx(ctx, tx, roomID, battlePlayers, seed, 300)
}

func registerRoutes(router *gin.Engine, config configs.Config, db *database.Manager) (*rooms.Service, *battle.Service, error) {
	router.GET("/health", healthHandler)

	api := router.Group("/api/v1")
	if err := auth.RegisterRoutes(api.Group("/auth"), config, db); err != nil {
		return nil, nil, err
	}
	users.RegisterRoutes(api.Group("/users"))

	// 1. Initialize Questions Module and load startup cache
	questionsRepo := questions.NewRepository(db.Pool())
	questionsService := questions.NewService(questionsRepo)
	if err := questionsService.LoadCache(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("failed to load questions cache: %w", err)
	}
	questions.RegisterRoutes(api.Group("/questions"))

	// 2. Initialize Battle Module
	battleRepo := battle.NewRepository(db.Pool())
	battleService := battle.NewService(battleRepo, questionsService, battle.RealClock{}, battle.MVPScoreCalculator{})
	battle.RegisterRoutes(api.Group("/battle"))

	// 3. Initialize Rooms Module with BattleCoordinator dependency inversion
	roomsRepo := rooms.NewRepository(db.Pool())
	battleAdapter := &battleCoordinatorAdapter{battleService: battleService}
	roomsService := rooms.NewService(roomsRepo, battleAdapter)

	// Auth token manager and middleware for rooms
	tokens, err := auth.NewTokenManager(config.JWTSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("auth token manager initialization: %w", err)
	}
	authMiddleware := auth.JWTMiddleware(tokens)

	rooms.RegisterRoutes(api.Group("/rooms"), roomsService, authMiddleware)

	return roomsService, battleService, nil
}

func healthHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"checks": gin.H{},
	})
}
