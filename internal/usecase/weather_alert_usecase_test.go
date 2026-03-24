package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
)

type fakeWeatherAlertRepository struct {
	resolveActiveIDs  []string
	resolveObservedAt time.Time
	resolveProvider   string
	resolveResult     []entity.Post
	upsertCalls       []entity.WeatherAlertUpsertInput
	upsertObservedAt  []time.Time
	upsertResults     map[string]entity.WeatherAlertUpsertResult
}

func (r *fakeWeatherAlertRepository) UpsertAlert(ctx context.Context, input entity.WeatherAlertUpsertInput, observedAt time.Time) (entity.WeatherAlertUpsertResult, error) {
	r.upsertCalls = append(r.upsertCalls, input)
	r.upsertObservedAt = append(r.upsertObservedAt, observedAt)
	if result, ok := r.upsertResults[input.ExternalID]; ok {
		return result, nil
	}
	return entity.WeatherAlertUpsertResult{
		Alert: entity.WeatherAlert{
			Provider:   input.Provider,
			ExternalID: input.ExternalID,
			Severity:   input.Severity,
			RadiusKm:   input.RadiusKm,
		},
		Post: entity.Post{ID: input.ExternalID},
	}, nil
}

func (r *fakeWeatherAlertRepository) ResolveInactiveAlerts(ctx context.Context, provider string, activeExternalIDs []string, observedAt time.Time) ([]entity.Post, error) {
	r.resolveProvider = provider
	r.resolveActiveIDs = append([]string(nil), activeExternalIDs...)
	r.resolveObservedAt = observedAt
	return r.resolveResult, nil
}

type fakeWeatherAlertNotifier struct {
	calls []struct {
		postID   string
		radiusKm float64
	}
}

func (n *fakeWeatherAlertNotifier) EnqueueWeatherAlertNotifications(ctx context.Context, post entity.Post, radiusKm float64) error {
	n.calls = append(n.calls, struct {
		postID   string
		radiusKm float64
	}{postID: post.ID, radiusKm: radiusKm})
	return nil
}

type fakePostPublisher struct {
	events []model.PostStreamEvent
}

func (p *fakePostPublisher) Publish(event model.PostStreamEvent) {
	p.events = append(p.events, event)
}

func TestWeatherAlertUseCaseSyncWeatherAlertsIngestsAndNotifiesUrgentAlerts(t *testing.T) {
	t.Parallel()

	repo := &fakeWeatherAlertRepository{
		upsertResults: map[string]entity.WeatherAlertUpsertResult{
			"alert-critical": {
				Alert:   entity.WeatherAlert{ExternalID: "alert-critical", Severity: entity.WeatherAlertSeverityCritical, RadiusKm: 12},
				Post:    entity.Post{ID: "post-critical"},
				Created: true,
			},
			"alert-moderate": {
				Alert:   entity.WeatherAlert{ExternalID: "alert-moderate", Severity: entity.WeatherAlertSeverityModerate, RadiusKm: 8},
				Post:    entity.Post{ID: "post-moderate"},
				Created: true,
			},
		},
	}
	notifier := &fakeWeatherAlertNotifier{}
	publisher := &fakePostPublisher{}
	useCase := NewWeatherAlertUseCase(repo, publisher, notifier)

	observedAt := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	result, err := useCase.SyncWeatherAlerts(context.Background(), entity.WeatherAlertSyncInput{
		Provider:   "nws",
		ObservedAt: observedAt,
		Alerts: []entity.WeatherAlertUpsertInput{
			{
				ExternalID:      "alert-critical",
				Event:           "Tornado Warning",
				Severity:        entity.WeatherAlertSeverityCritical,
				CenterLatitude:  38.9,
				CenterLongitude: -77.0,
				RadiusKm:        12,
			},
			{
				ExternalID:      "alert-moderate",
				Event:           "Flood Advisory",
				Severity:        entity.WeatherAlertSeverityModerate,
				CenterLatitude:  38.8,
				CenterLongitude: -77.1,
				RadiusKm:        8,
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncWeatherAlerts() error = %v", err)
	}

	if result.CreatedCount != 2 || result.UpdatedCount != 0 || result.ResolvedCount != 0 || result.NotifiedCount != 1 {
		t.Fatalf("unexpected result = %+v", result)
	}
	if len(repo.upsertCalls) != 2 {
		t.Fatalf("upsert calls = %d, want 2", len(repo.upsertCalls))
	}
	if len(notifier.calls) != 1 || notifier.calls[0].postID != "post-critical" {
		t.Fatalf("notifier calls = %+v, want only critical alert", notifier.calls)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("publisher events = %d, want 2", len(publisher.events))
	}
	for _, event := range publisher.events {
		if event.Type != postCreatedEventType {
			t.Fatalf("event type = %q, want %q", event.Type, postCreatedEventType)
		}
	}
}

func TestWeatherAlertUseCaseSyncWeatherAlertsDedupesByExternalID(t *testing.T) {
	t.Parallel()

	repo := &fakeWeatherAlertRepository{
		upsertResults: map[string]entity.WeatherAlertUpsertResult{
			"alert-1": {
				Alert:   entity.WeatherAlert{ExternalID: "alert-1", Severity: entity.WeatherAlertSeverityHigh, RadiusKm: 5},
				Post:    entity.Post{ID: "post-1"},
				Created: true,
			},
		},
	}
	notifier := &fakeWeatherAlertNotifier{}
	useCase := NewWeatherAlertUseCase(repo, nil, notifier)

	_, err := useCase.SyncWeatherAlerts(context.Background(), entity.WeatherAlertSyncInput{
		Provider: "nws",
		Alerts: []entity.WeatherAlertUpsertInput{
			{
				ExternalID:      "alert-1",
				Event:           "Special Weather Statement",
				Severity:        entity.WeatherAlertSeverityLow,
				CenterLatitude:  40,
				CenterLongitude: -74,
				RadiusKm:        3,
			},
			{
				ExternalID:      "alert-1",
				Event:           "Severe Thunderstorm Warning",
				Severity:        entity.WeatherAlertSeverityHigh,
				CenterLatitude:  40,
				CenterLongitude: -74,
				RadiusKm:        5,
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncWeatherAlerts() error = %v", err)
	}

	if len(repo.upsertCalls) != 1 {
		t.Fatalf("upsert calls = %d, want 1", len(repo.upsertCalls))
	}
	if repo.upsertCalls[0].Severity != entity.WeatherAlertSeverityHigh {
		t.Fatalf("deduped severity = %q, want %q", repo.upsertCalls[0].Severity, entity.WeatherAlertSeverityHigh)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notifier calls = %d, want 1", len(notifier.calls))
	}
}

func TestWeatherAlertUseCaseSyncWeatherAlertsExpiresMissingAlerts(t *testing.T) {
	t.Parallel()

	repo := &fakeWeatherAlertRepository{
		upsertResults: map[string]entity.WeatherAlertUpsertResult{},
		resolveResult: []entity.Post{
			{ID: "resolved-post"},
		},
	}
	publisher := &fakePostPublisher{}
	useCase := NewWeatherAlertUseCase(repo, publisher, nil)

	result, err := useCase.SyncWeatherAlerts(context.Background(), entity.WeatherAlertSyncInput{
		Provider: "nws",
		Alerts:   nil,
	})
	if err != nil {
		t.Fatalf("SyncWeatherAlerts() error = %v", err)
	}

	if result.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", result.ResolvedCount)
	}
	if len(repo.resolveActiveIDs) != 0 {
		t.Fatalf("resolve active ids = %v, want empty", repo.resolveActiveIDs)
	}
	if len(publisher.events) != 1 || publisher.events[0].Type != postUpdatedEventType {
		t.Fatalf("publisher events = %+v, want single %q", publisher.events, postUpdatedEventType)
	}
}
