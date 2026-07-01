package server

import (
	"dsablitz/backend/configs"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config configs.Config
	router *gin.Engine
}

func New(config configs.Config) *Server {
	router := gin.New()
	router.Use(middlewares()...)
	registerRoutes(router)

	return &Server{
		config: config,
		router: router,
	}
}

func (s *Server) Run() error {
	return s.router.Run(s.config.HTTPAddr)
}

func (s *Server) Handler() *gin.Engine {
	return s.router
}
