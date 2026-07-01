package main

import (
	"log"

	"dsablitz/backend/configs"
	"dsablitz/backend/internal/server"
)

func main() {
	cfg := configs.Load()

	srv := server.New(cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
