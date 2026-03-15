package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type AuthHandler struct {
	authUseCase usecase.AuthUseCase
}

type authResponse struct {
	AccessToken           string           `json:"accessToken"`
	AccessTokenExpiresAt  string           `json:"accessTokenExpiresAt"`
	RefreshToken          string           `json:"refreshToken"`
	RefreshTokenExpiresAt string           `json:"refreshTokenExpiresAt"`
	User                  authUserResponse `json:"user"`
}

type authUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type signUpRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type socialSignInRequest struct {
	Provider       string `json:"provider"`
	ProviderUserID string `json:"providerUserId"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	AvatarURL      string `json:"avatarUrl"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type signOutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func NewAuthHandler(authUseCase usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUseCase: authUseCase}
}

func (h *AuthHandler) Register(app fiber.Router) {
	tooManyRequests := func(c *fiber.Ctx) error {
		return c.Status(stdhttp.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many requests, please try again later",
		})
	}

	// Strict limit for endpoints that mutate credentials or create sessions.
	strictLimiter := limiter.New(limiter.Config{
		Max:          10,
		Expiration:   1 * time.Minute,
		LimitReached: tooManyRequests,
	})

	// More lenient limit for token refresh (clients refresh every 15 min).
	refreshLimiter := limiter.New(limiter.Config{
		Max:          30,
		Expiration:   1 * time.Minute,
		LimitReached: tooManyRequests,
	})

	app.Post("/sign-up", strictLimiter, h.signUp)
	app.Post("/sign-in", strictLimiter, h.signIn)
	app.Post("/social", strictLimiter, h.socialSignIn)
	app.Post("/refresh", refreshLimiter, h.refresh)
	app.Post("/sign-out", h.signOut)
}

func (h *AuthHandler) signUp(c *fiber.Ctx) error {
	var request signUpRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.authUseCase.SignUp(ctx, model.SignUpInput{
		Name:     request.Name,
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return writeAuthError(c, err)
	}

	return c.Status(stdhttp.StatusCreated).JSON(presentAuthResult(result))
}

func (h *AuthHandler) signIn(c *fiber.Ctx) error {
	var request signInRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.authUseCase.SignIn(ctx, model.SignInInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return writeAuthError(c, err)
	}

	return c.JSON(presentAuthResult(result))
}

func (h *AuthHandler) socialSignIn(c *fiber.Ctx) error {
	var request socialSignInRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.authUseCase.SocialSignIn(ctx, model.SocialSignInInput{
		Provider:       request.Provider,
		ProviderUserID: request.ProviderUserID,
		Email:          request.Email,
		Name:           request.Name,
		AvatarURL:      request.AvatarURL,
	})
	if err != nil {
		return writeAuthError(c, err)
	}

	return c.JSON(presentAuthResult(result))
}

func (h *AuthHandler) refresh(c *fiber.Ctx) error {
	var request refreshRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.authUseCase.RefreshSession(ctx, model.RefreshSessionInput{
		RefreshToken: request.RefreshToken,
	})
	if err != nil {
		return writeAuthError(c, err)
	}

	return c.JSON(presentAuthResult(result))
}

func (h *AuthHandler) signOut(c *fiber.Ctx) error {
	var request signOutRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.authUseCase.SignOut(ctx, model.SignOutInput{
		RefreshToken: request.RefreshToken,
	}); err != nil {
		return writeAuthError(c, err)
	}

	return c.SendStatus(stdhttp.StatusNoContent)
}

func presentAuthResult(result model.AuthResult) authResponse {
	return authResponse{
		AccessToken:           result.AccessToken,
		AccessTokenExpiresAt:  result.AccessTokenExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken:          result.RefreshToken,
		RefreshTokenExpiresAt: result.RefreshTokenExpiresAt.UTC().Format(time.RFC3339),
		User: authUserResponse{
			ID:        result.User.ID,
			Email:     result.User.Email,
			Name:      result.User.Name,
			AvatarURL: result.User.AvatarURL,
		},
	}
}

func writeAuthError(c *fiber.Ctx, err error) error {
	var validationError model.ValidationError
	switch {
	case errors.As(err, &validationError):
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": validationError.Error(),
		})
	case errors.Is(err, model.ErrEmailAlreadyExists):
		return c.Status(stdhttp.StatusConflict).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrInvalidCredentials):
		return c.Status(stdhttp.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrInvalidRefreshToken):
		return c.Status(stdhttp.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrUnsupportedProvider):
		return c.Status(stdhttp.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrSocialAuthDisabled):
		return c.Status(stdhttp.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrSessionRequired):
		return c.Status(stdhttp.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	default:
		return c.Status(stdhttp.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}
}
