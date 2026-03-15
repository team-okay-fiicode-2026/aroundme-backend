package main

import (
	"context"
	"log"
	"time"

	"github.com/aroundme/aroundme-backend/internal/app"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	application, err := app.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("bootstrap application: %v", err)
	}
	defer application.Close()

	log.Printf("aroundme-backend listening on :%s", application.Config.AppPort)
	if err := application.HTTP.Listen(":" + application.Config.AppPort); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
