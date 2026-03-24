package http

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type InternalPostHandler struct {
	useCase        usecase.InternalPostUseCase
	internalAPIKey string
	env            string
}

func NewInternalPostHandler(useCase usecase.InternalPostUseCase, internalAPIKey, env string) *InternalPostHandler {
	return &InternalPostHandler{
		useCase:        useCase,
		internalAPIKey: internalAPIKey,
		env:            env,
	}
}

func (h *InternalPostHandler) Register(rg fiber.Router) {
	rg.Use(h.requireInternalKey)
	rg.Post("/:id/classification", h.writeClassification)
}

func (h *InternalPostHandler) requireInternalKey(c *fiber.Ctx) error {
	if h.env != "production" {
		return c.Next()
	}
	if h.internalAPIKey == "" || c.Get("X-Internal-Key") != h.internalAPIKey {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.Next()
}

func (h *InternalPostHandler) writeClassification(c *fiber.Ctx) error {
	postID := strings.TrimSpace(c.Params("id"))
	if postID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "post id is required"})
	}

	var body struct {
		PostType   string   `json:"post_type"`
		Urgency    string   `json:"urgency"`
		Confidence float64  `json:"confidence"`
		Rationale  string   `json:"rationale"`
		Tags       []string `json:"tags"`
		Status     string   `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	body.Status = strings.ToLower(strings.TrimSpace(body.Status))
	body.PostType = strings.ToLower(strings.TrimSpace(body.PostType))
	body.Urgency = strings.ToLower(strings.TrimSpace(body.Urgency))

	if body.Status != "classified" && body.Status != "failed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status must be classified or failed"})
	}

	if body.Status == "classified" {
		validTypes := map[string]bool{
			"emergency": true,
			"request":   true,
			"offer":     true,
			"item":      true,
			"event":     true,
		}
		if !validTypes[body.PostType] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid post_type"})
		}

		validUrgencies := map[string]bool{
			"critical": true,
			"high":     true,
			"normal":   true,
		}
		if !validUrgencies[body.Urgency] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid urgency"})
		}
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	if err := h.useCase.WriteClassification(ctx, postID, entity.PostClassificationInput{
		Status:     body.Status,
		PostType:   entity.PostCategory(body.PostType),
		Urgency:    entity.PostUrgency(body.Urgency),
		Confidence: body.Confidence,
		Rationale:  body.Rationale,
		Tags:       body.Tags,
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
