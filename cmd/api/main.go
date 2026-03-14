package main

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"github.com/aroundme/aroundme-backend/internal/config"
	"github.com/aroundme/aroundme-backend/internal/db"
	"github.com/aroundme/aroundme-backend/internal/graph"
	"github.com/aroundme/aroundme-backend/internal/rest"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	postgres, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer postgres.Close()

	app := fiber.New(fiber.Config{
		AppName:       "aroundme-backend",
		CaseSensitive: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigin,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,OPTIONS",
	}))

	rest.Register(app, postgres, cfg)
	graph.Register(app, postgres)

	log.Printf("aroundme-backend listening on :%s", cfg.AppPort)
	if err := app.Listen(":" + cfg.AppPort); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
