package http

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type InternalNotificationHandler struct {
	useCase usecase.InternalNotificationUseCase
}

func NewInternalNotificationHandler(useCase usecase.InternalNotificationUseCase) *InternalNotificationHandler {
	return &InternalNotificationHandler{useCase: useCase}
}

func (h *InternalNotificationHandler) Register(rg fiber.Router) {
	rg.Post("/skill-match", h.enqueueSkillMatch)
}

func (h *InternalNotificationHandler) enqueueSkillMatch(c *fiber.Ctx) error {
	var body struct {
		PostID string   `json:"postId"`
		UserIDs []string `json:"userIds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	body.PostID = strings.TrimSpace(body.PostID)
	if body.PostID == "" || len(body.UserIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "postId and userIds are required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.useCase.EnqueueSkillMatchNotifications(ctx, body.PostID, body.UserIDs); err != nil {
		if _, ok := err.(model.ValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.SendStatus(fiber.StatusAccepted)
}
