package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"dsablitz/backend/configs"
	"dsablitz/backend/internal/battle"
	"dsablitz/backend/internal/platform/database"
	"dsablitz/backend/internal/rooms"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config        configs.Config
	router        *gin.Engine
	http          *http.Server
	roomsService  *rooms.Service
	battleService *battle.Service
	ctx           context.Context
	cancel        context.CancelFunc
}

func New(config configs.Config, db *database.Manager) (*Server, error) {
	router := gin.New()
	router.Use(middlewares()...)
	roomsService, battleService, err := registerRoutes(router, config, db)
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: router,
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:        config,
		router:        router,
		http:          httpServer,
		roomsService:  roomsService,
		battleService: battleService,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

func (s *Server) Run() error {
	go s.startCleanupWorkers()

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	return s.http.Shutdown(ctx)
}

func (s *Server) Handler() *gin.Engine {
	return s.router
}

func (s *Server) startCleanupWorkers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if s.roomsService != nil {
				count, err := s.roomsService.ExpireRooms(s.ctx)
				if err != nil {
					log.Printf("Lobby expiration worker error: %v", err)
				} else if count > 0 {
					log.Printf("Lobby expiration worker: expired %d idle lobbies", count)
				}
			}

			if s.battleService != nil {
				count, err := s.battleService.ExpireActiveBattles(s.ctx)
				if err != nil {
					log.Printf("Battle timer expiration worker error: %v", err)
				} else if count > 0 {
					log.Printf("Battle timer expiration worker: completed %d expired active battles", count)
				}
			}
		}
	}
}

