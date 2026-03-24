package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type PostRepository struct {
	postgres *database.Postgres
}

type geoPoint struct {
	Latitude  float64
	Longitude float64
}

func NewPostRepository(postgres *database.Postgres) repository.PostRepository {
	return &PostRepository{postgres: postgres}
}

func (r *PostRepository) ListPosts(ctx context.Context, input entity.ListPostsInput) ([]entity.Post, *entity.PostCursor, error) {
	origin, err := r.viewerOrigin(ctx, input.ViewerUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("viewer origin: %w", err)
	}

	args := []any{input.ViewerUserID}
	statusRankExpr := "CASE WHEN p.status = 'active' THEN 0 ELSE 1 END"
	priorityExpr := "COALESCE(p.visibility_priority, 0)"
	where := []string{"1=1"}

	if input.Status == nil {
		where = append(where, "NOT (p.origin = 'weather_alert' AND p.status = 'resolved')")
	}

	distanceSelect := "NULL::double precision AS distance_km"
	var distanceExpr string
	if origin != nil {
		latPos := len(args) + 1
		args = append(args, origin.Latitude)
		lonPos := len(args) + 1
		args = append(args, origin.Longitude)
		distanceExpr = distanceExpression(latPos, lonPos)
		distanceSelect = distanceExpr + " AS distance_km"

		if input.DistanceKm != nil {
			latDelta, lonDelta := distanceBoundingBox(origin.Latitude, *input.DistanceKm)
			minLatPos := len(args) + 1
			args = append(args, origin.Latitude-latDelta)
			maxLatPos := len(args) + 1
			args = append(args, origin.Latitude+latDelta)
			minLonPos := len(args) + 1
			args = append(args, origin.Longitude-lonDelta)
			maxLonPos := len(args) + 1
			args = append(args, origin.Longitude+lonDelta)
			maxDistancePos := len(args) + 1
			args = append(args, *input.DistanceKm)

			where = append(where,
				fmt.Sprintf("p.latitude BETWEEN $%d AND $%d", minLatPos, maxLatPos),
				fmt.Sprintf("p.longitude BETWEEN $%d AND $%d", minLonPos, maxLonPos),
				fmt.Sprintf("%s <= $%d", distanceExpr, maxDistancePos),
			)
		}
	}

	if input.AuthorID != nil {
		where = append(where, fmt.Sprintf("p.user_id = $%d", len(args)+1))
		args = append(args, *input.AuthorID)
	}

	if input.Category != nil {
		where = append(where, fmt.Sprintf("p.category = $%d", len(args)+1))
		args = append(args, *input.Category)
	}

	if input.Status != nil {
		where = append(where, fmt.Sprintf("p.status = $%d", len(args)+1))
		args = append(args, *input.Status)
	}

	if input.Cursor != nil {
		priorityPos := len(args) + 1
		args = append(args, input.Cursor.VisibilityPriority)
		rankPos := len(args) + 1
		args = append(args, input.Cursor.StatusRank)
		createdAtPos := len(args) + 1
		args = append(args, input.Cursor.CreatedAt)
		idPos := len(args) + 1
		args = append(args, input.Cursor.ID)

		where = append(where, fmt.Sprintf(
			"(%s < $%d OR (%s = $%d AND (%s > $%d OR (%s = $%d AND (p.created_at < $%d OR (p.created_at = $%d AND p.id < $%d))))))",
			priorityExpr,
			priorityPos,
			priorityExpr,
			priorityPos,
			statusRankExpr,
			rankPos,
			statusRankExpr,
			rankPos,
			createdAtPos,
			createdAtPos,
			idPos,
		))
	}

	limitPos := len(args) + 1
	args = append(args, input.Limit+1)

	query := fmt.Sprintf(`
		SELECT
			p.id,
			p.user_id,
			u.name,
			p.kind,
			p.category,
			COALESCE(p.override_urgency, p.ai_urgency, ''),
			COALESCE(p.ai_post_type, ''),
			COALESCE(p.ai_classification_status, 'pending'),
			p.status,
			p.title,
			p.excerpt,
			p.body,
			p.location_name,
			p.latitude,
			p.longitude,
			p.share_location,
			COALESCE(p.image_url, ''),
			p.tags,
			p.reaction_count,
			p.comment_count,
			p.origin,
			p.visibility_priority,
			p.expires_at,
			p.created_at,
			p.updated_at,
			EXISTS(
				SELECT 1
				FROM post_reactions pr
				WHERE pr.post_id = p.id AND pr.user_id = $1
			) AS is_reacted,
			%s
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE %s
		ORDER BY %s DESC, %s ASC, p.created_at DESC, p.id DESC
		LIMIT $%d
	`, distanceSelect, strings.Join(where, " AND "), priorityExpr, statusRankExpr, limitPos)

	rows, err := r.postgres.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	posts := make([]entity.Post, 0, input.Limit+1)
	for rows.Next() {
		var post entity.Post
		var aiPostType string
		var aiUrgency string
		if err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.AuthorName,
			&post.Kind,
			&post.Category,
			&aiUrgency,
			&aiPostType,
			&post.AIClassificationStatus,
			&post.Status,
			&post.Title,
			&post.Excerpt,
			&post.Body,
			&post.LocationName,
			&post.Latitude,
			&post.Longitude,
			&post.ShareLocation,
			&post.ImageURL,
			&post.Tags,
			&post.ReactionCount,
			&post.CommentCount,
			&post.Origin,
			&post.VisibilityPriority,
			&post.ExpiresAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.IsReacted,
			&post.DistanceKm,
		); err != nil {
			return nil, nil, fmt.Errorf("scan post: %w", err)
		}
		post.AIUrgency = entity.PostUrgency(aiUrgency)
		post.AIPostType = entity.PostCategory(aiPostType)

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate posts: %w", err)
	}

	var nextCursor *entity.PostCursor
	if len(posts) > input.Limit {
		last := posts[input.Limit]
		nextCursor = &entity.PostCursor{
			VisibilityPriority: last.VisibilityPriority,
			StatusRank:         statusRank(last.Status),
			CreatedAt:          last.CreatedAt,
			ID:                 last.ID,
		}
		posts = posts[:input.Limit]
	}

	return posts, nextCursor, nil
}

func (r *PostRepository) GetPost(ctx context.Context, viewerUserID, postID string) (entity.Post, error) {
	origin, err := r.viewerOrigin(ctx, viewerUserID)
	if err != nil {
		return entity.Post{}, fmt.Errorf("viewer origin: %w", err)
	}

	args := []any{viewerUserID, postID}
	distanceSelect := "NULL::double precision AS distance_km"
	if origin != nil {
		latPos := len(args) + 1
		args = append(args, origin.Latitude)
		lonPos := len(args) + 1
		args = append(args, origin.Longitude)
		distanceSelect = distanceExpression(latPos, lonPos) + " AS distance_km"
	}

	query := fmt.Sprintf(`
		SELECT
			p.id,
			p.user_id,
			u.name,
			p.kind,
			p.category,
			COALESCE(p.override_urgency, p.ai_urgency, ''),
			COALESCE(p.ai_post_type, ''),
			COALESCE(p.ai_classification_status, 'pending'),
			p.status,
			p.title,
			p.excerpt,
			p.body,
			p.location_name,
			p.latitude,
			p.longitude,
			p.share_location,
			COALESCE(p.image_url, ''),
			p.tags,
			p.reaction_count,
			p.comment_count,
			p.origin,
			p.visibility_priority,
			p.expires_at,
			p.created_at,
			p.updated_at,
			EXISTS(
				SELECT 1
				FROM post_reactions pr
				WHERE pr.post_id = p.id AND pr.user_id = $1
			) AS is_reacted,
			%s
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.id = $2
	`, distanceSelect)

	var post entity.Post
	var aiPostType string
	var aiUrgency string
	err = r.postgres.Pool().QueryRow(ctx, query, args...).Scan(
		&post.ID,
		&post.UserID,
		&post.AuthorName,
		&post.Kind,
		&post.Category,
		&aiUrgency,
		&aiPostType,
		&post.AIClassificationStatus,
		&post.Status,
		&post.Title,
		&post.Excerpt,
		&post.Body,
		&post.LocationName,
		&post.Latitude,
		&post.Longitude,
		&post.ShareLocation,
		&post.ImageURL,
		&post.Tags,
		&post.ReactionCount,
		&post.CommentCount,
		&post.Origin,
		&post.VisibilityPriority,
		&post.ExpiresAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.IsReacted,
		&post.DistanceKm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Post{}, repository.ErrNotFound
		}
		return entity.Post{}, fmt.Errorf("get post: %w", err)
	}
	post.AIUrgency = entity.PostUrgency(aiUrgency)
	post.AIPostType = entity.PostCategory(aiPostType)

	return post, nil
}

func (r *PostRepository) CreatePost(ctx context.Context, post entity.Post) (entity.Post, error) {
	var postID string
	err := r.postgres.Pool().QueryRow(ctx, `
		INSERT INTO posts (
			user_id,
			kind,
			category,
			status,
			title,
			excerpt,
			body,
			location_name,
			latitude,
			longitude,
			share_location,
			image_url,
			tags
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), $13)
		RETURNING id
	`, post.UserID, post.Kind, post.Category, post.Status, post.Title, post.Excerpt, post.Body, post.LocationName, post.Latitude, post.Longitude, post.ShareLocation, post.ImageURL, post.Tags).Scan(&postID)
	if err != nil {
		return entity.Post{}, fmt.Errorf("insert post: %w", err)
	}

	return r.GetPost(ctx, post.UserID, postID)
}

func (r *PostRepository) WriteClassification(ctx context.Context, postID string, input entity.PostClassificationInput) (entity.Post, error) {
	switch input.Status {
	case "failed":
		result, err := r.postgres.Pool().Exec(ctx, `
			UPDATE posts
			SET ai_classification_status = 'failed'
			WHERE id = $1
		`, postID)
		if err != nil {
			return entity.Post{}, fmt.Errorf("mark classification failed: %w", err)
		}
		if result.RowsAffected() == 0 {
			return entity.Post{}, repository.ErrNotFound
		}
		return entity.Post{
			ID:                     postID,
			AIClassificationStatus: "failed",
		}, nil
	case "classified":
		result, err := r.postgres.Pool().Exec(ctx, `
			UPDATE posts
			SET category = $2,
			    ai_post_type = $2,
			    ai_urgency = $3,
			    ai_confidence = $4,
			    ai_rationale = $5,
			    ai_classified_at = NOW(),
			    ai_classification_status = 'classified',
			    ai_tagged_at = NOW(),
			    tags = CASE WHEN cardinality($6::text[]) > 0 THEN $6 ELSE tags END
			WHERE id = $1
		`, postID, input.PostType, input.Urgency, input.Confidence, input.Rationale, input.Tags)
		if err != nil {
			return entity.Post{}, fmt.Errorf("write classification: %w", err)
		}
		if result.RowsAffected() == 0 {
			return entity.Post{}, repository.ErrNotFound
		}
		return r.GetPost(ctx, "", postID)
	default:
		return entity.Post{}, fmt.Errorf("unsupported classification status %q", input.Status)
	}
}

func (r *PostRepository) ToggleReaction(ctx context.Context, postID, userID string) (entity.PostReactionState, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.PostReactionState{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePostExistsTx(ctx, tx, postID); err != nil {
		return entity.PostReactionState{}, err
	}

	inserted := false
	err = tx.QueryRow(ctx, `
		INSERT INTO post_reactions (post_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		RETURNING TRUE
	`, postID, userID).Scan(&inserted)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		inserted = false
	case err != nil:
		return entity.PostReactionState{}, fmt.Errorf("toggle reaction insert: %w", err)
	}

	if !inserted {
		if _, err := tx.Exec(ctx, `
			DELETE FROM post_reactions
			WHERE post_id = $1 AND user_id = $2
		`, postID, userID); err != nil {
			return entity.PostReactionState{}, fmt.Errorf("toggle reaction delete: %w", err)
		}
	}

	var result entity.PostReactionState
	result.IsReacted = inserted

	if err := tx.QueryRow(ctx, `
		UPDATE posts
		SET reaction_count = (
			SELECT COUNT(*)
			FROM post_reactions
			WHERE post_id = $1
		),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING reaction_count
	`, postID).Scan(&result.ReactionCount); err != nil {
		return entity.PostReactionState{}, fmt.Errorf("update reaction count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.PostReactionState{}, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

func (r *PostRepository) ListComments(ctx context.Context, postID string, input entity.ListPostCommentsInput) ([]entity.PostComment, *entity.PostCommentCursor, error) {
	if err := r.ensurePostExists(ctx, postID); err != nil {
		return nil, nil, err
	}

	// Fetch only top-level comments (no parent) for cursor-based pagination.
	// Replies are fetched inline per top-level comment.
	args := []any{postID}
	where := []string{"c.post_id = $1", "c.parent_id IS NULL"}

	if input.Cursor != nil {
		createdAtPos := len(args) + 1
		args = append(args, input.Cursor.CreatedAt)
		idPos := len(args) + 1
		args = append(args, input.Cursor.ID)
		where = append(where, fmt.Sprintf(
			"(c.created_at > $%d OR (c.created_at = $%d AND c.id > $%d))",
			createdAtPos, createdAtPos, idPos,
		))
	}

	limitPos := len(args) + 1
	args = append(args, input.Limit+1)

	rows, err := r.postgres.Pool().Query(ctx, fmt.Sprintf(`
		SELECT
			c.id, c.post_id, c.user_id, u.name,
			c.body, c.reaction_count,
			(SELECT COUNT(*) FROM post_comments r WHERE r.parent_id = c.id) AS reply_count,
			c.created_at, c.updated_at
		FROM post_comments c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $%d
	`, strings.Join(where, " AND "), limitPos), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

	comments := make([]entity.PostComment, 0, input.Limit+1)
	for rows.Next() {
		var comment entity.PostComment
		if err := rows.Scan(
			&comment.ID, &comment.PostID, &comment.UserID, &comment.AuthorName,
			&comment.Body, &comment.ReactionCount, &comment.ReplyCount,
			&comment.CreatedAt, &comment.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate comments: %w", err)
	}

	var nextCursor *entity.PostCommentCursor
	if len(comments) > input.Limit {
		last := comments[input.Limit]
		nextCursor = &entity.PostCommentCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		comments = comments[:input.Limit]
	}

	return comments, nextCursor, nil
}

func (r *PostRepository) CreateComment(ctx context.Context, postID, userID string, parentID *string, body string) (entity.PostComment, int, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.PostComment{}, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePostExistsTx(ctx, tx, postID); err != nil {
		return entity.PostComment{}, 0, err
	}

	var comment entity.PostComment
	var commentCount int
	err = tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO post_comments (post_id, user_id, parent_id, body)
			VALUES ($1, $2, $4, $3)
			RETURNING id, post_id, user_id, parent_id, body, reaction_count, created_at, updated_at
		),
		updated AS (
			UPDATE posts
			SET comment_count = comment_count + 1,
			    updated_at = NOW()
			WHERE id = $1
			RETURNING comment_count
		)
		SELECT
			i.id, i.post_id, i.user_id, u.name, i.parent_id,
			i.body, i.reaction_count,
			i.created_at, i.updated_at,
			(SELECT comment_count FROM updated)
		FROM inserted i
		JOIN users u ON u.id = i.user_id
	`, postID, userID, body, parentID).Scan(
		&comment.ID, &comment.PostID, &comment.UserID, &comment.AuthorName, &comment.ParentID,
		&comment.Body, &comment.ReactionCount,
		&comment.CreatedAt, &comment.UpdatedAt,
		&commentCount,
	)
	if err != nil {
		return entity.PostComment{}, 0, fmt.Errorf("insert comment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.PostComment{}, 0, fmt.Errorf("commit: %w", err)
	}

	return comment, commentCount, nil
}

func (r *PostRepository) ToggleCommentReaction(ctx context.Context, commentID, userID string) (int, bool, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Try to delete existing reaction; if nothing deleted, insert instead.
	var deleted bool
	err = tx.QueryRow(ctx, `
		WITH del AS (
			DELETE FROM post_comment_reactions
			WHERE comment_id = $1 AND user_id = $2
			RETURNING 1
		)
		SELECT EXISTS (SELECT 1 FROM del)
	`, commentID, userID).Scan(&deleted)
	if err != nil {
		return 0, false, fmt.Errorf("delete reaction: %w", err)
	}

	delta := 1
	if deleted {
		delta = -1
	} else {
		if _, err := tx.Exec(ctx, `
			INSERT INTO post_comment_reactions (comment_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, commentID, userID); err != nil {
			return 0, false, fmt.Errorf("insert reaction: %w", err)
		}
	}

	var reactionCount int
	if err := tx.QueryRow(ctx, `
		UPDATE post_comments
		SET reaction_count = GREATEST(0, reaction_count + $2)
		WHERE id = $1
		RETURNING reaction_count
	`, commentID, delta).Scan(&reactionCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, repository.ErrNotFound
		}
		return 0, false, fmt.Errorf("update reaction count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf("commit: %w", err)
	}

	return reactionCount, !deleted, nil
}

func (r *PostRepository) UpdateStatus(ctx context.Context, postID, userID string, status entity.PostStatus) (entity.Post, error) {
	var ownerID string
	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT user_id
		FROM posts
		WHERE id = $1
	`, postID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Post{}, repository.ErrNotFound
		}
		return entity.Post{}, fmt.Errorf("get post owner: %w", err)
	}

	if ownerID != userID {
		return entity.Post{}, repository.ErrForbidden
	}

	if _, err := r.postgres.Pool().Exec(ctx, `
		UPDATE posts
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, postID, status); err != nil {
		return entity.Post{}, fmt.Errorf("update status: %w", err)
	}

	return r.GetPost(ctx, userID, postID)
}

func (r *PostRepository) ResolvePost(ctx context.Context, postID, userID string, helperUserIDs []string) (entity.Post, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.Post{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID string
	var postTitle string
	err = tx.QueryRow(ctx, `
		SELECT user_id, title FROM posts WHERE id = $1
	`, postID).Scan(&ownerID, &postTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Post{}, repository.ErrNotFound
		}
		return entity.Post{}, fmt.Errorf("get post: %w", err)
	}

	if ownerID != userID {
		return entity.Post{}, repository.ErrForbidden
	}

	if _, err := tx.Exec(ctx, `
		UPDATE posts SET status = 'resolved', updated_at = NOW() WHERE id = $1
	`, postID); err != nil {
		return entity.Post{}, fmt.Errorf("update post status: %w", err)
	}

	acknowledgementNote := buildResolvedPostAcknowledgement(postTitle)
	eligibleHelperIDs := make([]string, 0, len(helperUserIDs))

	for _, helperUserID := range helperUserIDs {
		var commented bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM post_comments
				WHERE post_id = $1 AND user_id = $2
			)
		`, postID, helperUserID).Scan(&commented); err != nil {
			return entity.Post{}, fmt.Errorf("validate helper commenter: %w", err)
		}
		if !commented {
			continue
		}

		eligibleHelperIDs = append(eligibleHelperIDs, helperUserID)

		if _, err := tx.Exec(ctx, `
			INSERT INTO trust_interactions (
				provider_user_id, recipient_user_id, kind, status,
				source_post_id, note, feedback_note, positive_feedback,
				created_at, completed_at, feedback_at, updated_at
			)
			SELECT $1, $2, 'help', 'completed', $3, $4, 'Credited when the post was resolved', TRUE, NOW(), NOW(), NOW(), NOW()
			WHERE NOT EXISTS (
				SELECT 1 FROM trust_interactions ti
				WHERE ti.provider_user_id = $1
				  AND ti.recipient_user_id = $2
				  AND ti.kind = 'help'
				  AND ti.source_post_id = $3
				  AND ti.status = 'completed'
			)
		`, helperUserID, userID, postID, acknowledgementNote); err != nil {
			return entity.Post{}, fmt.Errorf("insert trust interaction: %w", err)
		}
	}

	if len(eligibleHelperIDs) == 0 {
		return entity.Post{}, repository.ErrNoEligibleHelpers
	}

	for _, helperUserID := range eligibleHelperIDs {
		score, err := buildTrustScore(ctx, tx, helperUserID)
		if err != nil {
			return entity.Post{}, fmt.Errorf("rebuild trust score: %w", err)
		}
		if err := syncTrustScoreCache(ctx, tx, helperUserID, score.Score); err != nil {
			return entity.Post{}, fmt.Errorf("sync trust score cache: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.Post{}, fmt.Errorf("commit: %w", err)
	}

	return r.GetPost(ctx, userID, postID)
}

func (r *PostRepository) viewerOrigin(ctx context.Context, userID string) (*geoPoint, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}

	var latitude *float64
	var longitude *float64

	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT latitude, longitude
		FROM users
		WHERE id = $1
	`, userID).Scan(&latitude, &longitude)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("get viewer origin: %w", err)
	}

	if latitude == nil || longitude == nil {
		return nil, nil
	}

	return &geoPoint{
		Latitude:  *latitude,
		Longitude: *longitude,
	}, nil
}

func (r *PostRepository) ensurePostExists(ctx context.Context, postID string) error {
	return ensurePostExistsTx(ctx, r.postgres.Pool(), postID)
}

func ensurePostExistsTx(ctx context.Context, queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, postID string) error {
	var exists bool
	if err := queryRower.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM posts
			WHERE id = $1
		)
	`, postID).Scan(&exists); err != nil {
		return fmt.Errorf("ensure post exists: %w", err)
	}
	if !exists {
		return repository.ErrNotFound
	}
	return nil
}

func distanceBoundingBox(latitude, distanceKm float64) (float64, float64) {
	latDelta := distanceKm / 111.32
	cosLatitude := math.Cos(latitude * math.Pi / 180)
	if math.Abs(cosLatitude) < 0.01 {
		cosLatitude = 0.01
	}
	lonDelta := distanceKm / (111.32 * math.Abs(cosLatitude))
	return latDelta, lonDelta
}

func distanceExpression(latitudeArgPos, longitudeArgPos int) string {
	return fmt.Sprintf(`
		6371 * 2 * ASIN(SQRT(
			POWER(SIN(RADIANS((p.latitude - $%d) / 2)), 2) +
			COS(RADIANS($%d)) * COS(RADIANS(p.latitude)) *
			POWER(SIN(RADIANS((p.longitude - $%d) / 2)), 2)
		))
	`, latitudeArgPos, latitudeArgPos, longitudeArgPos)
}

func statusRank(status entity.PostStatus) int {
	if status == entity.PostStatusActive {
		return 0
	}
	return 1
}

// ClaimUntaggedPosts atomically selects up to limit untagged posts (created
// within the last 2 hours) and marks them as claimed by setting ai_tagged_at =
// NOW(). Using FOR UPDATE SKIP LOCKED makes it safe to run multiple worker
// instances without double-processing the same post.
func (r *PostRepository) ClaimUntaggedPosts(ctx context.Context, limit int) ([]entity.Post, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		UPDATE posts
		SET ai_tagged_at = NOW()
		WHERE id IN (
			SELECT id
			FROM posts
			WHERE ai_tagged_at IS NULL
			  AND created_at > NOW() - INTERVAL '2 hours'
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, user_id, kind, title, body, tags, latitude, longitude, location_name
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim untagged posts: %w", err)
	}
	defer rows.Close()

	var posts []entity.Post
	for rows.Next() {
		var p entity.Post
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Kind, &p.Title, &p.Body,
			&p.Tags, &p.Latitude, &p.Longitude, &p.LocationName,
		); err != nil {
			return nil, fmt.Errorf("scan untagged post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// UpdateAITags replaces the stored tags for postID with the merged set
// produced by the AI tagger.
func (r *PostRepository) UpdateAITags(ctx context.Context, postID string, tags []string) error {
	_, err := r.postgres.Pool().Exec(ctx, `
		UPDATE posts SET tags = $2, updated_at = NOW() WHERE id = $1
	`, postID, tags)
	return err
}
