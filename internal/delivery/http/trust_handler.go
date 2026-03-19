package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type TrustHandler struct {
	trustUseCase usecase.TrustUseCase
}

func NewTrustHandler(trustUseCase usecase.TrustUseCase) *TrustHandler {
	return &TrustHandler{trustUseCase: trustUseCase}
}

func (h *TrustHandler) Register(app fiber.Router) {
	app.Get("/users/:id/trust-score", h.getTrustScore)
	app.Post("/trust/interactions", h.createInteraction)
	app.Post("/trust/interactions/:id/feedback", h.recordFeedback)
}

func (h *TrustHandler) getTrustScore(c *fiber.Ctx) error {
	userID := strings.TrimSpace(c.Params("id"))

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.trustUseCase.GetTrustScore(ctx, userID)
	if err != nil {
		return writeTrustError(c, err)
	}

	return c.JSON(result)
}

func (h *TrustHandler) createInteraction(c *fiber.Ctx) error {
	authUser := GetAuthUser(c)

	var input model.CreateTrustInteractionInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.trustUseCase.CreateInteraction(ctx, authUser.ID, input)
	if err != nil {
		return writeTrustError(c, err)
	}

	return c.Status(stdhttp.StatusCreated).JSON(result)
}

func (h *TrustHandler) recordFeedback(c *fiber.Ctx) error {
	authUser := GetAuthUser(c)
	interactionID := strings.TrimSpace(c.Params("id"))

	var input model.RecordTrustFeedbackInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.trustUseCase.RecordFeedback(ctx, interactionID, authUser.ID, input)
	if err != nil {
		return writeTrustError(c, err)
	}

	return c.JSON(result)
}

func writeTrustError(c *fiber.Ctx, err error) error {
	var validationError model.ValidationError
	switch {
	case errors.As(err, &validationError):
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{"error": validationError.Error()})
	case errors.Is(err, model.ErrProfileNotFound):
		return c.Status(stdhttp.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	case errors.Is(err, model.ErrTrustInteractionSelf):
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{"error": model.ErrTrustInteractionSelf.Error()})
	case errors.Is(err, model.ErrTrustInteractionNotFound):
		return c.Status(stdhttp.StatusNotFound).JSON(fiber.Map{"error": model.ErrTrustInteractionNotFound.Error()})
	case errors.Is(err, model.ErrTrustInteractionFeedbackCompleted):
		return c.Status(stdhttp.StatusConflict).JSON(fiber.Map{"error": model.ErrTrustInteractionFeedbackCompleted.Error()})
	default:
		return c.Status(stdhttp.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
