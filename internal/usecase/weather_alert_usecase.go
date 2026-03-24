package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type InternalWeatherAlertUseCase interface {
	SyncWeatherAlerts(ctx context.Context, input entity.WeatherAlertSyncInput) (entity.WeatherAlertSyncResult, error)
}

type noopWeatherAlertNotifier struct{}

func (noopWeatherAlertNotifier) EnqueueWeatherAlertNotifications(context.Context, entity.Post, float64) error {
	return nil
}

type weatherAlertUseCase struct {
	repo      repository.WeatherAlertRepository
	publisher PostEventPublisher
	notifier  WeatherAlertNotifier
}

func NewWeatherAlertUseCase(
	repo repository.WeatherAlertRepository,
	publisher PostEventPublisher,
	notifier WeatherAlertNotifier,
) InternalWeatherAlertUseCase {
	if publisher == nil {
		publisher = noopPostEventPublisher{}
	}
	if notifier == nil {
		notifier = noopWeatherAlertNotifier{}
	}
	return &weatherAlertUseCase{
		repo:      repo,
		publisher: publisher,
		notifier:  notifier,
	}
}

func (u *weatherAlertUseCase) SyncWeatherAlerts(
	ctx context.Context,
	input entity.WeatherAlertSyncInput,
) (entity.WeatherAlertSyncResult, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		return entity.WeatherAlertSyncResult{}, model.ValidationError{Message: "provider is required"}
	}

	observedAt := input.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	deduped, orderedIDs, err := dedupeWeatherAlerts(provider, input.Alerts)
	if err != nil {
		return entity.WeatherAlertSyncResult{}, err
	}

	result := entity.WeatherAlertSyncResult{}
	activeExternalIDs := make([]string, 0, len(orderedIDs))

	for _, externalID := range orderedIDs {
		alert := deduped[externalID]
		activeExternalIDs = append(activeExternalIDs, alert.ExternalID)

		upserted, err := u.repo.UpsertAlert(ctx, alert, observedAt)
		if err != nil {
			return entity.WeatherAlertSyncResult{}, fmt.Errorf("upsert weather alert %s: %w", alert.ExternalID, err)
		}

		if upserted.Created {
			result.CreatedCount++
			u.publisher.Publish(model.PostStreamEvent{Type: postCreatedEventType, PostID: upserted.Post.ID})
		} else {
			result.UpdatedCount++
			u.publisher.Publish(model.PostStreamEvent{Type: postUpdatedEventType, PostID: upserted.Post.ID})
		}

		if shouldNotifyWeatherAlert(upserted) {
			if err := u.notifier.EnqueueWeatherAlertNotifications(ctx, upserted.Post, upserted.Alert.RadiusKm); err != nil {
				return entity.WeatherAlertSyncResult{}, fmt.Errorf("notify weather alert %s: %w", alert.ExternalID, err)
			}
			result.NotifiedCount++
		}
	}

	resolvedPosts, err := u.repo.ResolveInactiveAlerts(ctx, provider, activeExternalIDs, observedAt)
	if err != nil {
		return entity.WeatherAlertSyncResult{}, fmt.Errorf("resolve inactive weather alerts: %w", err)
	}
	for _, post := range resolvedPosts {
		result.ResolvedCount++
		u.publisher.Publish(model.PostStreamEvent{Type: postUpdatedEventType, PostID: post.ID})
	}

	return result, nil
}

func dedupeWeatherAlerts(
	provider string,
	alerts []entity.WeatherAlertUpsertInput,
) (map[string]entity.WeatherAlertUpsertInput, []string, error) {
	items := make(map[string]entity.WeatherAlertUpsertInput, len(alerts))
	order := make([]string, 0, len(alerts))

	for _, raw := range alerts {
		alert, err := normalizeWeatherAlertInput(provider, raw)
		if err != nil {
			return nil, nil, err
		}

		existing, exists := items[alert.ExternalID]
		if !exists {
			items[alert.ExternalID] = alert
			order = append(order, alert.ExternalID)
			continue
		}

		if weatherSeverityRank(alert.Severity) >= weatherSeverityRank(existing.Severity) {
			items[alert.ExternalID] = alert
		}
	}

	sort.Strings(order)
	return items, order, nil
}

func normalizeWeatherAlertInput(
	provider string,
	input entity.WeatherAlertUpsertInput,
) (entity.WeatherAlertUpsertInput, error) {
	input.Provider = provider
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	if input.ExternalID == "" {
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert externalId is required"}
	}

	input.Event = strings.TrimSpace(input.Event)
	if input.Event == "" {
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert event is required"}
	}

	if input.CenterLatitude < -90 || input.CenterLatitude > 90 {
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert center latitude must be between -90 and 90"}
	}
	if input.CenterLongitude < -180 || input.CenterLongitude > 180 {
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert center longitude must be between -180 and 180"}
	}
	if input.RadiusKm < 0 {
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert radiusKm must be positive"}
	}

	switch input.Severity {
	case entity.WeatherAlertSeverityCritical,
		entity.WeatherAlertSeverityHigh,
		entity.WeatherAlertSeverityModerate,
		entity.WeatherAlertSeverityLow:
	default:
		return entity.WeatherAlertUpsertInput{}, model.ValidationError{Message: "alert severity must be one of critical, high, moderate, or low"}
	}

	input.Headline = strings.TrimSpace(input.Headline)
	input.Description = strings.TrimSpace(input.Description)
	input.Instruction = strings.TrimSpace(input.Instruction)
	input.AreaDesc = strings.TrimSpace(input.AreaDesc)
	input.ProviderSeverity = strings.TrimSpace(input.ProviderSeverity)
	input.Urgency = strings.TrimSpace(input.Urgency)
	input.Certainty = strings.TrimSpace(input.Certainty)
	input.Source = strings.TrimSpace(input.Source)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	return input, nil
}

func shouldNotifyWeatherAlert(result entity.WeatherAlertUpsertResult) bool {
	if !isUrgentWeatherSeverity(result.Alert.Severity) {
		return false
	}
	if result.Created || !result.WasActive {
		return true
	}
	return weatherSeverityRank(result.Alert.Severity) > weatherSeverityRank(result.PreviousSeverity)
}

func isUrgentWeatherSeverity(severity entity.WeatherAlertSeverity) bool {
	return severity == entity.WeatherAlertSeverityCritical || severity == entity.WeatherAlertSeverityHigh
}

func weatherSeverityRank(severity entity.WeatherAlertSeverity) int {
	switch severity {
	case entity.WeatherAlertSeverityCritical:
		return 4
	case entity.WeatherAlertSeverityHigh:
		return 3
	case entity.WeatherAlertSeverityModerate:
		return 2
	case entity.WeatherAlertSeverityLow:
		return 1
	default:
		return 0
	}
}
