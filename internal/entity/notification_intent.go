package entity

type NotificationIntent struct {
	UserID            string
	Type              string
	Title             string
	Body              string
	EntityID          string
	Data              map[string]string
	RespectQuietHours bool
	DedupeKey         string
}
