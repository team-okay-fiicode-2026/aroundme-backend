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
	postUseCase usecase.PostUseCase,
	postStreamHub *PostStreamHub,
	postImageStore PostImageStore,
	messageUseCase usecase.MessageUseCase,
	messageStreamHub *MessageStreamHub,
	messageImageStore MessageImageStore,
	avatarImageStore AvatarImageStore,
	db *database.Postgres,
) {
	NewHealthHandler(db).Register(app)
	NewAuthHandler(authUseCase).Register(app.Group("/auth"))
	profileHandler := NewProfileHandler(profileUseCase, avatarImageStore, postImageStore, messageImageStore)
	profileHandler.Register(app.Group("/profile", AuthRequired(authUseCase)))
	profileHandler.RegisterPublic(app.Group("/users", AuthRequired(authUseCase)))
	NewPostHandler(postUseCase, authUseCase, postStreamHub, postImageStore).Register(app.Group("/posts"))
	NewMessageHandler(authUseCase, messageUseCase, messageStreamHub, messageImageStore).Register(app.Group("/messages"))
}
