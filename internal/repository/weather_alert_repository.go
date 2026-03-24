package repository

import (
	"context"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type WeatherAlertRepository interface {
	UpsertAlert(ctx context.Context, input entity.WeatherAlertUpsertInput, observedAt time.Time) (entity.WeatherAlertUpsertResult, error)
	ResolveInactiveAlerts(ctx context.Context, provider string, activeExternalIDs []string, observedAt time.Time) ([]entity.Post, error)
}
