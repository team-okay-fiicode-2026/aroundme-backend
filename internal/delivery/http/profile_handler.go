package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type ProfileHandler struct {
	profileUseCase usecase.ProfileUseCase
}

func NewProfileHandler(profileUseCase usecase.ProfileUseCase) *ProfileHandler {
	return &ProfileHandler{profileUseCase: profileUseCase}
}

func (h *ProfileHandler) Register(app fiber.Router) {
	app.Get("/", h.getProfile)
	app.Patch("/", h.updateProfile)
	app.Delete("/", h.deleteAccount)
	app.Put("/skills", h.setSkills)
	app.Post("/items", h.createItem)
	app.Patch("/items/:id", h.updateItem)
	app.Delete("/items/:id", h.deleteItem)
}

func (h *ProfileHandler) getProfile(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	profile, err := h.profileUseCase.GetProfile(ctx, user.ID)
	if err != nil {
		return writeProfileError(c, err)
	}

	return c.JSON(presentProfile(profile))
}

func (h *ProfileHandler) updateProfile(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var input model.UpdateProfileInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	profile, err := h.profileUseCase.UpdateProfile(ctx, user.ID, input)
	if err != nil {
		return writeProfileError(c, err)
	}

	return c.JSON(presentProfile(profile))
}

func (h *ProfileHandler) deleteAccount(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.profileUseCase.DeleteAccount(ctx, user.ID); err != nil {
		return writeProfileError(c, err)
	}

	return c.SendStatus(stdhttp.StatusNoContent)
}

func (h *ProfileHandler) setSkills(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var input model.SetSkillsInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	tags, err := h.profileUseCase.SetSkills(ctx, user.ID, input)
	if err != nil {
		return writeProfileError(c, err)
	}

	return c.JSON(fiber.Map{"tags": tags})
}

func (h *ProfileHandler) createItem(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var input model.CreateItemInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	item, err := h.profileUseCase.CreateItem(ctx, user.ID, input)
	if err != nil {
		return writeProfileError(c, err)
	}

	return c.Status(stdhttp.StatusCreated).JSON(presentItem(item))
}

func (h *ProfileHandler) updateItem(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	itemID := c.Params("id")

	var input model.UpdateItemInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	item, err := h.profileUseCase.UpdateItem(ctx, user.ID, itemID, input)
	if err != nil {
		return writeProfileError(c, err)
	}

	return c.JSON(presentItem(item))
}

func (h *ProfileHandler) deleteItem(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	itemID := c.Params("id")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.profileUseCase.DeleteItem(ctx, user.ID, itemID); err != nil {
		return writeProfileError(c, err)
	}

	return c.SendStatus(stdhttp.StatusNoContent)
}

type profileResponse struct {
	ID                   string         `json:"id"`
	Email                string         `json:"email"`
	Name                 string         `json:"name"`
	AvatarURL            string         `json:"avatarUrl"`
	Bio                  string         `json:"bio"`
	Latitude             *float64       `json:"latitude"`
	Longitude            *float64       `json:"longitude"`
	NeighborhoodRadiusKm float64        `json:"neighborhoodRadiusKm"`
	QuietHoursStart      *string        `json:"quietHoursStart"`
	QuietHoursEnd        *string        `json:"quietHoursEnd"`
	DistanceLimitKm      float64        `json:"distanceLimitKm"`
	Skills               []string       `json:"skills"`
	Items                []itemResponse `json:"items"`
}

type itemResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Available   bool   `json:"available"`
}

func presentProfile(p entity.Profile) profileResponse {
	skills := p.Skills
	if skills == nil {
		skills = []string{}
	}

	items := make([]itemResponse, len(p.Items))
	for i, item := range p.Items {
		items[i] = presentItem(item)
	}
	if items == nil {
		items = []itemResponse{}
	}

	return profileResponse{
		ID:                   p.ID,
		Email:                p.Email,
		Name:                 p.Name,
		AvatarURL:            p.AvatarURL,
		Bio:                  p.Bio,
		Latitude:             p.Latitude,
		Longitude:            p.Longitude,
		NeighborhoodRadiusKm: p.NeighborhoodRadiusKm,
		QuietHoursStart:      p.QuietHoursStart,
		QuietHoursEnd:        p.QuietHoursEnd,
		DistanceLimitKm:      p.DistanceLimitKm,
		Skills:               skills,
		Items:                items,
	}
}

func presentItem(item entity.ProfileItem) itemResponse {
	return itemResponse{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Category:    item.Category,
		Available:   item.Available,
	}
}

func writeProfileError(c *fiber.Ctx, err error) error {
	var validationError model.ValidationError
	switch {
	case errors.As(err, &validationError):
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": validationError.Error(),
		})
	case errors.Is(err, model.ErrProfileNotFound):
		return c.Status(stdhttp.StatusNotFound).JSON(fiber.Map{
			"error": "profile not found",
		})
	case errors.Is(err, model.ErrItemNotFound):
		return c.Status(stdhttp.StatusNotFound).JSON(fiber.Map{
			"error": "item not found",
		})
	default:
		return c.Status(stdhttp.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}
}
