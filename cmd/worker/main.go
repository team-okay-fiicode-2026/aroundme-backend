package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/aroundme/aroundme-backend/internal/config"
	"github.com/aroundme/aroundme-backend/internal/platform/ai"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	postgresrepository "github.com/aroundme/aroundme-backend/internal/repository/postgres"
	"github.com/aroundme/aroundme-backend/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.AnthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required for the AI tagger worker")
	}

	postgres, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer postgres.Close()

	postRepository := postgresrepository.NewPostRepository(postgres)

	tagger := ai.NewClaudeTagger(cfg.AnthropicAPIKey)

	w := worker.NewPostTaggerWorker(postRepository, tagger)
	log.Println("AI post tagger worker started")
	w.Run(ctx)
	log.Println("AI post tagger worker stopped")
}
