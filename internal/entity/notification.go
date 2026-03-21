package entity

import "time"

type Notification struct {
	ID        string
	UserID    string
	Type      string
	Title     string
	Body      string
	EntityID  string
	IsRead    bool
	CreatedAt time.Time
}

