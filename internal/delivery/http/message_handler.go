package http

import (
	"context"
	"errors"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	platformstorage "github.com/aroundme/aroundme-backend/internal/platform/storage"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type MessageImageStore interface {
	Save(file *multipart.FileHeader) (string, error)
	Delete(publicPath string) error
}

type MessageHandler struct {
	authUseCase    usecase.AuthUseCase
	messageUseCase usecase.MessageUseCase
	streamHub      *MessageStreamHub
	imageStore     MessageImageStore
}

func NewMessageHandler(
	authUseCase usecase.AuthUseCase,
	messageUseCase usecase.MessageUseCase,
	streamHub *MessageStreamHub,
	imageStore MessageImageStore,
) *MessageHandler {
	return &MessageHandler{
		authUseCase:    authUseCase,
		messageUseCase: messageUseCase,
		streamHub:      streamHub,
		imageStore:     imageStore,
	}
}

func (h *MessageHandler) Register(rg fiber.Router) {
	auth := AuthRequired(h.authUseCase)

	// Friends list
	rg.Get("/friends", auth, h.listFriends)

	// Friend requests
	rg.Post("/friend-requests", auth, h.sendFriendRequest)
	rg.Get("/friend-requests/pending", auth, h.listPendingFriendRequests)
	rg.Post("/friend-requests/:id/respond", auth, h.respondFriendRequest)

	// Conversations
	rg.Get("/conversations", auth, h.listConversations)
	rg.Post("/conversations/direct", auth, h.getOrCreateDirectConversation)
	rg.Post("/conversations/group", auth, h.createGroupConversation)
	rg.Get("/conversations/:id", auth, h.getConversation)
	rg.Post("/conversations/:id/read", auth, h.markRead)

	// Messages
	rg.Get("/conversations/:id/messages", auth, h.listMessages)
	rg.Post("/conversations/:id/messages", auth, h.sendMessage)

	// Real-time stream (WebSocket) — auth via ?accessToken= query param
	rg.Use("/stream", h.authorizeStreamUpgrade)
	rg.Get("/stream", websocket.New(h.stream))
}

// ─── Friends list ─────────────────────────────────────────────────────────────

func (h *MessageHandler) listFriends(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	friends, err := h.messageUseCase.ListFriends(c.UserContext(), user.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": friends})
}

// ─── Friend requests ──────────────────────────────────────────────────────────

func (h *MessageHandler) sendFriendRequest(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var body struct {
		ReceiverID string `json:"receiverId"`
		Message    string `json:"message"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.ErrBadRequest
	}
	if body.ReceiverID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "receiverId is required")
	}

	resp, err := h.messageUseCase.SendFriendRequest(c.UserContext(), user.ID, body.ReceiverID, body.Message)
	if errors.Is(err, model.ErrCannotSelfRequest) {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *MessageHandler) listPendingFriendRequests(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	reqs, err := h.messageUseCase.ListPendingFriendRequests(c.UserContext(), user.ID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": reqs})
}

func (h *MessageHandler) respondFriendRequest(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	requestID := c.Params("id")

	var body struct {
		Accept bool `json:"accept"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.ErrBadRequest
	}

	resp, err := h.messageUseCase.RespondFriendRequest(c.UserContext(), requestID, user.ID, body.Accept)
	if errors.Is(err, model.ErrFriendRequestNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// ─── Conversations ─────────────────────────────────────────────────────────────

func (h *MessageHandler) listConversations(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	convs, err := h.messageUseCase.ListConversations(c.UserContext(), user.ID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": convs})
}

func (h *MessageHandler) getOrCreateDirectConversation(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var body model.CreateDirectConversationInput
	if err := c.BodyParser(&body); err != nil {
		return fiber.ErrBadRequest
	}
	if body.ParticipantID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "participantId is required")
	}

	conv, err := h.messageUseCase.GetOrCreateDirectConversation(c.UserContext(), user.ID, body.ParticipantID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.Status(fiber.StatusOK).JSON(conv)
}

func (h *MessageHandler) createGroupConversation(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	var body model.CreateGroupConversationInput
	if err := c.BodyParser(&body); err != nil {
		return fiber.ErrBadRequest
	}

	conv, err := h.messageUseCase.CreateGroupConversation(c.UserContext(), user.ID, body)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(conv)
}

func (h *MessageHandler) getConversation(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	convID := c.Params("id")

	conv, err := h.messageUseCase.GetConversation(c.UserContext(), convID, user.ID)
	if errors.Is(err, model.ErrConversationNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}

	return c.JSON(conv)
}

func (h *MessageHandler) markRead(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	convID := c.Params("id")

	if err := h.messageUseCase.MarkRead(c.UserContext(), convID, user.ID); err != nil {
		if errors.Is(err, model.ErrConversationNotFound) {
			return fiber.ErrNotFound
		}
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ─── Messages ─────────────────────────────────────────────────────────────────

func (h *MessageHandler) listMessages(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	convID := c.Params("id")

	limit := c.QueryInt("limit", 50)
	cursor := c.Query("cursor", "")

	result, err := h.messageUseCase.ListMessages(c.UserContext(), model.ListMessagesInput{
		ConversationID: convID,
		ViewerUserID:   user.ID,
		Cursor:         cursor,
		Limit:          limit,
	})
	if errors.Is(err, model.ErrConversationNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}

	return c.JSON(result)
}

func (h *MessageHandler) sendMessage(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	convID := c.Params("id")

	var msgBody, imageURL, locationName string
	var latitude, longitude *float64

	ct := strings.ToLower(c.Get(fiber.HeaderContentType))
	if strings.HasPrefix(ct, "multipart/form-data") {
		msgBody = strings.TrimSpace(c.FormValue("body"))
		locationName = strings.TrimSpace(c.FormValue("locationName"))
		if rawLatitude := strings.TrimSpace(c.FormValue("latitude")); rawLatitude != "" {
			parsedLatitude, parseErr := strconv.ParseFloat(rawLatitude, 64)
			if parseErr != nil {
				return fiber.NewError(fiber.StatusBadRequest, "latitude is invalid")
			}
			latitude = &parsedLatitude
		}
		if rawLongitude := strings.TrimSpace(c.FormValue("longitude")); rawLongitude != "" {
			parsedLongitude, parseErr := strconv.ParseFloat(rawLongitude, 64)
			if parseErr != nil {
				return fiber.NewError(fiber.StatusBadRequest, "longitude is invalid")
			}
			longitude = &parsedLongitude
		}
		if file, err := c.FormFile("image"); err == nil && file != nil {
			url, saveErr := h.imageStore.Save(file)
			if saveErr != nil {
				if errors.Is(saveErr, platformstorage.ErrImageTooLarge) {
					return fiber.NewError(fiber.StatusRequestEntityTooLarge, saveErr.Error())
				}
				if errors.Is(saveErr, platformstorage.ErrUnsupportedImageType) {
					return fiber.NewError(fiber.StatusUnprocessableEntity, saveErr.Error())
				}
				return saveErr
			}
			imageURL = url
		}
	} else {
		var req struct {
			Body     string `json:"body"`
			Location *struct {
				Label     string  `json:"label"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
		}
		if err := c.BodyParser(&req); err != nil {
			return fiber.ErrBadRequest
		}
		msgBody = strings.TrimSpace(req.Body)
		if req.Location != nil {
			locationName = strings.TrimSpace(req.Location.Label)
			latitude = &req.Location.Latitude
			longitude = &req.Location.Longitude
		}
	}

	msg, err := h.messageUseCase.SendMessage(c.UserContext(), model.SendMessageInput{
		ConversationID: convID,
		SenderID:       user.ID,
		Body:           msgBody,
		ImageURL:       imageURL,
		LocationName:   locationName,
		Latitude:       latitude,
		Longitude:      longitude,
	})
	if errors.Is(err, model.ErrConversationNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(msg)
}

// ─── Real-time stream ──────────────────────────────────────────────────────────

func (h *MessageHandler) authorizeStreamUpgrade(c *fiber.Ctx) error {
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

func (h *MessageHandler) stream(conn *websocket.Conn) {
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
