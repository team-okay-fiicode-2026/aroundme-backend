package entity

import (
	"encoding/json"
	"time"
)

type WeatherAlertSeverity string

const (
	WeatherAlertSeverityCritical WeatherAlertSeverity = "critical"
	WeatherAlertSeverityHigh     WeatherAlertSeverity = "high"
	WeatherAlertSeverityModerate WeatherAlertSeverity = "moderate"
	WeatherAlertSeverityLow      WeatherAlertSeverity = "low"
)

type WeatherAlertStatus string

const (
	WeatherAlertStatusActive  WeatherAlertStatus = "active"
	WeatherAlertStatusExpired WeatherAlertStatus = "expired"
)

type WeatherAlert struct {
	ID               string
	Provider         string
	ExternalID       string
	Status           WeatherAlertStatus
	Event            string
	Headline         string
	Description      string
	Instruction      string
	AreaDesc         string
	Severity         WeatherAlertSeverity
	ProviderSeverity string
	Urgency          string
	Certainty        string
	Source           string
	SourceURL        string
	StartsAt         *time.Time
	EndsAt           *time.Time
	ExpiresAt        *time.Time
	Geometry         json.RawMessage
	CenterLatitude   float64
	CenterLongitude  float64
	RadiusKm         float64
	PostID           string
	LastSeenAt       time.Time
	ResolvedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type WeatherAlertUpsertInput struct {
	Provider         string
	ExternalID       string
	Event            string
	Headline         string
	Description      string
	Instruction      string
	AreaDesc         string
	Severity         WeatherAlertSeverity
	ProviderSeverity string
	Urgency          string
	Certainty        string
	Source           string
	SourceURL        string
	StartsAt         *time.Time
	EndsAt           *time.Time
	ExpiresAt        *time.Time
	Geometry         json.RawMessage
	CenterLatitude   float64
	CenterLongitude  float64
	RadiusKm         float64
}

type WeatherAlertUpsertResult struct {
	Alert            WeatherAlert
	Post             Post
	Created          bool
	WasActive        bool
	PreviousSeverity WeatherAlertSeverity
}

type WeatherAlertSyncInput struct {
	Provider   string
	ObservedAt time.Time
	Alerts     []WeatherAlertUpsertInput
}

type WeatherAlertSyncResult struct {
	CreatedCount  int
	UpdatedCount  int
	ResolvedCount int
	NotifiedCount int
}
