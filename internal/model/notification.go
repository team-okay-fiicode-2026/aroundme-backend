package model

import "time"

type NotificationResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	EntityID  string    `json:"entityId,omitempty"`
	IsRead    bool      `json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
}

type ListNotificationsResult struct {
	Items       []NotificationResponse `json:"items"`
	UnreadCount int                    `json:"unreadCount"`
}

type NotificationStreamEvent struct {
	Type         string                `json:"type"`
	Notification *NotificationResponse `json:"notification,omitempty"`
	UnreadCount  int                   `json:"unreadCount"`
}
