package entity

import "time"

type PostKind string

const (
	PostKindEmergency PostKind = "emergency"
	PostKindResource  PostKind = "resource"
	PostKindEvent     PostKind = "event"
)

type PostCategory string

const (
	PostCategoryUncategorized PostCategory = "uncategorized"
	PostCategoryEmergency     PostCategory = "emergency"
	PostCategoryRequest       PostCategory = "request"
	PostCategoryOffer         PostCategory = "offer"
	PostCategoryItem          PostCategory = "item"
	PostCategoryEvent         PostCategory = "event"
)

type PostUrgency string

const (
	PostUrgencyCritical PostUrgency = "critical"
	PostUrgencyHigh     PostUrgency = "high"
	PostUrgencyNormal   PostUrgency = "normal"
)

type PostStatus string

const (
	PostStatusActive   PostStatus = "active"
	PostStatusResolved PostStatus = "resolved"
)

type PostOrigin string

const (
	PostOriginUser         PostOrigin = "user"
	PostOriginWeatherAlert PostOrigin = "weather_alert"
)

type Post struct {
	ID                 string
	UserID             string
	AuthorName         string
	Kind               PostKind
	Category           PostCategory
	Status             PostStatus
	Title              string
	Excerpt            string
	Body               string
	LocationName       string
	Latitude           float64
	Longitude          float64
	ShareLocation      bool
	ImageURL           string
	Tags               []string
	ReactionCount      int
	CommentCount       int
	IsReacted          bool
	DistanceKm         *float64
	Origin             PostOrigin
	ExpiresAt          *time.Time
	VisibilityPriority int
	CreatedAt          time.Time
	UpdatedAt          time.Time

	AIPostType             PostCategory
	AIUrgency              PostUrgency
	AIConfidence           float64
	AIRationale            string
	AIClassificationStatus string

	OverridePostType *PostCategory
	OverrideUrgency  *PostUrgency
	OverriddenAt     *time.Time
}

type PostClassificationInput struct {
	Status     string
	PostType   PostCategory
	Urgency    PostUrgency
	Confidence float64
	Rationale  string
	Tags       []string
}

type PostCursor struct {
	VisibilityPriority int       `json:"visibilityPriority"`
	StatusRank         int       `json:"statusRank"`
	CreatedAt          time.Time `json:"createdAt"`
	ID                 string    `json:"id"`
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
	Category     *PostCategory
	Status       *PostStatus
	Cursor       *PostCursor
	Limit        int
}

type ListPostCommentsInput struct {
	Cursor *PostCommentCursor
	Limit  int
}
