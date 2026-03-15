package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type ProfileUseCase interface {
	GetProfile(ctx context.Context, userID string) (entity.Profile, error)
	UpdateProfile(ctx context.Context, userID string, input model.UpdateProfileInput) (entity.Profile, error)
	SetSkills(ctx context.Context, userID string, input model.SetSkillsInput) ([]string, error)
	CreateItem(ctx context.Context, userID string, input model.CreateItemInput) (entity.ProfileItem, error)
	UpdateItem(ctx context.Context, userID, itemID string, input model.UpdateItemInput) (entity.ProfileItem, error)
	DeleteItem(ctx context.Context, userID, itemID string) error
	DeleteAccount(ctx context.Context, userID string) error
}

type profileUseCase struct {
	profileRepository repository.ProfileRepository
}

func NewProfileUseCase(profileRepository repository.ProfileRepository) ProfileUseCase {
	return &profileUseCase{profileRepository: profileRepository}
}

func (u *profileUseCase) GetProfile(ctx context.Context, userID string) (entity.Profile, error) {
	profile, err := u.profileRepository.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return entity.Profile{}, model.ErrProfileNotFound
		}
		return entity.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return profile, nil
}

func (u *profileUseCase) UpdateProfile(ctx context.Context, userID string, input model.UpdateProfileInput) (entity.Profile, error) {
	current, err := u.profileRepository.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return entity.Profile{}, model.ErrProfileNotFound
		}
		return entity.Profile{}, fmt.Errorf("get profile: %w", err)
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return entity.Profile{}, model.ValidationError{Message: "name is required"}
		}
		if len(name) > 120 {
			return entity.Profile{}, model.ValidationError{Message: "name is too long"}
		}
		current.Name = name
	}
	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)
		if len(bio) > 500 {
			return entity.Profile{}, model.ValidationError{Message: "bio must be 500 characters or fewer"}
		}
		current.Bio = bio
	}
	if input.Latitude != nil {
		current.Latitude = input.Latitude
	}
	if input.Longitude != nil {
		current.Longitude = input.Longitude
	}
	if input.NeighborhoodRadiusKm != nil {
		if *input.NeighborhoodRadiusKm < 0.1 || *input.NeighborhoodRadiusKm > 50 {
			return entity.Profile{}, model.ValidationError{Message: "neighborhood radius must be between 0.1 and 50 km"}
		}
		current.NeighborhoodRadiusKm = *input.NeighborhoodRadiusKm
	}
	if input.QuietHoursStart != nil {
		current.QuietHoursStart = input.QuietHoursStart
	}
	if input.QuietHoursEnd != nil {
		current.QuietHoursEnd = input.QuietHoursEnd
	}
	if input.DistanceLimitKm != nil {
		if *input.DistanceLimitKm < 0.5 || *input.DistanceLimitKm > 100 {
			return entity.Profile{}, model.ValidationError{Message: "distance limit must be between 0.5 and 100 km"}
		}
		current.DistanceLimitKm = *input.DistanceLimitKm
	}

	updated, err := u.profileRepository.UpdateProfile(ctx, current)
	if err != nil {
		return entity.Profile{}, fmt.Errorf("update profile: %w", err)
	}

	updated.Skills = current.Skills
	updated.Items = current.Items
	return updated, nil
}

func (u *profileUseCase) SetSkills(ctx context.Context, userID string, input model.SetSkillsInput) ([]string, error) {
	if len(input.Tags) > 20 {
		return nil, model.ValidationError{Message: "maximum 20 skill tags allowed"}
	}

	cleaned := make([]string, 0, len(input.Tags))
	seen := make(map[string]struct{})
	for _, raw := range input.Tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if len(tag) > 50 {
			return nil, model.ValidationError{Message: fmt.Sprintf("skill tag %q is too long", tag)}
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		cleaned = append(cleaned, tag)
	}

	tags, err := u.profileRepository.SetSkills(ctx, userID, cleaned)
	if err != nil {
		return nil, fmt.Errorf("set skills: %w", err)
	}
	return tags, nil
}

func (u *profileUseCase) CreateItem(ctx context.Context, userID string, input model.CreateItemInput) (entity.ProfileItem, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return entity.ProfileItem{}, model.ValidationError{Message: "item name is required"}
	}
	if len(name) > 120 {
		return entity.ProfileItem{}, model.ValidationError{Message: "item name is too long"}
	}

	category := strings.ToLower(strings.TrimSpace(input.Category))
	if category == "" {
		category = "other"
	}

	item, err := u.profileRepository.CreateItem(ctx, entity.ProfileItem{
		UserID:      userID,
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		Category:    category,
	})
	if err != nil {
		return entity.ProfileItem{}, fmt.Errorf("create item: %w", err)
	}
	return item, nil
}

func (u *profileUseCase) UpdateItem(ctx context.Context, userID, itemID string, input model.UpdateItemInput) (entity.ProfileItem, error) {
	profile, err := u.profileRepository.GetProfile(ctx, userID)
	if err != nil {
		return entity.ProfileItem{}, fmt.Errorf("get profile: %w", err)
	}

	var current *entity.ProfileItem
	for i := range profile.Items {
		if profile.Items[i].ID == itemID {
			current = &profile.Items[i]
			break
		}
	}
	if current == nil {
		return entity.ProfileItem{}, model.ErrItemNotFound
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return entity.ProfileItem{}, model.ValidationError{Message: "item name is required"}
		}
		current.Name = name
	}
	if input.Description != nil {
		current.Description = strings.TrimSpace(*input.Description)
	}
	if input.Category != nil {
		current.Category = strings.ToLower(strings.TrimSpace(*input.Category))
	}
	if input.Available != nil {
		current.Available = *input.Available
	}

	updated, err := u.profileRepository.UpdateItem(ctx, *current)
	if err != nil {
		return entity.ProfileItem{}, fmt.Errorf("update item: %w", err)
	}
	return updated, nil
}

func (u *profileUseCase) DeleteItem(ctx context.Context, userID, itemID string) error {
	if err := u.profileRepository.DeleteItem(ctx, userID, itemID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ErrItemNotFound
		}
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (u *profileUseCase) DeleteAccount(ctx context.Context, userID string) error {
	if err := u.profileRepository.DeleteAccount(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ErrProfileNotFound
		}
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}
