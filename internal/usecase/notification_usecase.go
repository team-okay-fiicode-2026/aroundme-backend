package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

const (
	notifTypeEmergency    = "emergency.nearby"
	notifTypeComment      = "post.comment"
	notifTypeDirectMsg    = "message.direct"
	notifTypeGroupMsg     = "message.group"
	notifTypeSkillMatch   = "post.skill_match"
	emergencyNotifyRadius = 5.0 // km
	notificationListLimit = 50
)

// ─── Interfaces ───────────────────────────────────────────────────────────────

// NotificationUseCase is what the HTTP handler uses.
type NotificationUseCase interface {
	List(ctx context.Context, userID string) (model.ListNotificationsResult, error)
	MarkRead(ctx context.Context, notificationID, userID string) (model.ListNotificationsResult, error)
	MarkAllRead(ctx context.Context, userID string) error
	RegisterPushToken(ctx context.Context, userID, token string) error
}

// PostNotifier is called by the post usecase to trigger notifications.
type PostNotifier interface {
	NotifyEmergencyPost(ctx context.Context, post entity.Post)
	NotifyNewComment(ctx context.Context, postID, commenterID, commenterName string)
	// NotifySkillMatchPost finds nearby users whose skills overlap with the post's
	// tags and sends them a "Hero Alert". Quiet hours suppress the push but the
	// in-app notification is always stored.
	NotifySkillMatchPost(ctx context.Context, post entity.Post)
}

// MessageNotifier is called by the message usecase to trigger notifications.
type MessageNotifier interface {
	NotifyNewMessage(ctx context.Context, conversationID, senderID, senderName, convKind, convName string, recipientIDs []string)
}

// NotificationStreamPublisher pushes real-time events to connected clients.
type NotificationStreamPublisher interface {
	PublishToUser(userID string, event model.NotificationStreamEvent)
}

// PushSender sends Expo push notifications.
type PushSender interface {
	Send(ctx context.Context, tokens []string, title, body string, data any) error
}

// ─── Noop implementations ─────────────────────────────────────────────────────

type noopPostNotifier struct{}

func (noopPostNotifier) NotifyEmergencyPost(context.Context, entity.Post)         {}
func (noopPostNotifier) NotifyNewComment(context.Context, string, string, string) {}
func (noopPostNotifier) NotifySkillMatchPost(context.Context, entity.Post)        {}

type noopMessageNotifier struct{}

func (noopMessageNotifier) NotifyNewMessage(context.Context, string, string, string, string, string, []string) {
}

type noopPushSender struct{}

func (noopPushSender) Send(context.Context, []string, string, string, any) error { return nil }

type noopNotificationStreamPublisher struct{}

func (noopNotificationStreamPublisher) PublishToUser(string, model.NotificationStreamEvent) {}

// ─── Service ──────────────────────────────────────────────────────────────────

type notificationService struct {
	repo      repository.NotificationRepository
	stream    NotificationStreamPublisher
	push      PushSender
}

// NewNotificationService creates a service that implements NotificationUseCase,
// PostNotifier, and MessageNotifier.
func NewNotificationService(
	repo repository.NotificationRepository,
	stream NotificationStreamPublisher,
	push PushSender,
) interface {
	NotificationUseCase
	PostNotifier
	MessageNotifier
} {
	if stream == nil {
		stream = noopNotificationStreamPublisher{}
	}
	if push == nil {
		push = noopPushSender{}
	}
	return &notificationService{repo: repo, stream: stream, push: push}
}

// ─── NotificationUseCase ──────────────────────────────────────────────────────

func (s *notificationService) List(ctx context.Context, userID string) (model.ListNotificationsResult, error) {
	items, unread, err := s.repo.List(ctx, userID, notificationListLimit)
	if err != nil {
		return model.ListNotificationsResult{}, fmt.Errorf("list notifications: %w", err)
	}

	resp := make([]model.NotificationResponse, len(items))
	for i, n := range items {
		resp[i] = toNotificationResponse(n)
	}
	return model.ListNotificationsResult{Items: resp, UnreadCount: unread}, nil
}

func (s *notificationService) MarkRead(ctx context.Context, notificationID, userID string) (model.ListNotificationsResult, error) {
	if err := s.repo.MarkRead(ctx, notificationID, userID); err != nil {
		return model.ListNotificationsResult{}, err
	}
	return s.List(ctx, userID)
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) error {
	if err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return err
	}
	s.stream.PublishToUser(userID, model.NotificationStreamEvent{
		Type:        "notification.read_all",
		UnreadCount: 0,
	})
	return nil
}

func (s *notificationService) RegisterPushToken(ctx context.Context, userID, token string) error {
	return s.repo.UpsertPushToken(ctx, userID, token)
}

// ─── PostNotifier ─────────────────────────────────────────────────────────────

func (s *notificationService) NotifyEmergencyPost(ctx context.Context, post entity.Post) {
	userIDs, err := s.repo.ListNearbyUserIDs(ctx, post.Latitude, post.Longitude, emergencyNotifyRadius, post.UserID)
	if err != nil || len(userIDs) == 0 {
		return
	}

	title := "Emergency nearby"
	body := post.Title
	if post.LocationName != "" {
		body = post.Title + " — " + post.LocationName
	}

	for _, uid := range userIDs {
		n, err := s.repo.Create(ctx, entity.Notification{
			UserID:   uid,
			Type:     notifTypeEmergency,
			Title:    title,
			Body:     body,
			EntityID: post.ID,
		})
		if err != nil {
			continue
		}
		resp := toNotificationResponse(n)
		s.stream.PublishToUser(uid, model.NotificationStreamEvent{
			Type:         "notification.new",
			Notification: &resp,
		})
		s.sendPush(ctx, uid, title, body, map[string]string{"type": notifTypeEmergency, "entityId": post.ID})
	}
}

func (s *notificationService) NotifyNewComment(ctx context.Context, postID, commenterID, commenterName string) {
	authorID, postTitle, err := s.repo.GetPostInfo(ctx, postID)
	if err != nil || authorID == commenterID {
		return
	}

	title := commenterName + " commented on your post"
	body := postTitle

	n, err := s.repo.Create(ctx, entity.Notification{
		UserID:   authorID,
		Type:     notifTypeComment,
		Title:    title,
		Body:     body,
		EntityID: postID,
	})
	if err != nil {
		return
	}
	resp := toNotificationResponse(n)
	s.stream.PublishToUser(authorID, model.NotificationStreamEvent{
		Type:         "notification.new",
		Notification: &resp,
	})
	s.sendPush(ctx, authorID, title, body, map[string]string{"type": notifTypeComment, "entityId": postID})
}

// NotifySkillMatchPost sends a "Hero Alert" to nearby users whose skills or
// available items match the post's tags. The in-app notification is always created; the Expo push is
// skipped only if the recipient is currently in their quiet-hours window.
func (s *notificationService) NotifySkillMatchPost(ctx context.Context, post entity.Post) {
	if len(post.Tags) == 0 {
		return
	}

	users, err := s.repo.ListNearbyUsersForSkillMatch(ctx, post.Latitude, post.Longitude, post.Tags, post.UserID)
	if err != nil || len(users) == 0 {
		return
	}

	title := "A nearby post matches what you offer"
	body := post.Title
	if post.LocationName != "" {
		body = post.Title + " — " + post.LocationName
	}

	for _, u := range users {
		n, err := s.repo.Create(ctx, entity.Notification{
			UserID:   u.UserID,
			Type:     notifTypeSkillMatch,
			Title:    title,
			Body:     body,
			EntityID: post.ID,
		})
		if err != nil {
			continue
		}
		resp := toNotificationResponse(n)
		s.stream.PublishToUser(u.UserID, model.NotificationStreamEvent{
			Type:         "notification.new",
			Notification: &resp,
		})
		// Respect the user's quiet-hours window: save the notification but skip push.
		if !isInQuietHours(u.QuietHoursStart, u.QuietHoursEnd) {
			s.sendPush(ctx, u.UserID, title, body, map[string]string{
				"type":     notifTypeSkillMatch,
				"entityId": post.ID,
			})
		}
	}
}

// isInQuietHours returns true when the current local time falls inside the
// user's quiet-hours window (supports midnight-crossing ranges, e.g. 22:00–07:00).
func isInQuietHours(start, end *string) bool {
	if start == nil || end == nil {
		return false
	}
	now := time.Now()
	current := now.Hour()*60 + now.Minute()

	parseHHMM := func(s string) (int, bool) {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return 0, false
		}
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, false
		}
		return h*60 + m, true
	}

	s, ok1 := parseHHMM(*start)
	e, ok2 := parseHHMM(*end)
	if !ok1 || !ok2 {
		return false
	}

	if s <= e {
		return current >= s && current < e
	}
	// midnight-crossing range (e.g. 22:00–07:00)
	return current >= s || current < e
}

// ─── MessageNotifier ──────────────────────────────────────────────────────────

func (s *notificationService) NotifyNewMessage(ctx context.Context, conversationID, senderID, senderName, convKind, convName string, recipientIDs []string) {
	notifType := notifTypeDirectMsg
	title := senderName + " sent you a message"
	if convKind == "group" {
		notifType = notifTypeGroupMsg
		groupLabel := convName
		if groupLabel == "" {
			groupLabel = "a group"
		}
		title = senderName + " in " + groupLabel
	}

	for _, uid := range recipientIDs {
		n, err := s.repo.Create(ctx, entity.Notification{
			UserID:   uid,
			Type:     notifType,
			Title:    title,
			Body:     "New message",
			EntityID: conversationID,
		})
		if err != nil {
			continue
		}
		resp := toNotificationResponse(n)
		s.stream.PublishToUser(uid, model.NotificationStreamEvent{
			Type:         "notification.new",
			Notification: &resp,
		})
		s.sendPush(ctx, uid, title, "New message", map[string]string{"type": notifType, "entityId": conversationID})
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *notificationService) sendPush(ctx context.Context, userID, title, body string, data any) {
	tokens, err := s.repo.GetPushTokens(ctx, userID)
	if err != nil || len(tokens) == 0 {
		return
	}
	_ = s.push.Send(ctx, tokens, title, body, data)
}

func toNotificationResponse(n entity.Notification) model.NotificationResponse {
	return model.NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Body:      n.Body,
		EntityID:  n.EntityID,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}
