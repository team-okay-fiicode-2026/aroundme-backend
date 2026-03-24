package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type NotificationRepository struct {
	postgres *database.Postgres
}

func NewNotificationRepository(postgres *database.Postgres) repository.NotificationRepository {
	return &NotificationRepository{postgres: postgres}
}

func (r *NotificationRepository) Create(ctx context.Context, n entity.Notification) (entity.Notification, error) {
	err := r.postgres.Pool().QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, body, entity_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		RETURNING id, user_id, type, title, body, COALESCE(entity_id, ''), is_read, created_at
	`, n.UserID, n.Type, n.Title, n.Body, n.EntityID).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.EntityID, &n.IsRead, &n.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.Notification{}, repository.ErrDuplicate
		}
		return entity.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return n, nil
}

func (r *NotificationRepository) List(ctx context.Context, userID string, limit int) ([]entity.Notification, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var unreadCount int
	if err := r.postgres.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE
	`, userID).Scan(&unreadCount); err != nil {
		return nil, 0, fmt.Errorf("count unread: %w", err)
	}

	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT id, user_id, type, title, body, COALESCE(entity_id, ''), is_read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var items []entity.Notification
	for rows.Next() {
		var n entity.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.EntityID, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}

	return items, unreadCount, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, notificationID, userID string) error {
	_, err := r.postgres.Pool().Exec(ctx, `
		UPDATE notifications SET is_read = TRUE
		WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	return err
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.postgres.Pool().Exec(ctx, `
		UPDATE notifications SET is_read = TRUE WHERE user_id = $1
	`, userID)
	return err
}

func (r *NotificationRepository) UpsertPushToken(ctx context.Context, userID, token string) error {
	_, err := r.postgres.Pool().Exec(ctx, `
		INSERT INTO push_tokens (user_id, token)
		VALUES ($1, $2)
		ON CONFLICT (user_id, token) DO NOTHING
	`, userID, token)
	return err
}

func (r *NotificationRepository) GetPushTokens(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT token FROM push_tokens WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get push tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *NotificationRepository) GetQuietHours(ctx context.Context, userID string) (start, end *string, err error) {
	var rawStart, rawEnd string
	err = r.postgres.Pool().QueryRow(ctx, `
		SELECT COALESCE(TO_CHAR(quiet_hours_start, 'HH24:MI'), ''), COALESCE(TO_CHAR(quiet_hours_end, 'HH24:MI'), '')
		FROM users
		WHERE id = $1
	`, userID).Scan(&rawStart, &rawEnd)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, repository.ErrNotFound
		}
		return nil, nil, fmt.Errorf("get quiet hours: %w", err)
	}
	if rawStart != "" {
		start = &rawStart
	}
	if rawEnd != "" {
		end = &rawEnd
	}
	return start, end, nil
}

func (r *NotificationRepository) ListNearbyUserIDs(ctx context.Context, latitude, longitude, radiusKm float64, excludeUserID string) ([]string, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT id
		FROM users
		WHERE id != $4
		  AND latitude IS NOT NULL
		  AND longitude IS NOT NULL
		  AND (
		    6371 * 2 * ASIN(SQRT(
		      POWER(SIN(RADIANS((latitude - $1) / 2)), 2) +
		      COS(RADIANS($1)) * COS(RADIANS(latitude)) *
		      POWER(SIN(RADIANS((longitude - $2) / 2)), 2)
		    ))
		  ) <= $3
	`, latitude, longitude, radiusKm, excludeUserID)
	if err != nil {
		return nil, fmt.Errorf("list nearby users: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *NotificationRepository) GetPostInfo(ctx context.Context, postID string) (string, string, error) {
	var authorID, title string
	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT user_id, title FROM posts WHERE id = $1
	`, postID).Scan(&authorID, &title)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", repository.ErrNotFound
		}
		return "", "", fmt.Errorf("get post info: %w", err)
	}
	return authorID, title, nil
}
