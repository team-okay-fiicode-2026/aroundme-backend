package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

func Register(
	app *fiber.App,
	authUseCase usecase.AuthUseCase,
	profileUseCase usecase.ProfileUseCase,
	db *database.Postgres,
) {
	NewHealthHandler(db).Register(app)
	NewAuthHandler(authUseCase).Register(app.Group("/auth"))
	NewProfileHandler(profileUseCase).Register(app.Group("/profile", AuthRequired(authUseCase)))
}
