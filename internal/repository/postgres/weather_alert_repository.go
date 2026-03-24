package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

const (
	weatherAlertSystemEmail      = "alerts@system.aroundme.local"
	weatherAlertMaxLocationRunes = 160
	weatherAlertMaxTagCount      = 8
)

type WeatherAlertRepository struct {
	postgres *database.Postgres
}

func NewWeatherAlertRepository(postgres *database.Postgres) repository.WeatherAlertRepository {
	return &WeatherAlertRepository{postgres: postgres}
}

func (r *WeatherAlertRepository) UpsertAlert(
	ctx context.Context,
	input entity.WeatherAlertUpsertInput,
	observedAt time.Time,
) (entity.WeatherAlertUpsertResult, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.WeatherAlertUpsertResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	systemUserID, err := r.systemUserID(ctx, tx)
	if err != nil {
		return entity.WeatherAlertUpsertResult{}, err
	}

	postDraft := buildWeatherAlertPost(input)
	postUrgency := weatherAlertUrgency(input.Severity)

	var (
		alertID          string
		postID           string
		existingStatus   string
		previousSeverity string
	)

	err = tx.QueryRow(ctx, `
		SELECT id, COALESCE(post_id::text, ''), status, severity
		FROM weather_alerts
		WHERE provider = $1 AND external_id = $2
		FOR UPDATE
	`, input.Provider, input.ExternalID).Scan(&alertID, &postID, &existingStatus, &previousSeverity)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		postID, err = r.insertWeatherAlertPost(ctx, tx, systemUserID, postDraft, postUrgency)
		if err != nil {
			return entity.WeatherAlertUpsertResult{}, err
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO weather_alerts (
				provider, external_id, status, event, headline, description, instruction, area_desc,
				severity, provider_severity, urgency, certainty, source, source_url,
				starts_at, ends_at, expires_at, geometry, center_latitude, center_longitude, radius_km,
				post_id, last_seen_at, resolved_at, updated_at
			)
			VALUES (
				$1, $2, 'active', $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12, $13,
				$14, $15, $16, $17, $18, $19, $20,
				$21, $22, NULL, NOW()
			)
			RETURNING id
		`,
			input.Provider,
			input.ExternalID,
			postDraft.Title,
			strings.TrimSpace(input.Headline),
			strings.TrimSpace(input.Description),
			strings.TrimSpace(input.Instruction),
			postDraft.LocationName,
			input.Severity,
			strings.TrimSpace(input.ProviderSeverity),
			strings.TrimSpace(input.Urgency),
			strings.TrimSpace(input.Certainty),
			strings.TrimSpace(input.Source),
			strings.TrimSpace(input.SourceURL),
			input.StartsAt,
			input.EndsAt,
			input.ExpiresAt,
			nullableJSON(input.Geometry),
			input.CenterLatitude,
			input.CenterLongitude,
			input.RadiusKm,
			postID,
			observedAt,
		).Scan(&alertID)
		if err != nil {
			return entity.WeatherAlertUpsertResult{}, fmt.Errorf("insert weather alert: %w", err)
		}
	default:
		if err != nil {
			return entity.WeatherAlertUpsertResult{}, fmt.Errorf("select weather alert: %w", err)
		}

		if strings.TrimSpace(postID) == "" {
			postID, err = r.insertWeatherAlertPost(ctx, tx, systemUserID, postDraft, postUrgency)
			if err != nil {
				return entity.WeatherAlertUpsertResult{}, err
			}
		} else if err := r.updateWeatherAlertPost(ctx, tx, postID, postDraft, postUrgency); err != nil {
			return entity.WeatherAlertUpsertResult{}, err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE weather_alerts
			SET status = 'active',
			    event = $2,
			    headline = $3,
			    description = $4,
			    instruction = $5,
			    area_desc = $6,
			    severity = $7,
			    provider_severity = $8,
			    urgency = $9,
			    certainty = $10,
			    source = $11,
			    source_url = $12,
			    starts_at = $13,
			    ends_at = $14,
			    expires_at = $15,
			    geometry = $16,
			    center_latitude = $17,
			    center_longitude = $18,
			    radius_km = $19,
			    post_id = $20::uuid,
			    last_seen_at = $21,
			    resolved_at = NULL,
			    updated_at = NOW()
			WHERE id = $1
		`,
			alertID,
			postDraft.Title,
			strings.TrimSpace(input.Headline),
			strings.TrimSpace(input.Description),
			strings.TrimSpace(input.Instruction),
			postDraft.LocationName,
			input.Severity,
			strings.TrimSpace(input.ProviderSeverity),
			strings.TrimSpace(input.Urgency),
			strings.TrimSpace(input.Certainty),
			strings.TrimSpace(input.Source),
			strings.TrimSpace(input.SourceURL),
			input.StartsAt,
			input.EndsAt,
			input.ExpiresAt,
			nullableJSON(input.Geometry),
			input.CenterLatitude,
			input.CenterLongitude,
			input.RadiusKm,
			postID,
			observedAt,
		); err != nil {
			return entity.WeatherAlertUpsertResult{}, fmt.Errorf("update weather alert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.WeatherAlertUpsertResult{}, fmt.Errorf("commit weather alert tx: %w", err)
	}

	post, err := (&PostRepository{postgres: r.postgres}).GetPost(ctx, "", postID)
	if err != nil {
		return entity.WeatherAlertUpsertResult{}, fmt.Errorf("load alert post: %w", err)
	}

	return entity.WeatherAlertUpsertResult{
		Alert: entity.WeatherAlert{
			ID:               alertID,
			Provider:         input.Provider,
			ExternalID:       input.ExternalID,
			Status:           entity.WeatherAlertStatusActive,
			Event:            postDraft.Title,
			Headline:         strings.TrimSpace(input.Headline),
			Description:      strings.TrimSpace(input.Description),
			Instruction:      strings.TrimSpace(input.Instruction),
			AreaDesc:         postDraft.LocationName,
			Severity:         input.Severity,
			ProviderSeverity: strings.TrimSpace(input.ProviderSeverity),
			Urgency:          strings.TrimSpace(input.Urgency),
			Certainty:        strings.TrimSpace(input.Certainty),
			Source:           strings.TrimSpace(input.Source),
			SourceURL:        strings.TrimSpace(input.SourceURL),
			StartsAt:         input.StartsAt,
			EndsAt:           input.EndsAt,
			ExpiresAt:        input.ExpiresAt,
			Geometry:         input.Geometry,
			CenterLatitude:   input.CenterLatitude,
			CenterLongitude:  input.CenterLongitude,
			RadiusKm:         input.RadiusKm,
			PostID:           postID,
			LastSeenAt:       observedAt,
		},
		Post:             post,
		Created:          existingStatus == "",
		WasActive:        existingStatus == string(entity.WeatherAlertStatusActive),
		PreviousSeverity: entity.WeatherAlertSeverity(previousSeverity),
	}, nil
}

func (r *WeatherAlertRepository) ResolveInactiveAlerts(
	ctx context.Context,
	provider string,
	activeExternalIDs []string,
	observedAt time.Time,
) ([]entity.Post, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if activeExternalIDs == nil {
		activeExternalIDs = []string{}
	}

	rows, err := tx.Query(ctx, `
		SELECT COALESCE(post_id::text, '')
		FROM weather_alerts
		WHERE provider = $1
		  AND status = 'active'
		  AND NOT (external_id = ANY($2::text[]))
	`, provider, activeExternalIDs)
	if err != nil {
		return nil, fmt.Errorf("select stale weather alerts: %w", err)
	}
	defer rows.Close()

	postIDs := make([]string, 0)
	for rows.Next() {
		var postID string
		if err := rows.Scan(&postID); err != nil {
			return nil, fmt.Errorf("scan stale weather alert: %w", err)
		}
		if strings.TrimSpace(postID) != "" {
			postIDs = append(postIDs, postID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale weather alerts: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE weather_alerts
		SET status = 'expired',
		    resolved_at = $3,
		    updated_at = NOW()
		WHERE provider = $1
		  AND status = 'active'
		  AND NOT (external_id = ANY($2::text[]))
	`, provider, activeExternalIDs, observedAt); err != nil {
		return nil, fmt.Errorf("resolve weather alerts: %w", err)
	}

	if len(postIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE posts
			SET status = 'resolved',
			    visibility_priority = 0,
			    expires_at = $2,
			    updated_at = NOW()
			WHERE id::text = ANY($1::text[])
		`, postIDs, observedAt); err != nil {
			return nil, fmt.Errorf("resolve alert posts: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit resolve alerts tx: %w", err)
	}

	if len(postIDs) == 0 {
		return nil, nil
	}

	postRepo := &PostRepository{postgres: r.postgres}
	resolved := make([]entity.Post, 0, len(postIDs))
	for _, postID := range postIDs {
		post, err := postRepo.GetPost(ctx, "", postID)
		if err != nil {
			return nil, fmt.Errorf("load resolved alert post %s: %w", postID, err)
		}
		resolved = append(resolved, post)
	}

	return resolved, nil
}

func (r *WeatherAlertRepository) systemUserID(
	ctx context.Context,
	queryRower interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
) (string, error) {
	var userID string
	err := queryRower.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE email = $1
	`, weatherAlertSystemEmail).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", repository.ErrNotFound
		}
		return "", fmt.Errorf("lookup weather alert system user: %w", err)
	}
	return userID, nil
}

func (r *WeatherAlertRepository) insertWeatherAlertPost(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	post entity.Post,
	urgency entity.PostUrgency,
) (string, error) {
	var postID string
	err := tx.QueryRow(ctx, `
		INSERT INTO posts (
			user_id, kind, category, status, title, excerpt, body, location_name,
			latitude, longitude, share_location, image_url, tags,
			ai_post_type, ai_urgency, ai_confidence, ai_rationale, ai_classified_at,
			ai_classification_status, ai_tagged_at, origin, visibility_priority, expires_at
		)
		VALUES (
			$1, $2, $3, 'active', $4, $5, $6, $7,
			$8, $9, TRUE, NULL, $10,
			$11, $12, 1, 'deterministic weather alert ingest', NOW(),
			'classified', NOW(), $13, $14, $15
		)
		RETURNING id::text
	`,
		userID,
		entity.PostKindEmergency,
		entity.PostCategoryEmergency,
		post.Title,
		post.Excerpt,
		post.Body,
		post.LocationName,
		post.Latitude,
		post.Longitude,
		post.Tags,
		entity.PostCategoryEmergency,
		urgency,
		entity.PostOriginWeatherAlert,
		post.VisibilityPriority,
		post.ExpiresAt,
	).Scan(&postID)
	if err != nil {
		return "", fmt.Errorf("insert weather alert post: %w", err)
	}
	return postID, nil
}

func (r *WeatherAlertRepository) updateWeatherAlertPost(
	ctx context.Context,
	tx pgx.Tx,
	postID string,
	post entity.Post,
	urgency entity.PostUrgency,
) error {
	result, err := tx.Exec(ctx, `
		UPDATE posts
		SET kind = $2,
		    category = $3,
		    status = 'active',
		    title = $4,
		    excerpt = $5,
		    body = $6,
		    location_name = $7,
		    latitude = $8,
		    longitude = $9,
		    share_location = TRUE,
		    tags = $10,
		    ai_post_type = $3,
		    ai_urgency = $11,
		    ai_confidence = 1,
		    ai_rationale = 'deterministic weather alert ingest',
		    ai_classified_at = NOW(),
		    ai_classification_status = 'classified',
		    ai_tagged_at = NOW(),
		    origin = $12,
		    visibility_priority = $13,
		    expires_at = $14,
		    updated_at = NOW()
		WHERE id = $1
	`, postID, entity.PostKindEmergency, entity.PostCategoryEmergency, post.Title, post.Excerpt, post.Body, post.LocationName, post.Latitude, post.Longitude, post.Tags, urgency, entity.PostOriginWeatherAlert, post.VisibilityPriority, post.ExpiresAt)
	if err != nil {
		return fmt.Errorf("update weather alert post: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func buildWeatherAlertPost(input entity.WeatherAlertUpsertInput) entity.Post {
	title := strings.TrimSpace(input.Event)
	if title == "" {
		title = "Weather alert"
	}

	locationName := strings.TrimSpace(input.AreaDesc)
	if locationName == "" {
		locationName = "Affected area"
	}
	locationName = trimRunes(locationName, weatherAlertMaxLocationRunes)

	body := buildWeatherAlertBody(input, locationName)

	return entity.Post{
		Kind:               entity.PostKindEmergency,
		Category:           entity.PostCategoryEmergency,
		Status:             entity.PostStatusActive,
		Title:              title,
		Excerpt:            buildWeatherAlertExcerpt(body),
		Body:               body,
		LocationName:       locationName,
		Latitude:           input.CenterLatitude,
		Longitude:          input.CenterLongitude,
		ShareLocation:      true,
		Tags:               buildWeatherAlertTags(title, locationName),
		Origin:             entity.PostOriginWeatherAlert,
		ExpiresAt:          input.ExpiresAt,
		VisibilityPriority: weatherAlertPriority(input.Severity),
	}
}

func buildWeatherAlertBody(input entity.WeatherAlertUpsertInput, locationName string) string {
	lines := []string{
		"System-generated severe weather alert.",
		"",
		"Event: " + fallbackString(strings.TrimSpace(input.Event), "Weather alert"),
		"Area: " + locationName,
		"Severity: " + string(input.Severity),
	}

	if v := strings.TrimSpace(input.ProviderSeverity); v != "" {
		lines = append(lines, "Provider severity: "+v)
	}
	if v := strings.TrimSpace(input.Urgency); v != "" {
		lines = append(lines, "Urgency: "+v)
	}
	if v := strings.TrimSpace(input.Certainty); v != "" {
		lines = append(lines, "Certainty: "+v)
	}
	if input.ExpiresAt != nil {
		lines = append(lines, "Active until: "+input.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if v := strings.TrimSpace(input.Headline); v != "" {
		lines = append(lines, "", v)
	}
	if v := strings.TrimSpace(input.Description); v != "" {
		lines = append(lines, "", v)
	}
	if v := strings.TrimSpace(input.Instruction); v != "" {
		lines = append(lines, "", "Instructions: "+v)
	}
	if v := strings.TrimSpace(input.Source); v != "" {
		lines = append(lines, "", "Source: "+v)
	}
	if v := strings.TrimSpace(input.SourceURL); v != "" {
		lines = append(lines, "Details: "+v)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildWeatherAlertExcerpt(body string) string {
	normalized := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(normalized) <= 160 {
		return normalized
	}
	runes := []rune(normalized)
	return strings.TrimSpace(string(runes[:160])) + "..."
}

func buildWeatherAlertTags(parts ...string) []string {
	seen := map[string]struct{}{}
	tags := make([]string, 0, weatherAlertMaxTagCount)
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		tags = append(tags, value)
	}

	add("weather")
	for _, part := range parts {
		fields := strings.FieldsFunc(part, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		for _, field := range fields {
			if len(tags) >= weatherAlertMaxTagCount {
				return tags
			}
			if utf8.RuneCountInString(field) < 3 {
				continue
			}
			add(field)
		}
	}
	return tags
}

func weatherAlertUrgency(severity entity.WeatherAlertSeverity) entity.PostUrgency {
	switch severity {
	case entity.WeatherAlertSeverityCritical:
		return entity.PostUrgencyCritical
	case entity.WeatherAlertSeverityHigh:
		return entity.PostUrgencyHigh
	default:
		return entity.PostUrgencyNormal
	}
}

func weatherAlertPriority(severity entity.WeatherAlertSeverity) int {
	switch severity {
	case entity.WeatherAlertSeverityCritical:
		return 400
	case entity.WeatherAlertSeverityHigh:
		return 300
	case entity.WeatherAlertSeverityModerate:
		return 200
	case entity.WeatherAlertSeverityLow:
		return 100
	default:
		return 0
	}
}

func trimRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

func fallbackString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nullableJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
