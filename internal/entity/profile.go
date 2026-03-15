package entity

type Profile struct {
	ID                   string
	Email                string
	Name                 string
	AvatarURL            string
	Bio                  string
	Latitude             *float64
	Longitude            *float64
	NeighborhoodRadiusKm float64
	QuietHoursStart      *string // "HH:MM"
	QuietHoursEnd        *string
	DistanceLimitKm      float64
	Skills               []string
	Items                []ProfileItem
}

type ProfileItem struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Category    string
	Available   bool
}
