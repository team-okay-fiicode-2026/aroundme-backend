package entity

import "time"

type PostKind string

const (
	PostKindEmergency PostKind = "emergency"
	PostKindResource  PostKind = "resource"
	PostKindEvent     PostKind = "event"
)

type PostStatus string

const (
	PostStatusActive   PostStatus = "active"
	PostStatusResolved PostStatus = "resolved"
)

type Post struct {
	ID            string
	UserID        string
	AuthorName    string
	Kind          PostKind
	Status        PostStatus
	Title         string
	Excerpt       string
	Body          string
	LocationName  string
	Latitude      float64
	Longitude     float64
	ShareLocation bool
	ImageURL      string
	Tags          []string
	ReactionCount int
	CommentCount  int
	IsReacted     bool
	DistanceKm    *float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PostCursor struct {
	StatusRank int       `json:"statusRank"`
	CreatedAt  time.Time `json:"createdAt"`
	ID         string    `json:"id"`
}

type PostComment struct {
	ID            string
	PostID        string
	UserID        string
	AuthorName    string
	ParentID      *string
	Body          string
	ReactionCount int
	IsReacted     bool
	ReplyCount    int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PostCommentCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

type PostReactionState struct {
	IsReacted     bool
	ReactionCount int
}

type ListPostsInput struct {
	ViewerUserID string
	AuthorID     *string
	DistanceKm   *float64
	Kind         *PostKind
	Status       *PostStatus
	Cursor       *PostCursor
	Limit        int
}

type ListPostCommentsInput struct {
	Cursor *PostCommentCursor
	Limit  int
}
