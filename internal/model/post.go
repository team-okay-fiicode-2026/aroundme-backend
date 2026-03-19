package model

import (
	"errors"
	"time"
)

var (
	ErrPostNotFound  = errors.New("post not found")
	ErrPostForbidden = errors.New("you are not allowed to modify this post")
)

type PostAuthor struct {
	ID   string
	Name string
}

type PostCoordinates struct {
	Latitude  float64
	Longitude float64
}

type PostSummary struct {
	ID               string
	Title            string
	Excerpt          string
	Kind             string
	Status           string
	Author           PostAuthor
	LocationName     *string
	Coordinates      *PostCoordinates
	IsLocationShared bool
	DistanceKm       *float64
	ReactionCount    int
	CommentCount     int
	IsReacted        bool
	Tags             []string
	ImageURL         string
	CreatedAt        time.Time
}

type PostDetail struct {
	PostSummary
	Body    string
	IsOwner bool
}

type PostComment struct {
	ID            string
	ParentID      *string
	Author        PostAuthor
	Body          string
	ReactionCount int
	IsReacted     bool
	ReplyCount    int
	Replies       []PostComment
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ListPostsInput struct {
	AuthorID   *string
	DistanceKm *float64
	Kind       string
	Status     string
	Cursor     string
	Limit      int
}

type ListPostsResult struct {
	Items      []PostSummary
	NextCursor string
}

type CreatePostInput struct {
	Kind          string
	Title         string
	Body          string
	LocationName  string
	Latitude      float64
	Longitude     float64
	ShareLocation bool
	Tags          []string
	ImageURL      string
}

type TogglePostReactionResult struct {
	PostID        string
	ReactionCount int
	IsReacted     bool
}

type ListPostCommentsInput struct {
	Cursor string
	Limit  int
}

type ListPostCommentsResult struct {
	Items      []PostComment
	NextCursor string
}

type CreatePostCommentInput struct {
	ParentID *string
	Body     string
}

type CreatePostCommentResult struct {
	Comment      PostComment
	CommentCount int
}

type UpdatePostStatusInput struct {
	Status        string   `json:"status"`
	HelperUserIDs []string `json:"helperUserIds"`
}

type PostStreamEvent struct {
	Type      string `json:"type"`
	PostID    string `json:"postId"`
	CommentID string `json:"commentId,omitempty"`
}
