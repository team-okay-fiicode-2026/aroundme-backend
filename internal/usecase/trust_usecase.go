package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

const maxTrustNoteLength = 280

type TrustUseCase interface {
	GetTrustScore(ctx context.Context, userID string) (model.TrustScoreResponse, error)
	CreateInteraction(ctx context.Context, providerUserID string, input model.CreateTrustInteractionInput) (model.TrustInteractionResponse, error)
	RecordFeedback(ctx context.Context, interactionID, recipientUserID string, input model.RecordTrustFeedbackInput) (model.RecordTrustFeedbackResponse, error)
}

type trustUseCase struct {
	trustRepository repository.TrustRepository
}

func NewTrustUseCase(trustRepository repository.TrustRepository) *trustUseCase {
	return &trustUseCase{trustRepository: trustRepository}
}

func (u *trustUseCase) GetTrustScore(ctx context.Context, userID string) (model.TrustScoreResponse, error) {
	score, err := u.trustRepository.GetTrustScore(ctx, strings.TrimSpace(userID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.TrustScoreResponse{}, model.ErrProfileNotFound
		}
		return model.TrustScoreResponse{}, fmt.Errorf("get trust score: %w", err)
	}

	return toTrustScoreResponse(score), nil
}

func (u *trustUseCase) CreateInteraction(ctx context.Context, providerUserID string, input model.CreateTrustInteractionInput) (model.TrustInteractionResponse, error) {
	providerUserID = strings.TrimSpace(providerUserID)
	recipientUserID := strings.TrimSpace(input.RecipientUserID)
	if recipientUserID == "" {
		return model.TrustInteractionResponse{}, model.ValidationError{Message: "recipientUserId is required"}
	}
	if providerUserID == recipientUserID {
		return model.TrustInteractionResponse{}, model.ErrTrustInteractionSelf
	}

	kind, err := normalizeTrustInteractionKind(input.Kind)
	if err != nil {
		return model.TrustInteractionResponse{}, err
	}

	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > maxTrustNoteLength {
		return model.TrustInteractionResponse{}, model.ValidationError{Message: "note must be 280 characters or fewer"}
	}

	var sourcePostID *string
	if trimmed := strings.TrimSpace(input.SourcePostID); trimmed != "" {
		sourcePostID = &trimmed
	}

	interaction, err := u.trustRepository.CreateInteraction(ctx, entity.TrustInteraction{
		ProviderUserID:  providerUserID,
		RecipientUserID: recipientUserID,
		Kind:            kind,
		Status:          entity.TrustInteractionStatusPending,
		SourcePostID:    sourcePostID,
		Note:            note,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicate):
			return model.TrustInteractionResponse{}, model.ValidationError{Message: "a similar trust interaction is already pending"}
		case errors.Is(err, repository.ErrNotFound):
			return model.TrustInteractionResponse{}, model.ErrProfileNotFound
		default:
			return model.TrustInteractionResponse{}, fmt.Errorf("create trust interaction: %w", err)
		}
	}

	return toTrustInteractionResponse(interaction), nil
}

func (u *trustUseCase) RecordFeedback(ctx context.Context, interactionID, recipientUserID string, input model.RecordTrustFeedbackInput) (model.RecordTrustFeedbackResponse, error) {
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > maxTrustNoteLength {
		return model.RecordTrustFeedbackResponse{}, model.ValidationError{Message: "note must be 280 characters or fewer"}
	}

	interaction, score, err := u.trustRepository.RecordInteractionFeedback(ctx, strings.TrimSpace(interactionID), strings.TrimSpace(recipientUserID), input.Positive, note)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return model.RecordTrustFeedbackResponse{}, model.ErrTrustInteractionNotFound
		case errors.Is(err, repository.ErrForbidden):
			return model.RecordTrustFeedbackResponse{}, model.ValidationError{Message: "only the interaction recipient can leave feedback"}
		case errors.Is(err, repository.ErrDuplicate):
			return model.RecordTrustFeedbackResponse{}, model.ErrTrustInteractionFeedbackCompleted
		default:
			return model.RecordTrustFeedbackResponse{}, fmt.Errorf("record trust feedback: %w", err)
		}
	}

	return model.RecordTrustFeedbackResponse{
		Interaction: toTrustInteractionResponse(interaction),
		TrustScore:  toTrustScoreResponse(score),
	}, nil
}

func normalizeTrustInteractionKind(raw string) (entity.TrustInteractionKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(entity.TrustInteractionKindLend):
		return entity.TrustInteractionKindLend, nil
	case string(entity.TrustInteractionKindHelp):
		return entity.TrustInteractionKindHelp, nil
	default:
		return "", model.ValidationError{Message: "kind must be one of lend or help"}
	}
}

func toTrustScoreResponse(score entity.TrustScore) model.TrustScoreResponse {
	breakdown := make([]model.TrustScoreBreakdownResponse, len(score.Breakdown))
	for i, item := range score.Breakdown {
		details := make([]model.TrustScoreBreakdownDetailResponse, len(item.Details))
		for j, detail := range item.Details {
			var createdAt *string
			if detail.CreatedAt != nil {
				value := detail.CreatedAt.UTC().Format(time.RFC3339)
				createdAt = &value
			}
			details[j] = model.TrustScoreBreakdownDetailResponse{
				ID:        detail.ID,
				Title:     detail.Title,
				Subtitle:  detail.Subtitle,
				Count:     detail.Count,
				CreatedAt: createdAt,
			}
		}
		breakdown[i] = model.TrustScoreBreakdownResponse{
			Key:                     item.Key,
			Label:                   item.Label,
			SourceCount:             item.SourceCount,
			Points:                  item.Points,
			Milestone:               item.Milestone,
			RemainingUntilNextPoint: item.RemainingUntilNextPoint,
			Details:                 details,
		}
	}

	return model.TrustScoreResponse{
		UserID:                  score.UserID,
		Score:                   score.Score,
		QualifiedEventCount:     score.QualifiedEventCount,
		PostReactionCount:       score.PostReactionCount,
		CommentReactionCount:    score.CommentReactionCount,
		EventsUntilNextIncrease: score.EventsUntilNextIncrease,
		Breakdown:               breakdown,
	}
}

func toTrustInteractionResponse(interaction entity.TrustInteraction) model.TrustInteractionResponse {
	response := model.TrustInteractionResponse{
		ID:              interaction.ID,
		ProviderUserID:  interaction.ProviderUserID,
		RecipientUserID: interaction.RecipientUserID,
		Kind:            string(interaction.Kind),
		Status:          string(interaction.Status),
		Note:            interaction.Note,
	}
	if interaction.SourcePostID != nil {
		response.SourcePostID = *interaction.SourcePostID
	}
	return response
}
