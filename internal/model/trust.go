package model

import "errors"

var (
	ErrTrustInteractionSelf              = errors.New("cannot create a trust interaction with yourself")
	ErrTrustInteractionNotFound          = errors.New("trust interaction not found")
	ErrTrustInteractionFeedbackCompleted = errors.New("feedback has already been submitted for this interaction")
)

type CreateTrustInteractionInput struct {
	RecipientUserID string `json:"recipientUserId"`
	Kind            string `json:"kind"`
	SourcePostID    string `json:"sourcePostId"`
	Note            string `json:"note"`
}

type RecordTrustFeedbackInput struct {
	Positive bool   `json:"positive"`
	Note     string `json:"note"`
}

type TrustScoreResponse struct {
	UserID                  string                        `json:"userId"`
	Score                   int                           `json:"score"`
	QualifiedEventCount     int                           `json:"qualifiedEventCount"`
	PostReactionCount       int                           `json:"postReactionCount"`
	CommentReactionCount    int                           `json:"commentReactionCount"`
	EventsUntilNextIncrease int                           `json:"eventsUntilNextIncrease"`
	Breakdown               []TrustScoreBreakdownResponse `json:"breakdown"`
}

type TrustScoreBreakdownResponse struct {
	Key                     string                              `json:"key"`
	Label                   string                              `json:"label"`
	SourceCount             int                                 `json:"sourceCount"`
	Points                  int                                 `json:"points"`
	Milestone               int                                 `json:"milestone"`
	RemainingUntilNextPoint int                                 `json:"remainingUntilNextPoint"`
	Details                 []TrustScoreBreakdownDetailResponse `json:"details"`
}

type TrustScoreBreakdownDetailResponse struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Subtitle  string  `json:"subtitle"`
	Count     int     `json:"count"`
	CreatedAt *string `json:"createdAt,omitempty"`
}

type TrustInteractionResponse struct {
	ID              string `json:"id"`
	ProviderUserID  string `json:"providerUserId"`
	RecipientUserID string `json:"recipientUserId"`
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	SourcePostID    string `json:"sourcePostId,omitempty"`
	Note            string `json:"note,omitempty"`
}

type RecordTrustFeedbackResponse struct {
	Interaction TrustInteractionResponse `json:"interaction"`
	TrustScore  TrustScoreResponse       `json:"trustScore"`
}
