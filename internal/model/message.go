package model

import (
	"errors"
	"time"
)

var (
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrFriendRequestNotFound = errors.New("friend request not found")
	ErrAlreadyConnected      = errors.New("friend request already exists")
	ErrCannotSelfRequest     = errors.New("cannot send friend request to yourself")
)

// Friend requests

type SendFriendRequestInput struct {
	ReceiverID string
}

type FriendRequestResponse struct {
	ID           string    `json:"id"`
	SenderID     string    `json:"senderId"`
	SenderName   string    `json:"senderName"`
	ReceiverID   string    `json:"receiverId"`
	ReceiverName string    `json:"receiverName"`
	Message      string    `json:"message"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Conversations

type ConversationParticipant struct {
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type FriendResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type MessageLocationResponse struct {
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type MessageResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	SenderID       string    `json:"senderId"`
	SenderName     string    `json:"senderName"`
	Body           string    `json:"body"`
	ImageURL       string    `json:"imageUrl,omitempty"`
	Location       *MessageLocationResponse `json:"location,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ConversationResponse struct {
	ID            string                    `json:"id"`
	Kind          string                    `json:"kind"`
	Name          string                    `json:"name"`
	Participants  []ConversationParticipant `json:"participants"`
	LastMessage   *MessageResponse          `json:"lastMessage"`
	UnreadCount   int                       `json:"unreadCount"`
	LastMessageAt time.Time                 `json:"lastMessageAt"`
}

type CreateDirectConversationInput struct {
	ParticipantID string
}

type CreateGroupConversationInput struct {
	Name           string
	ParticipantIDs []string
}

type SendMessageInput struct {
	ConversationID string
	SenderID       string
	Body           string
	ImageURL       string
	LocationName   string
	Latitude       *float64
	Longitude      *float64
}

type ListMessagesInput struct {
	ConversationID string
	ViewerUserID   string
	Cursor         string
	Limit          int
}

type ListMessagesResult struct {
	Items      []MessageResponse `json:"items"`
	NextCursor string            `json:"nextCursor"`
}

// Stream events

type MessageStreamEvent struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId,omitempty"`
}
