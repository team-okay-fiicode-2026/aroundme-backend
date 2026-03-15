package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type ProfileRepository struct {
	postgres *database.Postgres
}

func NewProfileRepository(postgres *database.Postgres) repository.ProfileRepository {
	return &ProfileRepository{postgres: postgres}
}

func (r *ProfileRepository) GetProfile(ctx context.Context, userID string) (entity.Profile, error) {
	var p entity.Profile

	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT id, email, name, COALESCE(avatar_url, ''), bio,
		       latitude, longitude, neighborhood_radius_km,
		       quiet_hours_start::text, quiet_hours_end::text,
		       distance_limit_km
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&p.ID, &p.Email, &p.Name, &p.AvatarURL, &p.Bio,
		&p.Latitude, &p.Longitude, &p.NeighborhoodRadiusKm,
		&p.QuietHoursStart, &p.QuietHoursEnd,
		&p.DistanceLimitKm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Profile{}, repository.ErrNotFound
		}
		return entity.Profile{}, fmt.Errorf("get profile: %w", err)
	}

	skills, err := r.listSkills(ctx, userID)
	if err != nil {
		return entity.Profile{}, fmt.Errorf("list skills: %w", err)
	}
	p.Skills = skills

	items, err := r.listItems(ctx, userID)
	if err != nil {
		return entity.Profile{}, fmt.Errorf("list items: %w", err)
	}
	p.Items = items

	return p, nil
}

func (r *ProfileRepository) UpdateProfile(ctx context.Context, p entity.Profile) (entity.Profile, error) {
	err := r.postgres.Pool().QueryRow(ctx, `
		UPDATE users
		SET name = $2, bio = $3,
		    latitude = $4, longitude = $5,
		    neighborhood_radius_km = $6,
		    quiet_hours_start = $7::time, quiet_hours_end = $8::time,
		    distance_limit_km = $9,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, name, COALESCE(avatar_url, ''), bio,
		          latitude, longitude, neighborhood_radius_km,
		          quiet_hours_start::text, quiet_hours_end::text,
		          distance_limit_km
	`, p.ID, p.Name, p.Bio,
		p.Latitude, p.Longitude,
		p.NeighborhoodRadiusKm,
		p.QuietHoursStart, p.QuietHoursEnd,
		p.DistanceLimitKm,
	).Scan(
		&p.ID, &p.Email, &p.Name, &p.AvatarURL, &p.Bio,
		&p.Latitude, &p.Longitude, &p.NeighborhoodRadiusKm,
		&p.QuietHoursStart, &p.QuietHoursEnd,
		&p.DistanceLimitKm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Profile{}, repository.ErrNotFound
		}
		return entity.Profile{}, fmt.Errorf("update profile: %w", err)
	}

	return p, nil
}

func (r *ProfileRepository) SetSkills(ctx context.Context, userID string, tags []string) ([]string, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM user_skills WHERE user_id = $1`, userID); err != nil {
		return nil, fmt.Errorf("delete skills: %w", err)
	}

	for _, tag := range tags {
		if _, err := tx.Exec(ctx, `INSERT INTO user_skills (user_id, tag) VALUES ($1, $2)`, userID, tag); err != nil {
			return nil, fmt.Errorf("insert skill %q: %w", tag, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return tags, nil
}

func (r *ProfileRepository) CreateItem(ctx context.Context, item entity.ProfileItem) (entity.ProfileItem, error) {
	err := r.postgres.Pool().QueryRow(ctx, `
		INSERT INTO user_items (user_id, name, description, category)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, description, category, available
	`, item.UserID, item.Name, item.Description, item.Category).Scan(
		&item.ID, &item.UserID, &item.Name, &item.Description, &item.Category, &item.Available,
	)
	if err != nil {
		return entity.ProfileItem{}, fmt.Errorf("create item: %w", err)
	}
	return item, nil
}

func (r *ProfileRepository) UpdateItem(ctx context.Context, item entity.ProfileItem) (entity.ProfileItem, error) {
	err := r.postgres.Pool().QueryRow(ctx, `
		UPDATE user_items
		SET name = $3, description = $4, category = $5, available = $6, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, name, description, category, available
	`, item.ID, item.UserID, item.Name, item.Description, item.Category, item.Available).Scan(
		&item.ID, &item.UserID, &item.Name, &item.Description, &item.Category, &item.Available,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ProfileItem{}, repository.ErrNotFound
		}
		return entity.ProfileItem{}, fmt.Errorf("update item: %w", err)
	}
	return item, nil
}

func (r *ProfileRepository) DeleteItem(ctx context.Context, userID, itemID string) error {
	tag, err := r.postgres.Pool().Exec(ctx, `
		DELETE FROM user_items WHERE id = $1 AND user_id = $2
	`, itemID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ProfileRepository) DeleteAccount(ctx context.Context, userID string) error {
	tag, err := r.postgres.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ProfileRepository) listSkills(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT tag FROM user_skills WHERE user_id = $1 ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (r *ProfileRepository) listItems(ctx context.Context, userID string) ([]entity.ProfileItem, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT id, user_id, name, description, category, available
		FROM user_items
		WHERE user_id = $1
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []entity.ProfileItem
	for rows.Next() {
		var item entity.ProfileItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Description, &item.Category, &item.Available); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
