package server

import (
	"context"
	"dsablitz/backend/configs"
	"dsablitz/backend/internal/platform/database"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config configs.Config
	router *gin.Engine
	http   *http.Server
}

func New(config configs.Config, db *database.Manager) (*Server, error) {
	router := gin.New()
	router.Use(middlewares()...)
	if err := registerRoutes(router, config, db); err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: router,
	}

	return &Server{
		config: config,
		router: router,
		http:   httpServer,
	}, nil
}

func (s *Server) Run() error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) Handler() *gin.Engine {
	return s.router
}
