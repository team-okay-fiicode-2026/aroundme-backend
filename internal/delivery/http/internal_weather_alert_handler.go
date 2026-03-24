package http

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type InternalWeatherAlertHandler struct {
	useCase        usecase.InternalWeatherAlertUseCase
	internalAPIKey string
	env            string
}

func NewInternalWeatherAlertHandler(
	useCase usecase.InternalWeatherAlertUseCase,
	internalAPIKey string,
	env string,
) *InternalWeatherAlertHandler {
	return &InternalWeatherAlertHandler{
		useCase:        useCase,
		internalAPIKey: internalAPIKey,
		env:            env,
	}
}

func (h *InternalWeatherAlertHandler) Register(rg fiber.Router) {
	rg.Use(h.requireInternalKey)
	rg.Post("/sync", h.sync)
}

func (h *InternalWeatherAlertHandler) requireInternalKey(c *fiber.Ctx) error {
	if h.env != "production" {
		return c.Next()
	}
	if h.internalAPIKey == "" || c.Get("X-Internal-Key") != h.internalAPIKey {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.Next()
}

func (h *InternalWeatherAlertHandler) sync(c *fiber.Ctx) error {
	var body struct {
		Provider   string    `json:"provider"`
		ObservedAt time.Time `json:"observedAt"`
		Alerts     []struct {
			ExternalID       string          `json:"externalId"`
			Event            string          `json:"event"`
			Headline         string          `json:"headline"`
			Description      string          `json:"description"`
			Instruction      string          `json:"instruction"`
			AreaDesc         string          `json:"areaDesc"`
			Severity         string          `json:"severity"`
			ProviderSeverity string          `json:"providerSeverity"`
			Urgency          string          `json:"urgency"`
			Certainty        string          `json:"certainty"`
			Source           string          `json:"source"`
			SourceURL        string          `json:"sourceUrl"`
			StartsAt         *time.Time      `json:"startsAt"`
			EndsAt           *time.Time      `json:"endsAt"`
			ExpiresAt        *time.Time      `json:"expiresAt"`
			Geometry         json.RawMessage `json:"geometry"`
			Center           struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"center"`
			RadiusKm float64 `json:"radiusKm"`
		} `json:"alerts"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	input := entity.WeatherAlertSyncInput{
		Provider:   strings.TrimSpace(body.Provider),
		ObservedAt: body.ObservedAt,
		Alerts:     make([]entity.WeatherAlertUpsertInput, 0, len(body.Alerts)),
	}
	for _, alert := range body.Alerts {
		input.Alerts = append(input.Alerts, entity.WeatherAlertUpsertInput{
			Provider:         input.Provider,
			ExternalID:       alert.ExternalID,
			Event:            alert.Event,
			Headline:         alert.Headline,
			Description:      alert.Description,
			Instruction:      alert.Instruction,
			AreaDesc:         alert.AreaDesc,
			Severity:         entity.WeatherAlertSeverity(strings.ToLower(strings.TrimSpace(alert.Severity))),
			ProviderSeverity: alert.ProviderSeverity,
			Urgency:          alert.Urgency,
			Certainty:        alert.Certainty,
			Source:           alert.Source,
			SourceURL:        alert.SourceURL,
			StartsAt:         alert.StartsAt,
			EndsAt:           alert.EndsAt,
			ExpiresAt:        alert.ExpiresAt,
			Geometry:         alert.Geometry,
			CenterLatitude:   alert.Center.Latitude,
			CenterLongitude:  alert.Center.Longitude,
			RadiusKm:         alert.RadiusKm,
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()

	result, err := h.useCase.SyncWeatherAlerts(ctx, input)
	if err != nil {
		if _, ok := err.(model.ValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(fiber.Map{
		"createdCount":  result.CreatedCount,
		"updatedCount":  result.UpdatedCount,
		"resolvedCount": result.ResolvedCount,
		"notifiedCount": result.NotifiedCount,
	})
}
