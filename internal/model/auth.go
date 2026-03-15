package model

import (
	"errors"
	"time"
)

var (
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRefreshToken = errors.New("refresh token is invalid or expired")
	ErrUnsupportedProvider = errors.New("unsupported social provider")
	ErrSocialAuthDisabled  = errors.New("social auth is not enabled")
	ErrSessionRequired     = errors.New("session token is required")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type SignUpInput struct {
	Name     string
	Email    string
	Password string
}

type SignInInput struct {
	Email    string
	Password string
}

type SocialSignInInput struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

type RefreshSessionInput struct {
	RefreshToken string
}

type SignOutInput struct {
	RefreshToken string
}

type AuthUser struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
}

type AuthResult struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	User                  AuthUser
}
