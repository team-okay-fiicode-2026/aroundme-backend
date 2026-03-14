package graph

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/db"
)

func Register(app *fiber.App, database *db.Postgres) {
	schema := NewExecutableSchema(Config{Resolvers: &Resolver{DB: database}})
	server := handler.NewDefaultServer(schema)

	app.All("/graphql", adaptor.HTTPHandler(server))
}
