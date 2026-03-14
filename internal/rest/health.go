package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/config"
	"github.com/aroundme/aroundme-backend/internal/db"
)

type healthResponse struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Database    string `json:"database"`
	Environment string `json:"environment"`
	Time        string `json:"time"`
}

func Register(app *fiber.App, postgres *db.Postgres, cfg config.Config) {
	app.Get("/health", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		statusCode := http.StatusOK
		response := healthResponse{
			Status:      "ok",
			Service:     "aroundme-backend",
			Database:    "up",
			Environment: cfg.Env,
			Time:        time.Now().UTC().Format(time.RFC3339),
		}

		if err := postgres.Ping(ctx); err != nil {
			statusCode = http.StatusServiceUnavailable
			response.Status = "degraded"
			response.Database = "down"
		}

		return c.Status(statusCode).JSON(response)
	})
}
