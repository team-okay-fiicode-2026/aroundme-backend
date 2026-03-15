package repository

import (
	"context"
	"errors"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

var (
	ErrNotFound      = errors.New("repository record not found")
	ErrTokenConflict = errors.New("repository token conflict")
	ErrUniqueEmail   = errors.New("repository unique email")
)

type AuthRepository interface {
	CreatePasswordUser(ctx context.Context, user entity.User, password string, session entity.AuthSession) (entity.AuthResult, error)
	AuthenticateByPassword(ctx context.Context, email, password string, session entity.AuthSession) (entity.AuthResult, error)
	AuthenticateBySocial(
		ctx context.Context,
		provider entity.SocialProvider,
		providerUserID string,
		user entity.User,
		session entity.AuthSession,
	) (entity.AuthResult, error)
	RefreshSession(ctx context.Context, refreshTokenHash string, replacement entity.AuthSession) (entity.AuthResult, error)
	RevokeSession(ctx context.Context, refreshTokenHash string) error
	ValidateAccessToken(ctx context.Context, accessTokenHash string) (entity.User, error)
}
