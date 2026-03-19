package repository

import (
	"context"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type MessageRepository interface {
	// Friend requests
	SendFriendRequest(ctx context.Context, senderID, receiverID, message string) (entity.FriendRequest, error)
	ListPendingFriendRequests(ctx context.Context, userID string) ([]entity.FriendRequest, error)
	RespondFriendRequest(ctx context.Context, requestID, userID string, accept bool) (entity.FriendRequest, error)

	// Conversations
	GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID string) (entity.Conversation, error)
	CreateGroupConversation(ctx context.Context, creatorID, name string, participantIDs []string) (entity.Conversation, error)
	GetConversation(ctx context.Context, conversationID, userID string) (entity.Conversation, error)
	ListConversations(ctx context.Context, userID string) ([]entity.Conversation, error)
	IsParticipant(ctx context.Context, conversationID, userID string) (bool, error)

	// Messages
	SendMessage(ctx context.Context, conversationID, senderID, body, imageURL, locationName string, latitude, longitude *float64) (entity.Message, error)
	ListMessages(ctx context.Context, conversationID string, cursor *entity.MessageCursor, limit int) ([]entity.Message, *entity.MessageCursor, error)
	MarkRead(ctx context.Context, conversationID, userID string) error

	// Friends
	ListFriends(ctx context.Context, userID string) ([]entity.Friend, error)
}
