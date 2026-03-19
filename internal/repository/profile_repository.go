package repository

import (
	"context"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type ProfileRepository interface {
	GetProfile(ctx context.Context, userID string) (entity.Profile, error)
	UpdateProfile(ctx context.Context, profile entity.Profile) (entity.Profile, error)
	UpdateAvatarURL(ctx context.Context, userID, avatarURL string) error
	SetSkills(ctx context.Context, userID string, tags []string) ([]string, error)
	CreateItem(ctx context.Context, item entity.ProfileItem) (entity.ProfileItem, error)
	UpdateItem(ctx context.Context, item entity.ProfileItem) (entity.ProfileItem, error)
	DeleteItem(ctx context.Context, userID, itemID string) error
	GetAccountMediaPaths(ctx context.Context, userID string) (entity.AccountMediaPaths, error)
	DeleteAccount(ctx context.Context, userID string) error
}
