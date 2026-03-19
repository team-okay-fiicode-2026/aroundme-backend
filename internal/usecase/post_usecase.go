package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

const (
	defaultPostListLimit    = 20
	defaultCommentListLimit = 50
	maxCommentBodyLength    = 1000
	maxResolvedHelpers      = 12
	maxPostBodyLength       = 2000
	maxPostExcerptLength    = 160
	maxPostTags             = 8
	maxPostTitleLength      = 120
	maxPostLocationLength   = 160
	postCreatedEventType    = "post.created"
	postUpdatedEventType    = "post.updated"
	commentCreatedEventType = "comment.created"
)

type PostEventPublisher interface {
	Publish(event model.PostStreamEvent)
}

type PostUseCase interface {
	ListPosts(ctx context.Context, userID string, input model.ListPostsInput) (model.ListPostsResult, error)
	GetPost(ctx context.Context, userID, postID string) (model.PostDetail, error)
	CreatePost(ctx context.Context, userID string, input model.CreatePostInput) (model.PostDetail, error)
	ToggleReaction(ctx context.Context, userID, postID string) (model.TogglePostReactionResult, error)
	ListComments(ctx context.Context, userID, postID string, input model.ListPostCommentsInput) (model.ListPostCommentsResult, error)
	CreateComment(ctx context.Context, userID, postID string, input model.CreatePostCommentInput) (model.CreatePostCommentResult, error)
	ToggleCommentReaction(ctx context.Context, userID, commentID string) (reactionCount int, isReacted bool, err error)
	UpdateStatus(ctx context.Context, userID, postID string, input model.UpdatePostStatusInput) (model.PostDetail, error)
}

type noopPostEventPublisher struct{}

func (noopPostEventPublisher) Publish(model.PostStreamEvent) {}

type postUseCase struct {
	postRepository  repository.PostRepository
	trustRepository repository.TrustRepository
	publisher       PostEventPublisher
	notifier        PostNotifier
}

func NewPostUseCase(
	postRepository repository.PostRepository,
	trustRepository repository.TrustRepository,
	publisher PostEventPublisher,
	notifier PostNotifier,
) PostUseCase {
	if publisher == nil {
		publisher = noopPostEventPublisher{}
	}
	if notifier == nil {
		notifier = noopPostNotifier{}
	}

	return &postUseCase{
		postRepository:  postRepository,
		trustRepository: trustRepository,
		publisher:       publisher,
		notifier:        notifier,
	}
}

func (u *postUseCase) ListPosts(ctx context.Context, userID string, input model.ListPostsInput) (model.ListPostsResult, error) {
	var distanceFilter *float64
	if input.DistanceKm != nil {
		if !isAllowedDistance(*input.DistanceKm) {
			return model.ListPostsResult{}, model.ValidationError{Message: "distance filter must be one of 1, 3, or 5 km"}
		}
		distanceFilter = input.DistanceKm
	}

	kind, err := normalizePostKind(input.Kind, true)
	if err != nil {
		return model.ListPostsResult{}, err
	}

	status, err := normalizePostStatus(input.Status, true)
	if err != nil {
		return model.ListPostsResult{}, err
	}

	cursor, err := decodePostCursor(input.Cursor)
	if err != nil {
		return model.ListPostsResult{}, model.ValidationError{Message: "cursor is invalid"}
	}

	limit := normalizeLimit(input.Limit, defaultPostListLimit, 50)

	posts, nextCursor, err := u.postRepository.ListPosts(ctx, entity.ListPostsInput{
		ViewerUserID: userID,
		AuthorID:     input.AuthorID,
		DistanceKm:   distanceFilter,
		Kind:         kind,
		Status:       status,
		Cursor:       cursor,
		Limit:        limit,
	})
	if err != nil {
		return model.ListPostsResult{}, fmt.Errorf("list posts: %w", err)
	}

	result := model.ListPostsResult{
		Items:      make([]model.PostSummary, len(posts)),
		NextCursor: encodePostCursor(nextCursor),
	}
	for i, post := range posts {
		result.Items[i] = toPostSummary(post, userID)
	}

	return result, nil
}

func (u *postUseCase) GetPost(ctx context.Context, userID, postID string) (model.PostDetail, error) {
	post, err := u.postRepository.GetPost(ctx, userID, strings.TrimSpace(postID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.PostDetail{}, model.ErrPostNotFound
		}
		return model.PostDetail{}, fmt.Errorf("get post: %w", err)
	}

	return toPostDetail(post, userID), nil
}

func (u *postUseCase) CreatePost(ctx context.Context, userID string, input model.CreatePostInput) (model.PostDetail, error) {
	kind, err := normalizePostKind(input.Kind, false)
	if err != nil {
		return model.PostDetail{}, err
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return model.PostDetail{}, model.ValidationError{Message: "title is required"}
	}
	if utf8.RuneCountInString(title) > maxPostTitleLength {
		return model.PostDetail{}, model.ValidationError{Message: "title is too long"}
	}

	body := strings.TrimSpace(input.Body)
	if body == "" {
		return model.PostDetail{}, model.ValidationError{Message: "body is required"}
	}
	if utf8.RuneCountInString(body) > maxPostBodyLength {
		return model.PostDetail{}, model.ValidationError{Message: "body must be 2000 characters or fewer"}
	}

	locationName := strings.TrimSpace(input.LocationName)
	if locationName == "" {
		return model.PostDetail{}, model.ValidationError{Message: "location name is required"}
	}
	if utf8.RuneCountInString(locationName) > maxPostLocationLength {
		return model.PostDetail{}, model.ValidationError{Message: "location name is too long"}
	}

	if input.Latitude < -90 || input.Latitude > 90 {
		return model.PostDetail{}, model.ValidationError{Message: "latitude must be between -90 and 90"}
	}
	if input.Longitude < -180 || input.Longitude > 180 {
		return model.PostDetail{}, model.ValidationError{Message: "longitude must be between -180 and 180"}
	}

	tags, err := buildPostMatchTags(input.Tags, title, body)
	if err != nil {
		return model.PostDetail{}, err
	}

	post, err := u.postRepository.CreatePost(ctx, entity.Post{
		UserID:        userID,
		Kind:          *kind,
		Status:        entity.PostStatusActive,
		Title:         title,
		Excerpt:       buildExcerpt(body),
		Body:          body,
		LocationName:  locationName,
		Latitude:      input.Latitude,
		Longitude:     input.Longitude,
		ShareLocation: input.ShareLocation,
		ImageURL:      strings.TrimSpace(input.ImageURL),
		Tags:          tags,
	})
	if err != nil {
		return model.PostDetail{}, fmt.Errorf("create post: %w", err)
	}

	u.publisher.Publish(model.PostStreamEvent{
		Type:   postCreatedEventType,
		PostID: post.ID,
	})

	if post.Kind == entity.PostKindEmergency {
		go u.notifier.NotifyEmergencyPost(context.Background(), post)
	}
	// Skill-match notifications for non-emergency posts are sent by the
	// background AI tagger worker after it enriches the post's tags.

	return toPostDetail(post, userID), nil
}

func (u *postUseCase) ToggleReaction(ctx context.Context, userID, postID string) (model.TogglePostReactionResult, error) {
	state, err := u.postRepository.ToggleReaction(ctx, strings.TrimSpace(postID), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.TogglePostReactionResult{}, model.ErrPostNotFound
		}
		return model.TogglePostReactionResult{}, fmt.Errorf("toggle reaction: %w", err)
	}

	u.publisher.Publish(model.PostStreamEvent{
		Type:   postUpdatedEventType,
		PostID: strings.TrimSpace(postID),
	})

	return model.TogglePostReactionResult{
		PostID:        strings.TrimSpace(postID),
		ReactionCount: state.ReactionCount,
		IsReacted:     state.IsReacted,
	}, nil
}

func (u *postUseCase) ListComments(ctx context.Context, userID, postID string, input model.ListPostCommentsInput) (model.ListPostCommentsResult, error) {
	cursor, err := decodePostCommentCursor(input.Cursor)
	if err != nil {
		return model.ListPostCommentsResult{}, model.ValidationError{Message: "cursor is invalid"}
	}

	comments, nextCursor, err := u.postRepository.ListComments(ctx, strings.TrimSpace(postID), entity.ListPostCommentsInput{
		Cursor: cursor,
		Limit:  normalizeLimit(input.Limit, defaultCommentListLimit, 100),
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ListPostCommentsResult{}, model.ErrPostNotFound
		}
		return model.ListPostCommentsResult{}, fmt.Errorf("list comments: %w", err)
	}

	result := model.ListPostCommentsResult{
		Items:      make([]model.PostComment, len(comments)),
		NextCursor: encodePostCommentCursor(nextCursor),
	}
	for i, comment := range comments {
		result.Items[i] = toPostComment(comment)
	}

	return result, nil
}

func (u *postUseCase) CreateComment(ctx context.Context, userID, postID string, input model.CreatePostCommentInput) (model.CreatePostCommentResult, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return model.CreatePostCommentResult{}, model.ValidationError{Message: "comment body is required"}
	}
	if utf8.RuneCountInString(body) > maxCommentBodyLength {
		return model.CreatePostCommentResult{}, model.ValidationError{Message: "comment body must be 1000 characters or fewer"}
	}

	comment, commentCount, err := u.postRepository.CreateComment(ctx, strings.TrimSpace(postID), userID, input.ParentID, body)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.CreatePostCommentResult{}, model.ErrPostNotFound
		}
		return model.CreatePostCommentResult{}, fmt.Errorf("create comment: %w", err)
	}

	u.publisher.Publish(model.PostStreamEvent{
		Type:      commentCreatedEventType,
		PostID:    strings.TrimSpace(postID),
		CommentID: comment.ID,
	})

	go u.notifier.NotifyNewComment(context.Background(), strings.TrimSpace(postID), userID, comment.AuthorName)

	return model.CreatePostCommentResult{
		Comment:      toPostComment(comment),
		CommentCount: commentCount,
	}, nil
}

func (u *postUseCase) ToggleCommentReaction(ctx context.Context, userID, commentID string) (int, bool, error) {
	count, reacted, err := u.postRepository.ToggleCommentReaction(ctx, commentID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, false, model.ErrPostNotFound
		}
		return 0, false, fmt.Errorf("toggle comment reaction: %w", err)
	}
	return count, reacted, nil
}

func (u *postUseCase) UpdateStatus(ctx context.Context, userID, postID string, input model.UpdatePostStatusInput) (model.PostDetail, error) {
	status, err := normalizePostStatus(input.Status, false)
	if err != nil {
		return model.PostDetail{}, err
	}

	helperUserIDs := normalizeHelperUserIDs(input.HelperUserIDs)
	if len(helperUserIDs) > maxResolvedHelpers {
		return model.PostDetail{}, model.ValidationError{Message: "too many helpers selected"}
	}
	if *status != entity.PostStatusResolved && len(helperUserIDs) > 0 {
		return model.PostDetail{}, model.ValidationError{Message: "helpers can only be credited when resolving a post"}
	}

	var post entity.Post
	if *status == entity.PostStatusResolved && len(helperUserIDs) > 0 {
		post, err = u.postRepository.ResolvePost(ctx, strings.TrimSpace(postID), userID, helperUserIDs)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrNotFound):
				return model.PostDetail{}, model.ErrPostNotFound
			case errors.Is(err, repository.ErrForbidden):
				return model.PostDetail{}, model.ErrPostForbidden
			case errors.Is(err, repository.ErrNoEligibleHelpers):
				return model.PostDetail{}, model.ValidationError{Message: "helpers must be commenters on this post"}
			default:
				return model.PostDetail{}, fmt.Errorf("resolve post: %w", err)
			}
		}
	} else {
		post, err = u.postRepository.UpdateStatus(ctx, strings.TrimSpace(postID), userID, *status)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrNotFound):
				return model.PostDetail{}, model.ErrPostNotFound
			case errors.Is(err, repository.ErrForbidden):
				return model.PostDetail{}, model.ErrPostForbidden
			default:
				return model.PostDetail{}, fmt.Errorf("update post status: %w", err)
			}
		}
	}

	u.publisher.Publish(model.PostStreamEvent{
		Type:   postUpdatedEventType,
		PostID: strings.TrimSpace(postID),
	})

	return toPostDetail(post, userID), nil
}

func normalizeHelperUserIDs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(raw))
	helperUserIDs := make([]string, 0, len(raw))
	for _, helperUserID := range raw {
		trimmed := strings.TrimSpace(helperUserID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		helperUserIDs = append(helperUserIDs, trimmed)
	}

	return helperUserIDs
}

func toPostSummary(post entity.Post, viewerUserID string) model.PostSummary {
	tags := post.Tags
	if tags == nil {
		tags = []string{}
	}

	locationName := &post.LocationName
	coordinates := &model.PostCoordinates{
		Latitude:  post.Latitude,
		Longitude: post.Longitude,
	}
	isLocationShared := post.ShareLocation
	if !post.ShareLocation && post.UserID != viewerUserID {
		locationName = nil
		coordinates = nil
	}

	return model.PostSummary{
		ID:      post.ID,
		Title:   post.Title,
		Excerpt: post.Excerpt,
		Kind:    string(post.Kind),
		Status:  string(post.Status),
		Author: model.PostAuthor{
			ID:   post.UserID,
			Name: post.AuthorName,
		},
		LocationName:     locationName,
		Coordinates:      coordinates,
		IsLocationShared: isLocationShared,
		DistanceKm:       copyDistance(post.DistanceKm),
		ReactionCount:    post.ReactionCount,
		CommentCount:     post.CommentCount,
		IsReacted:        post.IsReacted,
		Tags:             tags,
		ImageURL:         post.ImageURL,
		CreatedAt:        post.CreatedAt,
	}
}

func toPostDetail(post entity.Post, viewerUserID string) model.PostDetail {
	return model.PostDetail{
		PostSummary: toPostSummary(post, viewerUserID),
		Body:        post.Body,
		IsOwner:     post.UserID == viewerUserID,
	}
}

func toPostComment(comment entity.PostComment) model.PostComment {
	return model.PostComment{
		ID:       comment.ID,
		ParentID: comment.ParentID,
		Author: model.PostAuthor{
			ID:   comment.UserID,
			Name: comment.AuthorName,
		},
		Body:          comment.Body,
		ReactionCount: comment.ReactionCount,
		IsReacted:     comment.IsReacted,
		ReplyCount:    comment.ReplyCount,
		CreatedAt:     comment.CreatedAt,
		UpdatedAt:     comment.UpdatedAt,
	}
}

func buildExcerpt(body string) string {
	normalized := strings.Join(strings.Fields(body), " ")
	runes := []rune(normalized)
	if len(runes) <= maxPostExcerptLength {
		return normalized
	}

	return strings.TrimSpace(string(runes[:maxPostExcerptLength])) + "..."
}

func normalizePostKind(raw string, allowEmpty bool) (*entity.PostKind, error) {
	normalized := entity.PostKind(strings.ToLower(strings.TrimSpace(raw)))
	if normalized == "" && allowEmpty {
		return nil, nil
	}

	switch normalized {
	case entity.PostKindEmergency, entity.PostKindResource, entity.PostKindEvent:
		return &normalized, nil
	default:
		return nil, model.ValidationError{Message: "kind must be one of emergency, resource, or event"}
	}
}

func normalizePostStatus(raw string, allowEmpty bool) (*entity.PostStatus, error) {
	normalized := entity.PostStatus(strings.ToLower(strings.TrimSpace(raw)))
	if normalized == "" && allowEmpty {
		return nil, nil
	}

	switch normalized {
	case entity.PostStatusActive, entity.PostStatusResolved:
		return &normalized, nil
	default:
		return nil, model.ValidationError{Message: "status must be active or resolved"}
	}
}

func isAllowedDistance(distanceKm float64) bool {
	switch distanceKm {
	case 1, 3, 5:
		return true
	default:
		return false
	}
}

func normalizeLimit(limit, fallback, max int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

func encodeCursor[T any](cursor *T) string {
	if cursor == nil {
		return ""
	}

	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCursor[T any](raw string, validate func(*T) bool) (*T, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	var cursor T
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, err
	}

	if !validate(&cursor) {
		return nil, errors.New("cursor missing required fields")
	}

	return &cursor, nil
}

func encodePostCursor(cursor *entity.PostCursor) string {
	return encodeCursor(cursor)
}

func decodePostCursor(raw string) (*entity.PostCursor, error) {
	return decodeCursor(raw, func(c *entity.PostCursor) bool {
		return c.ID != "" && !c.CreatedAt.IsZero()
	})
}

func encodePostCommentCursor(cursor *entity.PostCommentCursor) string {
	return encodeCursor(cursor)
}

func decodePostCommentCursor(raw string) (*entity.PostCommentCursor, error) {
	return decodeCursor(raw, func(c *entity.PostCommentCursor) bool {
		return c.ID != "" && !c.CreatedAt.IsZero()
	})
}

func copyDistance(distanceKm *float64) *float64 {
	if distanceKm == nil {
		return nil
	}

	value := *distanceKm
	return &value
}
