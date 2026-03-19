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

// NearbySkillUser is returned by the skill-match query: a nearby user whose
// skills overlap with a post's tags, together with their quiet-hours window so
// the notification layer can skip the push (but still store the in-app notification).
type NearbySkillUser struct {
	UserID          string
	QuietHoursStart *string // "HH:MM", nil if not set
	QuietHoursEnd   *string // "HH:MM", nil if not set
}
