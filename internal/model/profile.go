package model

import "errors"

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrItemNotFound    = errors.New("item not found")
)

const DeleteAccountConfirmationPhrase = "DELETE MY ACCOUNT"

type UpdateProfileInput struct {
	Name                 *string  `json:"name"`
	Bio                  *string  `json:"bio"`
	Latitude             *float64 `json:"latitude"`
	Longitude            *float64 `json:"longitude"`
	NeighborhoodRadiusKm *float64 `json:"neighborhoodRadiusKm"`
	QuietHoursStart      *string  `json:"quietHoursStart"`
	QuietHoursEnd        *string  `json:"quietHoursEnd"`
	DistanceLimitKm      *float64 `json:"distanceLimitKm"`
}

type SetSkillsInput struct {
	Tags []string `json:"tags"`
}

type CreateItemInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type UpdateItemInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	Available   *bool   `json:"available"`
}

type DeleteAccountInput struct {
	ConfirmationText string `json:"confirmationText"`
	CurrentPassword  string `json:"currentPassword"`
}
