package entity

import "time"

type FriendRequestStatus string

const (
	FriendRequestPending  FriendRequestStatus = "pending"
	FriendRequestAccepted FriendRequestStatus = "accepted"
	FriendRequestRejected FriendRequestStatus = "rejected"
)

type FriendRequest struct {
	ID           string
	SenderID     string
	SenderName   string
	ReceiverID   string
	ReceiverName string
	Message      string
	Status       FriendRequestStatus
	CreatedAt    time.Time
}

type ConversationKind string

const (
	ConversationKindDirect ConversationKind = "direct"
	ConversationKindGroup  ConversationKind = "group"
)

type ConversationParticipant struct {
	UserID     string
	Name       string
	AvatarURL  string
	LastReadAt *time.Time
}

type Friend struct {
	ID        string
	Name      string
	AvatarURL string
}

type Message struct {
	ID             string
	ConversationID string
	SenderID       string
	SenderName     string
	Body           string
	ImageURL       string
	LocationName   string
	Latitude       *float64
	Longitude      *float64
	CreatedAt      time.Time
}

type Conversation struct {
	ID            string
	Kind          ConversationKind
	Name          string
	CreatedBy     string
	Participants  []ConversationParticipant
	LastMessage   *Message
	UnreadCount   int
	LastMessageAt time.Time
	CreatedAt     time.Time
}

type MessageCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}
