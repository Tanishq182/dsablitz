package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"dsablitz/backend/configs"
	"dsablitz/backend/internal/platform/cache"
	"dsablitz/backend/internal/platform/database"
	"dsablitz/backend/internal/questions"
	"dsablitz/backend/internal/server"
)

const (
	startupTimeout  = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	cfg := configs.Load()

	seedFlag := flag.Bool("seed", false, "Seed the database with question bank and exit")
	flag.Parse()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelStartup()

	db, err := database.Connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if *seedFlag {
		log.Println("Starting question database seeding...")
		repo := questions.NewRepository(db.Pool())
		count, err := questions.SeedQuestions(context.Background(), repo, "internal/questions/seeds/questions.json")
		if err != nil {
			log.Fatalf("seeding failed: %v", err)
		}
		log.Printf("Successfully seeded %d questions. Exiting.", count)
		return
	}

	redisClient, err := cache.Connect(startupCtx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("redis close failed: %v", err)
		}
	}()

	srv, err := server.New(cfg, db)
	if err != nil {
		log.Fatalf("server initialization failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server stopped: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancelShutdown()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("server shutdown failed: %v", err)
		}

		if err := <-errCh; err != nil {
			log.Fatalf("server stopped: %v", err)
		}
	}
}
