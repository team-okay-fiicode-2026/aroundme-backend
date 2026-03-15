package app

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"github.com/aroundme/aroundme-backend/internal/config"
	deliveryhttp "github.com/aroundme/aroundme-backend/internal/delivery/http"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	postgresrepository "github.com/aroundme/aroundme-backend/internal/repository/postgres"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type Application struct {
	Config   config.Config
	Database *database.Postgres
	HTTP     *fiber.App
}

func Bootstrap(ctx context.Context) (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	postgres, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	authRepository := postgresrepository.NewAuthRepository(postgres)

	authUseCase := usecase.NewAuthUseCase(authRepository, usecase.AuthConfig{
		AccessTokenTTL:     time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute,
		RefreshTokenTTL:    time.Duration(cfg.RefreshTokenTTLHours) * time.Hour,
		AllowDevSocialAuth: cfg.AllowDevSocialAuth,
	})

	app := fiber.New(fiber.Config{
		AppName:       "aroundme-backend",
		CaseSensitive: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigin,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,OPTIONS",
	}))

	deliveryhttp.Register(app, authUseCase, postgres)

	return &Application{
		Config:   cfg,
		Database: postgres,
		HTTP:     app,
	}, nil
}

func (a *Application) Close() {
	if a.Database != nil {
		a.Database.Close()
	}
}
