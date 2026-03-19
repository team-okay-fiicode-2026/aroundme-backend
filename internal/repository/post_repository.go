package repository

import (
	"context"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type PostRepository interface {
	ListPosts(ctx context.Context, input entity.ListPostsInput) ([]entity.Post, *entity.PostCursor, error)
	GetPost(ctx context.Context, viewerUserID, postID string) (entity.Post, error)
	CreatePost(ctx context.Context, post entity.Post) (entity.Post, error)
	ToggleReaction(ctx context.Context, postID, userID string) (entity.PostReactionState, error)
	ListComments(ctx context.Context, postID string, input entity.ListPostCommentsInput) ([]entity.PostComment, *entity.PostCommentCursor, error)
	CreateComment(ctx context.Context, postID, userID string, parentID *string, body string) (entity.PostComment, int, error)
	ToggleCommentReaction(ctx context.Context, commentID, userID string) (reactionCount int, isReacted bool, err error)
	UpdateStatus(ctx context.Context, postID, userID string, status entity.PostStatus) (entity.Post, error)
	ResolvePost(ctx context.Context, postID, userID string, helperUserIDs []string) (entity.Post, error)
}
