package http

import (
	"context"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

const userContextKey = "auth.user"

func AuthRequired(authUseCase usecase.AuthUseCase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return c.Status(stdhttp.StatusUnauthorized).JSON(fiber.Map{
				"error": "authorization header required",
			})
		}

		token := strings.TrimPrefix(header, "Bearer ")

		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		user, err := authUseCase.ValidateAccessToken(ctx, token)
		if err != nil {
			return c.Status(stdhttp.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired access token",
			})
		}

		c.Locals(userContextKey, user)
		return c.Next()
	}
}

func GetAuthUser(c *fiber.Ctx) entity.User {
	user, _ := c.Locals(userContextKey).(entity.User)
	return user
}
