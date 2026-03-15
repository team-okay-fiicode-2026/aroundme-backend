package entity

import "time"

type User struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
}

type AuthSession struct {
	ID                    string
	UserID                string
	AccessToken           string
	AccessTokenHash       string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenHash      string
	RefreshTokenExpiresAt time.Time
}

type AuthResult struct {
	Session AuthSession
	User    User
}

type SocialProvider string

const (
	ProviderGoogle   SocialProvider = "google"
	ProviderApple    SocialProvider = "apple"
	ProviderFacebook SocialProvider = "facebook"
)
