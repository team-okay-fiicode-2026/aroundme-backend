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
	GetQuietHours(ctx context.Context, userID string) (start, end *string, err error)
	// ListNearbyUserIDs returns IDs of users who have a location set within radiusKm of the given point,
	// excluding the user with excludeUserID.
	ListNearbyUserIDs(ctx context.Context, latitude, longitude, radiusKm float64, excludeUserID string) ([]string, error)
	// GetPostInfo returns the author user ID and title for the given post.
	GetPostInfo(ctx context.Context, postID string) (authorID, title string, err error)
}
