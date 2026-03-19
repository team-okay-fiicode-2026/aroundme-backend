package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type TrustRepository struct {
	postgres *database.Postgres
}

type trustQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type trustExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func NewTrustRepository(postgres *database.Postgres) repository.TrustRepository {
	return &TrustRepository{postgres: postgres}
}

func (r *TrustRepository) GetTrustScore(ctx context.Context, userID string) (entity.TrustScore, error) {
	return buildTrustScore(ctx, r.postgres.Pool(), userID)
}

func (r *TrustRepository) AcknowledgePostHelpers(ctx context.Context, postID, recipientUserID string, helperUserIDs []string) error {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID string
	var postTitle string
	err = tx.QueryRow(ctx, `
		SELECT user_id, title
		FROM posts
		WHERE id = $1
	`, postID).Scan(&ownerID, &postTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrNotFound
		}
		return fmt.Errorf("load post owner: %w", err)
	}

	if ownerID != recipientUserID {
		return repository.ErrForbidden
	}

	normalizedHelperIDs := make([]string, 0, len(helperUserIDs))
	seen := make(map[string]struct{}, len(helperUserIDs))
	for _, helperUserID := range helperUserIDs {
		trimmed := strings.TrimSpace(helperUserID)
		if trimmed == "" || trimmed == recipientUserID {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedHelperIDs = append(normalizedHelperIDs, trimmed)
	}

	if len(normalizedHelperIDs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit empty helper acknowledgement: %w", err)
		}
		return nil
	}

	eligibleHelperIDs := make([]string, 0, len(normalizedHelperIDs))
	acknowledgementNote := buildResolvedPostAcknowledgement(postTitle)

	for _, helperUserID := range normalizedHelperIDs {
		var commented bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM post_comments
				WHERE post_id = $1
				  AND user_id = $2
			)
		`, postID, helperUserID).Scan(&commented); err != nil {
			return fmt.Errorf("validate helper commenter: %w", err)
		}
		if !commented {
			continue
		}

		eligibleHelperIDs = append(eligibleHelperIDs, helperUserID)

		if _, err := tx.Exec(ctx, `
			INSERT INTO trust_interactions (
				provider_user_id,
				recipient_user_id,
				kind,
				status,
				source_post_id,
				note,
				feedback_note,
				positive_feedback,
				created_at,
				completed_at,
				feedback_at,
				updated_at
			)
			SELECT $1, $2, 'help', 'completed', $3, $4, 'Credited when the post was resolved', TRUE, NOW(), NOW(), NOW(), NOW()
			WHERE NOT EXISTS (
				SELECT 1
				FROM trust_interactions ti
				WHERE ti.provider_user_id = $1
				  AND ti.recipient_user_id = $2
				  AND ti.kind = 'help'
				  AND ti.source_post_id = $3
				  AND ti.status = 'completed'
			)
		`, helperUserID, recipientUserID, postID, acknowledgementNote); err != nil {
			return fmt.Errorf("insert resolved helper acknowledgement: %w", err)
		}
	}

	if len(eligibleHelperIDs) == 0 {
		return repository.ErrForbidden
	}

	for _, helperUserID := range eligibleHelperIDs {
		score, err := buildTrustScore(ctx, tx, helperUserID)
		if err != nil {
			return fmt.Errorf("rebuild helper trust score: %w", err)
		}
		if err := syncTrustScoreCache(ctx, tx, helperUserID, score.Score); err != nil {
			return fmt.Errorf("sync helper trust score cache: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit helper acknowledgement: %w", err)
	}

	return nil
}

func (r *TrustRepository) CreateInteraction(ctx context.Context, interaction entity.TrustInteraction) (entity.TrustInteraction, error) {
	err := r.postgres.Pool().QueryRow(ctx, `
		INSERT INTO trust_interactions (
			provider_user_id,
			recipient_user_id,
			kind,
			status,
			source_post_id,
			note
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
		RETURNING id, provider_user_id, recipient_user_id, kind, status, source_post_id, COALESCE(note, ''), created_at
	`,
		interaction.ProviderUserID,
		interaction.RecipientUserID,
		interaction.Kind,
		interaction.Status,
		interaction.SourcePostID,
		interaction.Note,
	).Scan(
		&interaction.ID,
		&interaction.ProviderUserID,
		&interaction.RecipientUserID,
		&interaction.Kind,
		&interaction.Status,
		&interaction.SourcePostID,
		&interaction.Note,
		&interaction.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return entity.TrustInteraction{}, repository.ErrDuplicate
			}
			if pgErr.Code == "23503" {
				return entity.TrustInteraction{}, repository.ErrNotFound
			}
		}
		return entity.TrustInteraction{}, fmt.Errorf("create trust interaction: %w", err)
	}

	return interaction, nil
}

func (r *TrustRepository) RecordInteractionFeedback(ctx context.Context, interactionID, recipientUserID string, positive bool, note string) (entity.TrustInteraction, entity.TrustScore, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var interaction entity.TrustInteraction
	err = tx.QueryRow(ctx, `
		SELECT id, provider_user_id, recipient_user_id, kind, status, source_post_id, COALESCE(note, '')
		FROM trust_interactions
		WHERE id = $1
	`, interactionID).Scan(
		&interaction.ID,
		&interaction.ProviderUserID,
		&interaction.RecipientUserID,
		&interaction.Kind,
		&interaction.Status,
		&interaction.SourcePostID,
		&interaction.Note,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.TrustInteraction{}, entity.TrustScore{}, repository.ErrNotFound
		}
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("load interaction: %w", err)
	}

	if interaction.RecipientUserID != recipientUserID {
		return entity.TrustInteraction{}, entity.TrustScore{}, repository.ErrForbidden
	}
	if interaction.Status != entity.TrustInteractionStatusPending {
		return entity.TrustInteraction{}, entity.TrustScore{}, repository.ErrDuplicate
	}

	err = tx.QueryRow(ctx, `
		UPDATE trust_interactions
		SET status = $2,
		    positive_feedback = $3,
		    feedback_note = NULLIF($4, ''),
		    feedback_at = NOW(),
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING feedback_note, positive_feedback, feedback_at, completed_at
	`, interaction.ID, entity.TrustInteractionStatusCompleted, positive, note).Scan(
		&interaction.FeedbackNote,
		&interaction.PositiveFeedback,
		&interaction.FeedbackAt,
		&interaction.CompletedAt,
	)
	if err != nil {
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("record feedback: %w", err)
	}
	interaction.Status = entity.TrustInteractionStatusCompleted

	score, err := buildTrustScore(ctx, tx, interaction.ProviderUserID)
	if err != nil {
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("build trust score: %w", err)
	}

	if err := syncTrustScoreCache(ctx, tx, interaction.ProviderUserID, score.Score); err != nil {
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("update trust score cache: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.TrustInteraction{}, entity.TrustScore{}, fmt.Errorf("commit: %w", err)
	}

	return interaction, score, nil
}

func buildTrustScore(ctx context.Context, queryer trustQueryer, userID string) (entity.TrustScore, error) {
	var score entity.TrustScore
	score.UserID = userID

	err := queryer.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COUNT(*)
				FROM trust_interactions ti
				WHERE ti.provider_user_id = u.id
				  AND ti.status = 'completed'
				  AND ti.positive_feedback = TRUE
			), 0),
			COALESCE((
				SELECT COUNT(*)
				FROM post_reactions pr
				JOIN posts p ON p.id = pr.post_id
				WHERE p.user_id = u.id
				  AND pr.user_id <> u.id
			), 0),
			COALESCE((
				SELECT COUNT(*)
				FROM post_comment_reactions pcr
				JOIN post_comments pc ON pc.id = pcr.comment_id
				WHERE pc.user_id = u.id
				  AND pcr.user_id <> u.id
			), 0)
		FROM users u
		WHERE u.id = $1
	`, userID).Scan(
		&score.QualifiedEventCount,
		&score.PostReactionCount,
		&score.CommentReactionCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.TrustScore{}, repository.ErrNotFound
		}
		return entity.TrustScore{}, fmt.Errorf("get trust score counts: %w", err)
	}

	helpDetails, err := listHelpBreakdownDetails(ctx, queryer, userID)
	if err != nil {
		return entity.TrustScore{}, fmt.Errorf("list help trust details: %w", err)
	}
	postDetails, err := listPostReactionBreakdownDetails(ctx, queryer, userID)
	if err != nil {
		return entity.TrustScore{}, fmt.Errorf("list post reaction details: %w", err)
	}
	commentDetails, err := listCommentReactionBreakdownDetails(ctx, queryer, userID)
	if err != nil {
		return entity.TrustScore{}, fmt.Errorf("list comment reaction details: %w", err)
	}

	score.Score = entity.ComputeTotalTrustScore(
		score.QualifiedEventCount,
		score.PostReactionCount,
		score.CommentReactionCount,
	)
	score.EventsUntilNextIncrease = entity.EventsUntilNextTrustIncrease(score.QualifiedEventCount)
	score.Breakdown = []entity.TrustScoreBreakdown{
		{
			Key:                     "helpers",
			Label:                   "Resolved posts you helped with",
			SourceCount:             score.QualifiedEventCount,
			Points:                  entity.ComputeTrustScore(score.QualifiedEventCount),
			Milestone:               entity.TrustInteractionMilestone,
			RemainingUntilNextPoint: entity.RemainingUntilNextTrustPoint(score.QualifiedEventCount, entity.TrustInteractionMilestone),
			Details:                 helpDetails,
		},
		{
			Key:                     "post_reactions",
			Label:                   "Likes on your posts",
			SourceCount:             score.PostReactionCount,
			Points:                  score.PostReactionCount / entity.TrustPostReactionMilestone,
			Milestone:               entity.TrustPostReactionMilestone,
			RemainingUntilNextPoint: entity.RemainingUntilNextTrustPoint(score.PostReactionCount, entity.TrustPostReactionMilestone),
			Details:                 postDetails,
		},
		{
			Key:                     "comment_reactions",
			Label:                   "Likes on your comments",
			SourceCount:             score.CommentReactionCount,
			Points:                  score.CommentReactionCount / entity.TrustCommentReactionMilestone,
			Milestone:               entity.TrustCommentReactionMilestone,
			RemainingUntilNextPoint: entity.RemainingUntilNextTrustPoint(score.CommentReactionCount, entity.TrustCommentReactionMilestone),
			Details:                 commentDetails,
		},
	}

	return score, nil
}

func listHelpBreakdownDetails(ctx context.Context, queryer trustQueryer, userID string) ([]entity.TrustScoreBreakdownDetail, error) {
	rows, err := queryer.Query(ctx, `
		SELECT
			ti.id,
			COALESCE(NULLIF(p.title, ''), 'Neighbor help'),
			u.name,
			COALESCE(ti.feedback_at, ti.completed_at, ti.created_at)
		FROM trust_interactions ti
		JOIN users u ON u.id = ti.recipient_user_id
		LEFT JOIN posts p ON p.id = ti.source_post_id
		WHERE ti.provider_user_id = $1
		  AND ti.status = 'completed'
		  AND ti.positive_feedback = TRUE
		ORDER BY COALESCE(ti.feedback_at, ti.completed_at, ti.created_at) DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query help details: %w", err)
	}
	defer rows.Close()

	details := make([]entity.TrustScoreBreakdownDetail, 0, 5)
	for rows.Next() {
		var detail entity.TrustScoreBreakdownDetail
		var acknowledgedBy string
		var createdAt time.Time
		if err := rows.Scan(&detail.ID, &detail.Title, &acknowledgedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("scan help detail: %w", err)
		}
		detail.Subtitle = fmt.Sprintf("Acknowledged by %s", acknowledgedBy)
		detail.Count = 1
		detail.CreatedAt = &createdAt
		details = append(details, detail)
	}

	return details, rows.Err()
}

func listPostReactionBreakdownDetails(ctx context.Context, queryer trustQueryer, userID string) ([]entity.TrustScoreBreakdownDetail, error) {
	rows, err := queryer.Query(ctx, `
		SELECT
			p.id,
			p.title,
			COUNT(pr.user_id)::int,
			MAX(pr.created_at)
		FROM posts p
		JOIN post_reactions pr ON pr.post_id = p.id
		WHERE p.user_id = $1
		  AND pr.user_id <> $1
		GROUP BY p.id, p.title
		HAVING COUNT(pr.user_id) > 0
		ORDER BY COUNT(pr.user_id) DESC, MAX(pr.created_at) DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query post reaction details: %w", err)
	}
	defer rows.Close()

	details := make([]entity.TrustScoreBreakdownDetail, 0, 5)
	for rows.Next() {
		var detail entity.TrustScoreBreakdownDetail
		var createdAt time.Time
		if err := rows.Scan(&detail.ID, &detail.Title, &detail.Count, &createdAt); err != nil {
			return nil, fmt.Errorf("scan post reaction detail: %w", err)
		}
		detail.Subtitle = "Neighbors reacted to this post"
		detail.CreatedAt = &createdAt
		details = append(details, detail)
	}

	return details, rows.Err()
}

func listCommentReactionBreakdownDetails(ctx context.Context, queryer trustQueryer, userID string) ([]entity.TrustScoreBreakdownDetail, error) {
	rows, err := queryer.Query(ctx, `
		SELECT
			pc.id,
			pc.body,
			p.title,
			COUNT(pcr.user_id)::int,
			MAX(pcr.created_at)
		FROM post_comments pc
		JOIN post_comment_reactions pcr ON pcr.comment_id = pc.id
		JOIN posts p ON p.id = pc.post_id
		WHERE pc.user_id = $1
		  AND pcr.user_id <> $1
		GROUP BY pc.id, pc.body, p.title
		HAVING COUNT(pcr.user_id) > 0
		ORDER BY COUNT(pcr.user_id) DESC, MAX(pcr.created_at) DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query comment reaction details: %w", err)
	}
	defer rows.Close()

	details := make([]entity.TrustScoreBreakdownDetail, 0, 5)
	for rows.Next() {
		var detail entity.TrustScoreBreakdownDetail
		var postTitle string
		var createdAt time.Time
		if err := rows.Scan(&detail.ID, &detail.Title, &postTitle, &detail.Count, &createdAt); err != nil {
			return nil, fmt.Errorf("scan comment reaction detail: %w", err)
		}
		detail.Title = trimForDisplay(detail.Title, 72)
		detail.Subtitle = fmt.Sprintf("On \"%s\"", trimForDisplay(postTitle, 48))
		detail.CreatedAt = &createdAt
		details = append(details, detail)
	}

	return details, rows.Err()
}

func syncTrustScoreCache(ctx context.Context, execer trustExecer, userID string, score int) error {
	if _, err := execer.Exec(ctx, `
		UPDATE users
		SET trust_score = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, userID, score); err != nil {
		return err
	}
	return nil
}

func buildResolvedPostAcknowledgement(postTitle string) string {
	postTitle = strings.TrimSpace(postTitle)
	if postTitle == "" {
		return "Credited when a post was resolved"
	}
	return trimForDisplay(fmt.Sprintf("Credited when \"%s\" was resolved", postTitle), 280)
}

func trimForDisplay(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "..."
}
