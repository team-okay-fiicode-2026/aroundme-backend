package repository

import (
	"context"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type NotificationRepository interface {
	Create(ctx context.Context, n entity.Notification) (entity.Notification, error)
	List(ctx context.Context, userID string, limit int) ([]entity.Notification, int, error)
	MarkRead(ctx context.Context, notificationID, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	UpsertPushToken(ctx context.Context, userID, token string) error
	GetPushTokens(ctx context.Context, userID string) ([]string, error)
	// ListNearbyUserIDs returns IDs of users who have a location set within radiusKm of the given point,
	// excluding the user with excludeUserID.
	ListNearbyUserIDs(ctx context.Context, latitude, longitude, radiusKm float64, excludeUserID string) ([]string, error)
	// ListNearbyUsersForSkillMatch returns nearby users whose skills or available items overlap with the provided tags.
	// Each user's own distance_limit_km is used as the notification radius, so the radius is per-user.
	// Quiet-hours fields are included so the caller can suppress the push without a second query.
	ListNearbyUsersForSkillMatch(ctx context.Context, latitude, longitude float64, tags []string, excludeUserID string) ([]entity.NearbySkillUser, error)
	// GetPostInfo returns the author user ID and title for the given post.
	GetPostInfo(ctx context.Context, postID string) (authorID, title string, err error)
}
