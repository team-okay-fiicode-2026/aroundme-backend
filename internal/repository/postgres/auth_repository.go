package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type AuthRepository struct {
	postgres *database.Postgres
}

func NewAuthRepository(postgres *database.Postgres) repository.AuthRepository {
	return &AuthRepository{postgres: postgres}
}

func (r *AuthRepository) CreatePasswordUser(
	ctx context.Context,
	user entity.User,
	password string,
	session entity.AuthSession,
) (entity.AuthResult, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.AuthResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	createdUser, err := createUser(ctx, tx, user)
	if err != nil {
		if isUniqueConstraintError(err, "users_email_key") {
			return entity.AuthResult{}, repository.ErrUniqueEmail
		}

		return entity.AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	if err := storePassword(ctx, tx, createdUser.ID, password); err != nil {
		return entity.AuthResult{}, fmt.Errorf("store password: %w", err)
	}

	session.UserID = createdUser.ID
	if err := insertSession(ctx, tx, session); err != nil {
		return entity.AuthResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.AuthResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return entity.AuthResult{
		Session: session,
		User:    createdUser,
	}, nil
}

func (r *AuthRepository) AuthenticateByPassword(
	ctx context.Context,
	email string,
	password string,
	session entity.AuthSession,
) (entity.AuthResult, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.AuthResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	user, err := getUserByPassword(ctx, tx, email, password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.AuthResult{}, repository.ErrNotFound
		}

		return entity.AuthResult{}, fmt.Errorf("lookup user: %w", err)
	}

	session.UserID = user.ID
	if err := insertSession(ctx, tx, session); err != nil {
		return entity.AuthResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.AuthResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return entity.AuthResult{
		Session: session,
		User:    user,
	}, nil
}

func (r *AuthRepository) AuthenticateBySocial(
	ctx context.Context,
	provider entity.SocialProvider,
	providerUserID string,
	user entity.User,
	session entity.AuthSession,
) (entity.AuthResult, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.AuthResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	currentUser, found, err := getUserBySocialAccount(ctx, tx, provider, providerUserID)
	if err != nil {
		return entity.AuthResult{}, fmt.Errorf("lookup social account: %w", err)
	}

	if !found {
		currentUser, found, err = getUserByEmail(ctx, tx, user.Email)
		if err != nil {
			return entity.AuthResult{}, fmt.Errorf("lookup user by email: %w", err)
		}

		if !found {
			currentUser, err = createUser(ctx, tx, user)
			if err != nil {
				if isUniqueConstraintError(err, "users_email_key") {
					currentUser, found, err = getUserByEmail(ctx, tx, user.Email)
					if err != nil {
						return entity.AuthResult{}, fmt.Errorf("reload user by email: %w", err)
					}
					if !found {
						return entity.AuthResult{}, repository.ErrUniqueEmail
					}
				} else {
					return entity.AuthResult{}, fmt.Errorf("create user: %w", err)
				}
			}
		}
	}

	currentUser.Email = user.Email
	currentUser.Name = user.Name
	currentUser.AvatarURL = user.AvatarURL

	currentUser, err = updateUserProfile(ctx, tx, currentUser)
	if err != nil {
		if isUniqueConstraintError(err, "users_email_key") {
			return entity.AuthResult{}, repository.ErrUniqueEmail
		}

		return entity.AuthResult{}, fmt.Errorf("update user profile: %w", err)
	}

	if err := upsertSocialAccount(ctx, tx, currentUser.ID, provider, providerUserID, user.Email); err != nil {
		return entity.AuthResult{}, fmt.Errorf("upsert social account: %w", err)
	}

	session.UserID = currentUser.ID
	if err := insertSession(ctx, tx, session); err != nil {
		return entity.AuthResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.AuthResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return entity.AuthResult{
		Session: session,
		User:    currentUser,
	}, nil
}

func (r *AuthRepository) RefreshSession(
	ctx context.Context,
	refreshTokenHash string,
	replacement entity.AuthSession,
) (entity.AuthResult, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.AuthResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	sessionID, user, err := getRefreshSession(ctx, tx, refreshTokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.AuthResult{}, repository.ErrNotFound
		}

		return entity.AuthResult{}, fmt.Errorf("lookup session: %w", err)
	}

	replacement.ID = sessionID
	replacement.UserID = user.ID

	updated, err := rotateSession(ctx, tx, replacement)
	if err != nil {
		return entity.AuthResult{}, err
	}
	if !updated {
		return entity.AuthResult{}, repository.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.AuthResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return entity.AuthResult{
		Session: replacement,
		User:    user,
	}, nil
}

func (r *AuthRepository) RevokeSession(ctx context.Context, refreshTokenHash string) error {
	if _, err := r.postgres.Pool().Exec(ctx, `
		UPDATE sessions
		SET revoked_at = NOW(), last_used_at = NOW()
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
	`, refreshTokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func createUser(ctx context.Context, tx pgx.Tx, user entity.User) (entity.User, error) {
	var createdUser entity.User

	err := tx.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url)
		VALUES ($1, $2, NULLIF($3, ''))
		RETURNING id, email, name, COALESCE(avatar_url, '')
	`, user.Email, user.Name, user.AvatarURL).Scan(
		&createdUser.ID,
		&createdUser.Email,
		&createdUser.Name,
		&createdUser.AvatarURL,
	)
	if err != nil {
		return entity.User{}, err
	}

	return createdUser, nil
}

func storePassword(ctx context.Context, tx pgx.Tx, userID, password string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auth_passwords (user_id, password_hash)
		VALUES ($1, crypt($2, gen_salt('bf')))
	`, userID, password)

	return err
}

func getUserByPassword(ctx context.Context, tx pgx.Tx, email, password string) (entity.User, error) {
	var user entity.User

	err := tx.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, COALESCE(u.avatar_url, '')
		FROM users u
		INNER JOIN auth_passwords ap ON ap.user_id = u.id
		WHERE u.email = $1
		  AND ap.password_hash = crypt($2, ap.password_hash)
	`, email, password).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err != nil {
		return entity.User{}, err
	}

	return user, nil
}

func getUserByEmail(ctx context.Context, tx pgx.Tx, email string) (entity.User, bool, error) {
	var user entity.User

	err := tx.QueryRow(ctx, `
		SELECT id, email, name, COALESCE(avatar_url, '')
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, false, nil
		}

		return entity.User{}, false, err
	}

	return user, true, nil
}

func getUserBySocialAccount(
	ctx context.Context,
	tx pgx.Tx,
	provider entity.SocialProvider,
	providerUserID string,
) (entity.User, bool, error) {
	var user entity.User

	err := tx.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, COALESCE(u.avatar_url, '')
		FROM auth_social_accounts asa
		INNER JOIN users u ON u.id = asa.user_id
		WHERE asa.provider = $1
		  AND asa.provider_user_id = $2
	`, string(provider), providerUserID).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, false, nil
		}

		return entity.User{}, false, err
	}

	return user, true, nil
}

func updateUserProfile(ctx context.Context, tx pgx.Tx, user entity.User) (entity.User, error) {
	var updatedUser entity.User

	err := tx.QueryRow(ctx, `
		UPDATE users
		SET email = $2,
			name = $3,
			avatar_url = NULLIF($4, ''),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, name, COALESCE(avatar_url, '')
	`, user.ID, user.Email, user.Name, user.AvatarURL).Scan(
		&updatedUser.ID,
		&updatedUser.Email,
		&updatedUser.Name,
		&updatedUser.AvatarURL,
	)
	if err != nil {
		return entity.User{}, err
	}

	return updatedUser, nil
}

func upsertSocialAccount(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	provider entity.SocialProvider,
	providerUserID,
	email string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auth_social_accounts (user_id, provider, provider_user_id, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_user_id)
		DO UPDATE SET email = EXCLUDED.email, updated_at = NOW()
	`, userID, string(provider), providerUserID, email)

	return err
}

func insertSession(ctx context.Context, tx pgx.Tx, session entity.AuthSession) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sessions (
			user_id,
			access_token_hash,
			refresh_token_hash,
			access_expires_at,
			refresh_expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, session.UserID, session.AccessTokenHash, session.RefreshTokenHash, session.AccessTokenExpiresAt, session.RefreshTokenExpiresAt)
	if err == nil {
		return nil
	}

	if isUniqueConstraintError(err, "sessions_access_token_hash_key") ||
		isUniqueConstraintError(err, "sessions_refresh_token_hash_key") {
		return repository.ErrTokenConflict
	}

	return fmt.Errorf("insert session: %w", err)
}

func getRefreshSession(ctx context.Context, tx pgx.Tx, refreshTokenHash string) (string, entity.User, error) {
	var sessionID string
	var user entity.User

	err := tx.QueryRow(ctx, `
		SELECT s.id, u.id, u.email, u.name, COALESCE(u.avatar_url, '')
		FROM sessions s
		INNER JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.refresh_expires_at > NOW()
		FOR UPDATE
	`, refreshTokenHash).Scan(&sessionID, &user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err != nil {
		return "", entity.User{}, err
	}

	return sessionID, user, nil
}

func rotateSession(ctx context.Context, tx pgx.Tx, session entity.AuthSession) (bool, error) {
	commandTag, err := tx.Exec(ctx, `
		UPDATE sessions
		SET access_token_hash = $2,
			refresh_token_hash = $3,
			access_expires_at = $4,
			refresh_expires_at = $5,
			last_refreshed_at = NOW(),
			last_used_at = NOW()
		WHERE id = $1
		  AND revoked_at IS NULL
	`, session.ID, session.AccessTokenHash, session.RefreshTokenHash, session.AccessTokenExpiresAt, session.RefreshTokenExpiresAt)
	if err != nil {
		if isUniqueConstraintError(err, "sessions_access_token_hash_key") ||
			isUniqueConstraintError(err, "sessions_refresh_token_hash_key") {
			return false, repository.ErrTokenConflict
		}

		return false, fmt.Errorf("rotate session: %w", err)
	}

	return commandTag.RowsAffected() > 0, nil
}

func (r *AuthRepository) ValidateAccessToken(ctx context.Context, accessTokenHash string) (entity.User, error) {
	var user entity.User

	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT u.id, u.email, u.name, COALESCE(u.avatar_url, '')
		FROM sessions s
		INNER JOIN users u ON u.id = s.user_id
		WHERE s.access_token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.access_expires_at > NOW()
	`, accessTokenHash).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err != nil {
		return entity.User{}, err
	}

	return user, nil
}

func isUniqueConstraintError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
}
