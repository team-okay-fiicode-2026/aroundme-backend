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
	"github.com/aroundme/aroundme-backend/internal/platform/push"
	"github.com/aroundme/aroundme-backend/internal/platform/storage"
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
	if err := database.RunMigrations(ctx, postgres, cfg.MigrationsDir); err != nil {
		postgres.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	authRepository := postgresrepository.NewAuthRepository(postgres)
	profileRepository := postgresrepository.NewProfileRepository(postgres)
	postRepository := postgresrepository.NewPostRepository(postgres)
	messageRepository := postgresrepository.NewMessageRepository(postgres)
	notificationRepository := postgresrepository.NewNotificationRepository(postgres)

	postStreamHub := deliveryhttp.NewPostStreamHub()
	messageStreamHub := deliveryhttp.NewMessageStreamHub()
	notificationStreamHub := deliveryhttp.NewNotificationStreamHub()

	postImageStore, err := storage.NewLocalImageStore(cfg.UploadsDir, "posts", "post", 10<<20)
	if err != nil {
		return nil, fmt.Errorf("create post image store: %w", err)
	}
	messageImageStore, err := storage.NewLocalImageStore(cfg.UploadsDir, "messages", "msg", 10<<20)
	if err != nil {
		return nil, fmt.Errorf("create message image store: %w", err)
	}
	avatarImageStore, err := storage.NewLocalImageStore(cfg.UploadsDir, "avatars", "avatar", 5<<20)
	if err != nil {
		return nil, fmt.Errorf("create avatar image store: %w", err)
	}

	authUseCase := usecase.NewAuthUseCase(authRepository, usecase.AuthConfig{
		AccessTokenTTL:     time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute,
		RefreshTokenTTL:    time.Duration(cfg.RefreshTokenTTLHours) * time.Hour,
		AllowDevSocialAuth: cfg.AllowDevSocialAuth,
	})
	profileUseCase := usecase.NewProfileUseCase(profileRepository, authRepository, nil)

	expoPusher := push.NewExpoClient(cfg.ExpoPushAccessToken)
	notificationService := usecase.NewNotificationService(notificationRepository, notificationStreamHub, expoPusher)

	postUseCase := usecase.NewPostUseCase(postRepository, nil, postStreamHub, notificationService)
	messageUseCase := usecase.NewMessageUseCase(messageRepository, messageStreamHub, notificationService)

	app := fiber.New(fiber.Config{
		AppName:       "aroundme-backend",
		CaseSensitive: true,
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   120 * time.Second,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigin,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PATCH,PUT,DELETE,OPTIONS",
	}))
	app.Static("/uploads", cfg.UploadsDir)

	deliveryhttp.Register(app, authUseCase, profileUseCase, postUseCase, postStreamHub, postImageStore, messageUseCase, messageStreamHub, messageImageStore, avatarImageStore, notificationService, notificationStreamHub, postgres)

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
