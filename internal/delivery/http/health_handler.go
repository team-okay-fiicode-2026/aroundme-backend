package http

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/platform/database"
)

type HealthHandler struct {
	db *database.Postgres
}

func NewHealthHandler(db *database.Postgres) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Register(app fiber.Router) {
	app.Get("/health", h.health)
}

func (h *HealthHandler) health(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	status, dbStatus, statusCode := "ok", "up", stdhttp.StatusOK
	if err := h.db.Ping(ctx); err != nil {
		status, dbStatus, statusCode = "degraded", "down", stdhttp.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status":   status,
		"database": dbStatus,
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}
