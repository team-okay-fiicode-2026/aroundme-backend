package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

var supportedSocialProviders = map[entity.SocialProvider]struct{}{
	entity.ProviderGoogle:   {},
	entity.ProviderApple:    {},
	entity.ProviderFacebook: {},
}

type AuthUseCase interface {
	SignUp(ctx context.Context, input model.SignUpInput) (model.AuthResult, error)
	SignIn(ctx context.Context, input model.SignInInput) (model.AuthResult, error)
	SocialSignIn(ctx context.Context, input model.SocialSignInInput) (model.AuthResult, error)
	RefreshSession(ctx context.Context, input model.RefreshSessionInput) (model.AuthResult, error)
	SignOut(ctx context.Context, input model.SignOutInput) error
	ValidateAccessToken(ctx context.Context, accessToken string) (entity.User, error)
}

type AuthConfig struct {
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	AllowDevSocialAuth bool
}

type authUseCase struct {
	authRepository     repository.AuthRepository
	accessTokenTTL     time.Duration
	refreshTokenTTL    time.Duration
	allowDevSocialAuth bool
}

func NewAuthUseCase(authRepository repository.AuthRepository, cfg AuthConfig) AuthUseCase {
	return &authUseCase{
		authRepository:     authRepository,
		accessTokenTTL:     cfg.AccessTokenTTL,
		refreshTokenTTL:    cfg.RefreshTokenTTL,
		allowDevSocialAuth: cfg.AllowDevSocialAuth,
	}
}

func (u *authUseCase) SignUp(ctx context.Context, input model.SignUpInput) (model.AuthResult, error) {
	name, err := normalizeName(input.Name)
	if err != nil {
		return model.AuthResult{}, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return model.AuthResult{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return model.AuthResult{}, err
	}

	session, err := newSession(u.accessTokenTTL, u.refreshTokenTTL)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("generate session: %w", err)
	}

	result, err := u.authRepository.CreatePasswordUser(ctx, entity.User{Email: email, Name: name}, input.Password, session)
	switch {
	case errors.Is(err, repository.ErrUniqueEmail):
		return model.AuthResult{}, model.ErrEmailAlreadyExists
	case err != nil:
		return model.AuthResult{}, fmt.Errorf("create password user: %w", err)
	}

	return toAuthResult(result), nil
}

func (u *authUseCase) SignIn(ctx context.Context, input model.SignInInput) (model.AuthResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return model.AuthResult{}, err
	}

	if strings.TrimSpace(input.Password) == "" {
		return model.AuthResult{}, model.ValidationError{Message: "password is required"}
	}

	session, err := newSession(u.accessTokenTTL, u.refreshTokenTTL)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("generate session: %w", err)
	}

	result, err := u.authRepository.AuthenticateByPassword(ctx, email, input.Password, session)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return model.AuthResult{}, model.ErrInvalidCredentials
	case err != nil:
		return model.AuthResult{}, fmt.Errorf("authenticate by password: %w", err)
	}

	return toAuthResult(result), nil
}

func (u *authUseCase) SocialSignIn(ctx context.Context, input model.SocialSignInInput) (model.AuthResult, error) {
	if !u.allowDevSocialAuth {
		return model.AuthResult{}, model.ErrSocialAuthDisabled
	}

	provider, err := normalizeProvider(input.Provider)
	if err != nil {
		return model.AuthResult{}, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return model.AuthResult{}, err
	}

	name, err := normalizeName(input.Name)
	if err != nil {
		return model.AuthResult{}, err
	}

	providerUserID := strings.TrimSpace(input.ProviderUserID)
	if providerUserID == "" {
		return model.AuthResult{}, model.ValidationError{Message: "provider user id is required"}
	}

	session, err := newSession(u.accessTokenTTL, u.refreshTokenTTL)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("generate session: %w", err)
	}

	user := entity.User{
		Email:     email,
		Name:      name,
		AvatarURL: strings.TrimSpace(input.AvatarURL),
	}

	result, err := u.authRepository.AuthenticateBySocial(ctx, provider, providerUserID, user, session)
	switch {
	case errors.Is(err, repository.ErrUniqueEmail):
		return model.AuthResult{}, model.ErrEmailAlreadyExists
	case err != nil:
		return model.AuthResult{}, fmt.Errorf("authenticate by social provider: %w", err)
	}

	return toAuthResult(result), nil
}

func (u *authUseCase) RefreshSession(ctx context.Context, input model.RefreshSessionInput) (model.AuthResult, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return model.AuthResult{}, model.ErrSessionRequired
	}

	session, err := newSession(u.accessTokenTTL, u.refreshTokenTTL)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("generate session: %w", err)
	}

	result, err := u.authRepository.RefreshSession(ctx, hashToken(refreshToken), session)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return model.AuthResult{}, model.ErrInvalidRefreshToken
	case err != nil:
		return model.AuthResult{}, fmt.Errorf("refresh session: %w", err)
	}

	return toAuthResult(result), nil
}

func (u *authUseCase) ValidateAccessToken(ctx context.Context, accessToken string) (entity.User, error) {
	if strings.TrimSpace(accessToken) == "" {
		return entity.User{}, repository.ErrNotFound
	}

	user, err := u.authRepository.ValidateAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return entity.User{}, err
	}

	return user, nil
}

func (u *authUseCase) SignOut(ctx context.Context, input model.SignOutInput) error {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return model.ErrSessionRequired
	}

	if err := u.authRepository.RevokeSession(ctx, hashToken(refreshToken)); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func toAuthResult(result entity.AuthResult) model.AuthResult {
	return model.AuthResult{
		AccessToken:           result.Session.AccessToken,
		AccessTokenExpiresAt:  result.Session.AccessTokenExpiresAt,
		RefreshToken:          result.Session.RefreshToken,
		RefreshTokenExpiresAt: result.Session.RefreshTokenExpiresAt,
		User: model.AuthUser{
			ID:        result.User.ID,
			Email:     result.User.Email,
			Name:      result.User.Name,
			AvatarURL: result.User.AvatarURL,
		},
	}
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", model.ValidationError{Message: "email is required"}
	}

	address, err := mail.ParseAddress(normalized)
	if err != nil || !strings.EqualFold(address.Address, normalized) {
		return "", model.ValidationError{Message: "email is invalid"}
	}

	return normalized, nil
}

func normalizeName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", model.ValidationError{Message: "name is required"}
	}

	if len(normalized) > 120 {
		return "", model.ValidationError{Message: "name is too long"}
	}

	return normalized, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return model.ValidationError{Message: "password must be at least 8 characters"}
	}

	// bcrypt silently truncates at 72 bytes; reject longer passwords to avoid
	// two different passwords hashing identically.
	if len(password) > 72 {
		return model.ValidationError{Message: "password must be no longer than 72 characters"}
	}

	return nil
}

func normalizeProvider(provider string) (entity.SocialProvider, error) {
	normalized := entity.SocialProvider(strings.ToLower(strings.TrimSpace(provider)))
	if _, ok := supportedSocialProviders[normalized]; !ok {
		return "", model.ErrUnsupportedProvider
	}

	return normalized, nil
}

func newSession(accessTTL, refreshTTL time.Duration) (entity.AuthSession, error) {
	accessToken, err := generateToken()
	if err != nil {
		return entity.AuthSession{}, err
	}

	refreshToken, err := generateToken()
	if err != nil {
		return entity.AuthSession{}, err
	}

	now := time.Now().UTC()

	return entity.AuthSession{
		AccessToken:           accessToken,
		AccessTokenHash:       hashToken(accessToken),
		AccessTokenExpiresAt:  now.Add(accessTTL),
		RefreshToken:          refreshToken,
		RefreshTokenHash:      hashToken(refreshToken),
		RefreshTokenExpiresAt: now.Add(refreshTTL),
	}, nil
}

func generateToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
