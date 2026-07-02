package server

import (
	"context"
	"dsablitz/backend/configs"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config configs.Config
	router *gin.Engine
	http   *http.Server
}

func New(config configs.Config) *Server {
	router := gin.New()
	router.Use(middlewares()...)
	registerRoutes(router)

	httpServer := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: router,
	}

	return &Server{
		config: config,
		router: router,
		http:   httpServer,
	}
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
