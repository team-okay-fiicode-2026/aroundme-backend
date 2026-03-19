package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

const (
	maxMessageBodyLength   = 4000
	maxGroupNameLength     = 80
	maxGroupParticipants   = 50
	defaultMessageLimit    = 50
	messageNewEventType    = "message.new"
	messageReadEventType   = "message.read"
)

type MessageEventPublisher interface {
	PublishToUser(userID string, event model.MessageStreamEvent)
}

type MessageUseCase interface {
	// Friend requests
	SendFriendRequest(ctx context.Context, senderID, receiverID, message string) (model.FriendRequestResponse, error)
	ListPendingFriendRequests(ctx context.Context, userID string) ([]model.FriendRequestResponse, error)
	RespondFriendRequest(ctx context.Context, requestID, userID string, accept bool) (model.FriendRequestResponse, error)

	// Conversations
	GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID string) (model.ConversationResponse, error)
	CreateGroupConversation(ctx context.Context, creatorID string, input model.CreateGroupConversationInput) (model.ConversationResponse, error)
	GetConversation(ctx context.Context, conversationID, userID string) (model.ConversationResponse, error)
	ListConversations(ctx context.Context, userID string) ([]model.ConversationResponse, error)

	// Messages
	SendMessage(ctx context.Context, input model.SendMessageInput) (model.MessageResponse, error)
	ListMessages(ctx context.Context, input model.ListMessagesInput) (model.ListMessagesResult, error)
	MarkRead(ctx context.Context, conversationID, userID string) error

	// Friends
	ListFriends(ctx context.Context, userID string) ([]model.FriendResponse, error)
}

type noopMessageEventPublisher struct{}

func (noopMessageEventPublisher) PublishToUser(string, model.MessageStreamEvent) {}

type messageUseCase struct {
	messageRepository repository.MessageRepository
	publisher         MessageEventPublisher
	notifier          MessageNotifier
}

func NewMessageUseCase(messageRepository repository.MessageRepository, publisher MessageEventPublisher, notifier MessageNotifier) MessageUseCase {
	if publisher == nil {
		publisher = noopMessageEventPublisher{}
	}
	if notifier == nil {
		notifier = noopMessageNotifier{}
	}
	return &messageUseCase{
		messageRepository: messageRepository,
		publisher:         publisher,
		notifier:          notifier,
	}
}

// ─── Friend requests ──────────────────────────────────────────────────────────

func (u *messageUseCase) SendFriendRequest(ctx context.Context, senderID, receiverID, message string) (model.FriendRequestResponse, error) {
	if senderID == receiverID {
		return model.FriendRequestResponse{}, model.ErrCannotSelfRequest
	}

	req, err := u.messageRepository.SendFriendRequest(ctx, senderID, receiverID, strings.TrimSpace(message))
	if err != nil {
		return model.FriendRequestResponse{}, err
	}

	return presentFriendRequest(req), nil
}

func (u *messageUseCase) ListPendingFriendRequests(ctx context.Context, userID string) ([]model.FriendRequestResponse, error) {
	reqs, err := u.messageRepository.ListPendingFriendRequests(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]model.FriendRequestResponse, len(reqs))
	for i, r := range reqs {
		out[i] = presentFriendRequest(r)
	}
	return out, nil
}

func (u *messageUseCase) RespondFriendRequest(ctx context.Context, requestID, userID string, accept bool) (model.FriendRequestResponse, error) {
	req, err := u.messageRepository.RespondFriendRequest(ctx, requestID, userID, accept)
	if errors.Is(err, repository.ErrNotFound) {
		return model.FriendRequestResponse{}, model.ErrFriendRequestNotFound
	}
	if err != nil {
		return model.FriendRequestResponse{}, err
	}

	// When accepted, create the direct conversation and post the request message as the first message
	if accept && strings.TrimSpace(req.Message) != "" {
		conv, convErr := u.messageRepository.GetOrCreateDirectConversation(ctx, req.SenderID, req.ReceiverID)
		if convErr == nil {
			msg, msgErr := u.messageRepository.SendMessage(ctx, conv.ID, req.SenderID, req.Message, "", "", nil, nil)
			if msgErr == nil {
				event := model.MessageStreamEvent{
					Type:           messageNewEventType,
					ConversationID: conv.ID,
					MessageID:      msg.ID,
				}
				u.publisher.PublishToUser(req.SenderID, event)
				u.publisher.PublishToUser(req.ReceiverID, event)
			}
		}
	}

	return presentFriendRequest(req), nil
}

// ─── Conversations ─────────────────────────────────────────────────────────────

func (u *messageUseCase) GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID string) (model.ConversationResponse, error) {
	if userID == otherUserID {
		return model.ConversationResponse{}, fmt.Errorf("cannot create conversation with yourself")
	}

	conv, err := u.messageRepository.GetOrCreateDirectConversation(ctx, userID, otherUserID)
	if err != nil {
		return model.ConversationResponse{}, err
	}

	return presentConversation(conv), nil
}

func (u *messageUseCase) CreateGroupConversation(ctx context.Context, creatorID string, input model.CreateGroupConversationInput) (model.ConversationResponse, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.ConversationResponse{}, fmt.Errorf("group name is required")
	}
	if len(name) > maxGroupNameLength {
		return model.ConversationResponse{}, fmt.Errorf("group name too long (max %d characters)", maxGroupNameLength)
	}
	if len(input.ParticipantIDs) == 0 {
		return model.ConversationResponse{}, fmt.Errorf("at least one participant is required")
	}
	if len(input.ParticipantIDs)+1 > maxGroupParticipants {
		return model.ConversationResponse{}, fmt.Errorf("too many participants (max %d)", maxGroupParticipants)
	}

	conv, err := u.messageRepository.CreateGroupConversation(ctx, creatorID, name, input.ParticipantIDs)
	if err != nil {
		return model.ConversationResponse{}, err
	}

	return presentConversation(conv), nil
}

func (u *messageUseCase) GetConversation(ctx context.Context, conversationID, userID string) (model.ConversationResponse, error) {
	conv, err := u.messageRepository.GetConversation(ctx, conversationID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.ConversationResponse{}, model.ErrConversationNotFound
	}
	if err != nil {
		return model.ConversationResponse{}, err
	}

	return presentConversation(conv), nil
}

func (u *messageUseCase) ListConversations(ctx context.Context, userID string) ([]model.ConversationResponse, error) {
	convs, err := u.messageRepository.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]model.ConversationResponse, len(convs))
	for i, c := range convs {
		out[i] = presentConversation(c)
	}
	return out, nil
}

// ─── Messages ─────────────────────────────────────────────────────────────────

func (u *messageUseCase) SendMessage(ctx context.Context, input model.SendMessageInput) (model.MessageResponse, error) {
	body := strings.TrimSpace(input.Body)
	imageURL := strings.TrimSpace(input.ImageURL)
	locationName := strings.TrimSpace(input.LocationName)
	if body == "" && imageURL == "" && input.Latitude == nil && input.Longitude == nil {
		return model.MessageResponse{}, fmt.Errorf("message must have text, an image, or a location")
	}
	if len(body) > maxMessageBodyLength {
		return model.MessageResponse{}, fmt.Errorf("message too long (max %d characters)", maxMessageBodyLength)
	}
	if (input.Latitude == nil) != (input.Longitude == nil) {
		return model.MessageResponse{}, fmt.Errorf("location is incomplete")
	}
	if input.Latitude != nil {
		if locationName == "" {
			return model.MessageResponse{}, fmt.Errorf("location label is required")
		}
		if *input.Latitude < -90 || *input.Latitude > 90 {
			return model.MessageResponse{}, fmt.Errorf("location latitude is invalid")
		}
		if *input.Longitude < -180 || *input.Longitude > 180 {
			return model.MessageResponse{}, fmt.Errorf("location longitude is invalid")
		}
	}

	ok, err := u.messageRepository.IsParticipant(ctx, input.ConversationID, input.SenderID)
	if err != nil {
		return model.MessageResponse{}, err
	}
	if !ok {
		return model.MessageResponse{}, model.ErrConversationNotFound
	}

	msg, err := u.messageRepository.SendMessage(ctx, input.ConversationID, input.SenderID, body, imageURL, locationName, input.Latitude, input.Longitude)
	if err != nil {
		return model.MessageResponse{}, err
	}

	resp := presentMessage(msg)

	// Fan-out real-time event to all conversation participants
	conv, err := u.messageRepository.GetConversation(ctx, input.ConversationID, input.SenderID)
	if err == nil {
		event := model.MessageStreamEvent{
			Type:           messageNewEventType,
			ConversationID: input.ConversationID,
			MessageID:      msg.ID,
		}
		recipientIDs := make([]string, 0, len(conv.Participants))
		for _, p := range conv.Participants {
			u.publisher.PublishToUser(p.UserID, event)
			if p.UserID != input.SenderID {
				recipientIDs = append(recipientIDs, p.UserID)
			}
		}
		go u.notifier.NotifyNewMessage(context.Background(), input.ConversationID, input.SenderID, msg.SenderName, string(conv.Kind), conv.Name, recipientIDs)
	}

	return resp, nil
}

func (u *messageUseCase) ListMessages(ctx context.Context, input model.ListMessagesInput) (model.ListMessagesResult, error) {
	ok, err := u.messageRepository.IsParticipant(ctx, input.ConversationID, input.ViewerUserID)
	if err != nil {
		return model.ListMessagesResult{}, err
	}
	if !ok {
		return model.ListMessagesResult{}, model.ErrConversationNotFound
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = defaultMessageLimit
	}

	cursor, err := decodeMessageCursor(input.Cursor)
	if err != nil {
		return model.ListMessagesResult{}, fmt.Errorf("invalid cursor")
	}

	messages, nextCursor, err := u.messageRepository.ListMessages(ctx, input.ConversationID, cursor, limit)
	if err != nil {
		return model.ListMessagesResult{}, err
	}

	items := make([]model.MessageResponse, len(messages))
	for i, m := range messages {
		items[i] = presentMessage(m)
	}

	return model.ListMessagesResult{
		Items:      items,
		NextCursor: encodeMessageCursor(nextCursor),
	}, nil
}

func (u *messageUseCase) MarkRead(ctx context.Context, conversationID, userID string) error {
	ok, err := u.messageRepository.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrConversationNotFound
	}

	if err := u.messageRepository.MarkRead(ctx, conversationID, userID); err != nil {
		return err
	}

	u.publisher.PublishToUser(userID, model.MessageStreamEvent{
		Type:           messageReadEventType,
		ConversationID: conversationID,
	})

	return nil
}

// ─── Cursors ──────────────────────────────────────────────────────────────────

func encodeMessageCursor(cursor *entity.MessageCursor) string {
	return encodeCursor(cursor)
}

func decodeMessageCursor(raw string) (*entity.MessageCursor, error) {
	return decodeCursor(raw, func(c *entity.MessageCursor) bool {
		return c.ID != "" && !c.CreatedAt.IsZero()
	})
}

// ─── Presenters ───────────────────────────────────────────────────────────────

func presentFriendRequest(r entity.FriendRequest) model.FriendRequestResponse {
	return model.FriendRequestResponse{
		ID:           r.ID,
		SenderID:     r.SenderID,
		SenderName:   r.SenderName,
		ReceiverID:   r.ReceiverID,
		ReceiverName: r.ReceiverName,
		Message:      r.Message,
		Status:       string(r.Status),
		CreatedAt:    r.CreatedAt,
	}
}

func (u *messageUseCase) ListFriends(ctx context.Context, userID string) ([]model.FriendResponse, error) {
	friends, err := u.messageRepository.ListFriends(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.FriendResponse, len(friends))
	for i, f := range friends {
		out[i] = model.FriendResponse{ID: f.ID, Name: f.Name, AvatarURL: f.AvatarURL}
	}
	return out, nil
}

func presentMessage(m entity.Message) model.MessageResponse {
	var location *model.MessageLocationResponse
	if m.Latitude != nil && m.Longitude != nil && strings.TrimSpace(m.LocationName) != "" {
		location = &model.MessageLocationResponse{
			Label:     m.LocationName,
			Latitude:  *m.Latitude,
			Longitude: *m.Longitude,
		}
	}

	return model.MessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		SenderName:     m.SenderName,
		Body:           m.Body,
		ImageURL:       m.ImageURL,
		Location:       location,
		CreatedAt:      m.CreatedAt,
	}
}

func presentConversation(c entity.Conversation) model.ConversationResponse {
	participants := make([]model.ConversationParticipant, len(c.Participants))
	for i, p := range c.Participants {
		participants[i] = model.ConversationParticipant{
			UserID:    p.UserID,
			Name:      p.Name,
			AvatarURL: p.AvatarURL,
		}
	}

	var lastMsg *model.MessageResponse
	if c.LastMessage != nil {
		m := presentMessage(*c.LastMessage)
		lastMsg = &m
	}

	return model.ConversationResponse{
		ID:            c.ID,
		Kind:          string(c.Kind),
		Name:          c.Name,
		Participants:  participants,
		LastMessage:   lastMsg,
		UnreadCount:   c.UnreadCount,
		LastMessageAt: c.LastMessageAt,
	}
}
