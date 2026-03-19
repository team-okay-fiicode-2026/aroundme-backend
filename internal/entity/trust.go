package entity

import "time"

const (
	TrustInteractionMilestone     = 3
	TrustPostReactionMilestone    = 5
	TrustCommentReactionMilestone = 10
)

type TrustInteractionKind string

const (
	TrustInteractionKindLend TrustInteractionKind = "lend"
	TrustInteractionKindHelp TrustInteractionKind = "help"
)

type TrustInteractionStatus string

const (
	TrustInteractionStatusPending   TrustInteractionStatus = "pending"
	TrustInteractionStatusCompleted TrustInteractionStatus = "completed"
	TrustInteractionStatusCancelled TrustInteractionStatus = "cancelled"
)

type TrustInteraction struct {
	ID               string
	ProviderUserID   string
	RecipientUserID  string
	Kind             TrustInteractionKind
	Status           TrustInteractionStatus
	SourcePostID     *string
	Note             string
	FeedbackNote     string
	PositiveFeedback *bool
	CreatedAt        time.Time
	CompletedAt      *time.Time
	FeedbackAt       *time.Time
}

type TrustScoreBreakdownDetail struct {
	ID        string
	Title     string
	Subtitle  string
	Count     int
	CreatedAt *time.Time
}

type TrustScoreBreakdown struct {
	Key                     string
	Label                   string
	SourceCount             int
	Points                  int
	Milestone               int
	RemainingUntilNextPoint int
	Details                 []TrustScoreBreakdownDetail
}

type TrustScore struct {
	UserID                  string
	Score                   int
	QualifiedEventCount     int
	PostReactionCount       int
	CommentReactionCount    int
	EventsUntilNextIncrease int
	Breakdown               []TrustScoreBreakdown
}

func ComputeTrustScore(qualifiedEventCount int) int {
	if qualifiedEventCount < TrustInteractionMilestone {
		return 0
	}

	return qualifiedEventCount / TrustInteractionMilestone
}

func EventsUntilNextTrustIncrease(qualifiedEventCount int) int {
	return RemainingUntilNextTrustPoint(qualifiedEventCount, TrustInteractionMilestone)
}

func ComputeTotalTrustScore(qualifiedEventCount, postReactionCount, commentReactionCount int) int {
	return ComputeTrustScore(qualifiedEventCount) +
		(postReactionCount / TrustPostReactionMilestone) +
		(commentReactionCount / TrustCommentReactionMilestone)
}

func RemainingUntilNextTrustPoint(count, milestone int) int {
	if milestone <= 0 {
		return 0
	}

	remainder := count % milestone
	if remainder == 0 {
		return milestone
	}

	return milestone - remainder
}
