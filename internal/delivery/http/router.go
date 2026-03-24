package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/entity"
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
	notificationUseCase usecase.NotificationUseCase,
	internalNotificationUseCase usecase.InternalNotificationUseCase,
	internalPostUseCase usecase.InternalPostUseCase,
	adminUseCase usecase.AdminUseCase,
	internalWeatherAlertUseCase usecase.InternalWeatherAlertUseCase,
	internalAPIKey string,
	env string,
	notificationStreamHub *NotificationStreamHub,
	trustUseCase usecase.TrustUseCase,
	db *database.Postgres,
) {
	NewHealthHandler(db).Register(app)
	NewAuthHandler(authUseCase).Register(app.Group("/auth"))
	profileHandler := NewProfileHandler(profileUseCase, avatarImageStore, postImageStore, messageImageStore)
	profileHandler.Register(app.Group("/profile", AuthRequired(authUseCase)))
	profileHandler.RegisterPublic(app.Group("/users", AuthRequired(authUseCase)))
	NewPostHandler(postUseCase, authUseCase, postStreamHub, postImageStore).Register(app.Group("/posts"))
	NewMessageHandler(authUseCase, messageUseCase, messageStreamHub, messageImageStore).Register(app.Group("/messages"))
	NewNotificationHandler(authUseCase, notificationUseCase, notificationStreamHub).Register(app.Group("/notifications"))
	NewInternalNotificationHandler(internalNotificationUseCase).Register(app.Group("/internal/notifications"))
	NewInternalPostHandler(internalPostUseCase, internalAPIKey, env).Register(app.Group("/internal/posts"))
	NewAdminHandler(adminUseCase).Register(app.Group("/admin", AuthRequired(authUseCase), RequireAnyRole(entity.UserRoleAdmin, entity.UserRoleModerator)))
	NewInternalWeatherAlertHandler(internalWeatherAlertUseCase, internalAPIKey, env).Register(app.Group("/internal/weather-alerts"))
	NewTrustHandler(trustUseCase).Register(app.Group("", AuthRequired(authUseCase)))
}
