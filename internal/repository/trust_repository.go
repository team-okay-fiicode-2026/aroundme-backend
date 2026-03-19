package repository

import (
	"context"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type TrustRepository interface {
	GetTrustScore(ctx context.Context, userID string) (entity.TrustScore, error)
	AcknowledgePostHelpers(ctx context.Context, postID, recipientUserID string, helperUserIDs []string) error
	CreateInteraction(ctx context.Context, interaction entity.TrustInteraction) (entity.TrustInteraction, error)
	RecordInteractionFeedback(ctx context.Context, interactionID, recipientUserID string, positive bool, note string) (entity.TrustInteraction, entity.TrustScore, error)
}
