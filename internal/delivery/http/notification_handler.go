package http

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type NotificationHandler struct {
	authUseCase          usecase.AuthUseCase
	notificationUseCase  usecase.NotificationUseCase
	streamHub            *NotificationStreamHub
}

func NewNotificationHandler(
	authUseCase usecase.AuthUseCase,
	notificationUseCase usecase.NotificationUseCase,
	streamHub *NotificationStreamHub,
) *NotificationHandler {
	return &NotificationHandler{
		authUseCase:         authUseCase,
		notificationUseCase: notificationUseCase,
		streamHub:           streamHub,
	}
}

func (h *NotificationHandler) Register(rg fiber.Router) {
	auth := AuthRequired(h.authUseCase)

	rg.Get("/", auth, h.list)
	rg.Post("/read-all", auth, h.markAllRead)
	rg.Post("/:id/read", auth, h.markRead)
	rg.Post("/push-token", auth, h.registerPushToken)

	rg.Use("/stream", h.authorizeStreamUpgrade)
	rg.Get("/stream", websocket.New(h.stream))
}

func (h *NotificationHandler) list(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.notificationUseCase.List(ctx, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(result)
}

func (h *NotificationHandler) markRead(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	notifID := c.Params("id")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.notificationUseCase.MarkRead(ctx, notifID, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(result)
}

func (h *NotificationHandler) markAllRead(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.notificationUseCase.MarkAllRead(ctx, user.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *NotificationHandler) registerPushToken(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	if err := h.notificationUseCase.RegisterPushToken(ctx, user.ID, strings.TrimSpace(body.Token)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ─── Real-time stream ─────────────────────────────────────────────────────────

func (h *NotificationHandler) authorizeStreamUpgrade(c *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}

	accessToken := strings.TrimSpace(c.Query("accessToken"))
	if accessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "access token is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := h.authUseCase.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired access token"})
	}

	c.Locals(userContextKey, user)
	return c.Next()
}

func (h *NotificationHandler) stream(conn *websocket.Conn) {
	user, _ := conn.Locals(userContextKey).(entity.User)
	subID, events := h.streamHub.Subscribe(user.ID)
	defer h.streamHub.Unsubscribe(subID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
