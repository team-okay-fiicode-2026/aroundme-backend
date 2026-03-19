package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/database"
	"github.com/aroundme/aroundme-backend/internal/repository"
)

type MessageRepository struct {
	postgres *database.Postgres
}

func NewMessageRepository(postgres *database.Postgres) repository.MessageRepository {
	return &MessageRepository{postgres: postgres}
}

// ─── Friend requests ─────────────────────────────────────────────────────────

func (r *MessageRepository) SendFriendRequest(ctx context.Context, senderID, receiverID, message string) (entity.FriendRequest, error) {
	var req entity.FriendRequest

	err := r.postgres.Pool().QueryRow(ctx, `
		INSERT INTO friend_requests (sender_id, receiver_id, message)
		VALUES ($1, $2, $3)
		ON CONFLICT (sender_id, receiver_id) DO UPDATE
			SET status  = CASE
				WHEN friend_requests.status = 'rejected' THEN 'pending'
				ELSE friend_requests.status
			END,
			message = EXCLUDED.message
		RETURNING id, sender_id, receiver_id, message, status, created_at
	`, senderID, receiverID, message).Scan(&req.ID, &req.SenderID, &req.ReceiverID, &req.Message, &req.Status, &req.CreatedAt)
	if err != nil {
		return entity.FriendRequest{}, fmt.Errorf("send friend request: %w", err)
	}

	// If the request is already accepted, treat as a duplicate rather than silently succeeding
	if req.Status == entity.FriendRequestAccepted {
		return entity.FriendRequest{}, repository.ErrDuplicate
	}

	req.SenderName, _ = r.userName(ctx, senderID)
	req.ReceiverName, _ = r.userName(ctx, receiverID)
	return req, nil
}

func (r *MessageRepository) ListPendingFriendRequests(ctx context.Context, userID string) ([]entity.FriendRequest, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT fr.id, fr.sender_id, u.name, fr.receiver_id, fr.message, fr.status, fr.created_at
		FROM friend_requests fr
		JOIN users u ON u.id = fr.sender_id
		WHERE fr.receiver_id = $1 AND fr.status = 'pending'
		ORDER BY fr.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list friend requests: %w", err)
	}
	defer rows.Close()

	var requests []entity.FriendRequest
	for rows.Next() {
		var req entity.FriendRequest
		if err := rows.Scan(&req.ID, &req.SenderID, &req.SenderName, &req.ReceiverID, &req.Message, &req.Status, &req.CreatedAt); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func (r *MessageRepository) RespondFriendRequest(ctx context.Context, requestID, userID string, accept bool) (entity.FriendRequest, error) {
	status := entity.FriendRequestRejected
	if accept {
		status = entity.FriendRequestAccepted
	}

	var req entity.FriendRequest
	err := r.postgres.Pool().QueryRow(ctx, `
		UPDATE friend_requests
		SET status = $3
		WHERE id = $1 AND receiver_id = $2 AND status = 'pending'
		RETURNING id, sender_id, receiver_id, message, status, created_at
	`, requestID, userID, string(status)).Scan(&req.ID, &req.SenderID, &req.ReceiverID, &req.Message, &req.Status, &req.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.FriendRequest{}, repository.ErrNotFound
		}
		return entity.FriendRequest{}, fmt.Errorf("respond friend request: %w", err)
	}

	return req, nil
}

// ─── Conversations ────────────────────────────────────────────────────────────

func (r *MessageRepository) GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID string) (entity.Conversation, error) {
	// Canonical pair key — same value regardless of which user initiates
	directPair := leastOf(userID, otherUserID) + ":" + greatestOf(userID, otherUserID)

	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.Conversation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Upsert the conversation row — ON CONFLICT eliminates the SELECT-then-INSERT race.
	var conversationID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO conversations (kind, created_by, direct_pair) VALUES ('direct', $1, $2)
		ON CONFLICT (direct_pair) WHERE direct_pair IS NOT NULL
		DO UPDATE SET direct_pair = EXCLUDED.direct_pair
		RETURNING id
	`, userID, directPair).Scan(&conversationID); err != nil {
		return entity.Conversation{}, fmt.Errorf("upsert conversation: %w", err)
	}

	// Add both participants idempotently — safe to call even on the existing conversation
	for _, uid := range []string{userID, otherUserID} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, conversationID, uid); err != nil {
			return entity.Conversation{}, fmt.Errorf("add participant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.Conversation{}, fmt.Errorf("commit: %w", err)
	}

	return r.GetConversation(ctx, conversationID, userID)
}

func leastOf(a, b string) string {
	if a <= b {
		return a
	}
	return b
}

func greatestOf(a, b string) string {
	if a >= b {
		return a
	}
	return b
}

func (r *MessageRepository) CreateGroupConversation(ctx context.Context, creatorID, name string, participantIDs []string) (entity.Conversation, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.Conversation{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var conversationID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO conversations (kind, name, created_by) VALUES ('group', $1, $2)
		RETURNING id
	`, name, creatorID).Scan(&conversationID); err != nil {
		return entity.Conversation{}, fmt.Errorf("create group: %w", err)
	}

	allIDs := append([]string{creatorID}, participantIDs...)
	seen := make(map[string]struct{})
	for _, uid := range allIDs {
		if _, exists := seen[uid]; exists {
			continue
		}
		seen[uid] = struct{}{}
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2)
		`, conversationID, uid); err != nil {
			return entity.Conversation{}, fmt.Errorf("add participant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.Conversation{}, fmt.Errorf("commit: %w", err)
	}

	return r.GetConversation(ctx, conversationID, creatorID)
}

func (r *MessageRepository) GetConversation(ctx context.Context, conversationID, userID string) (entity.Conversation, error) {
	var conv entity.Conversation

	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT c.id, c.kind, COALESCE(c.name, ''), COALESCE(c.created_by::text, ''),
		       c.last_message_at, c.created_at
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id AND cp.user_id = $2
		WHERE c.id = $1
	`, conversationID, userID).Scan(
		&conv.ID, &conv.Kind, &conv.Name, &conv.CreatedBy,
		&conv.LastMessageAt, &conv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Conversation{}, repository.ErrNotFound
		}
		return entity.Conversation{}, fmt.Errorf("get conversation: %w", err)
	}

	participants, err := r.listParticipants(ctx, conversationID)
	if err != nil {
		return entity.Conversation{}, err
	}
	conv.Participants = participants

	lastMessage, err := r.lastMessage(ctx, conversationID)
	if err != nil {
		return entity.Conversation{}, err
	}
	conv.LastMessage = lastMessage

	conv.UnreadCount, err = r.unreadCount(ctx, conversationID, userID)
	if err != nil {
		return entity.Conversation{}, err
	}

	return conv, nil
}

func (r *MessageRepository) ListConversations(ctx context.Context, userID string) ([]entity.Conversation, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT c.id
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id AND cp.user_id = $1
		ORDER BY c.last_message_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	conversations := make([]entity.Conversation, 0, len(ids))
	for _, id := range ids {
		conv, err := r.GetConversation(ctx, id, userID)
		if err != nil {
			continue
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (r *MessageRepository) IsParticipant(ctx context.Context, conversationID, userID string) (bool, error) {
	var exists bool
	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversation_participants
			WHERE conversation_id = $1 AND user_id = $2
		)
	`, conversationID, userID).Scan(&exists)
	return exists, err
}

// ─── Messages ─────────────────────────────────────────────────────────────────

func (r *MessageRepository) SendMessage(ctx context.Context, conversationID, senderID, body, imageURL, locationName string, latitude, longitude *float64) (entity.Message, error) {
	tx, err := r.postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return entity.Message{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var msg entity.Message
	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body, image_url, location_name, latitude, longitude)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7)
		RETURNING id, conversation_id, sender_id, body, COALESCE(image_url, ''), COALESCE(location_name, ''), latitude, longitude, created_at
	`, conversationID, senderID, body, imageURL, locationName, latitude, longitude).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Body, &msg.ImageURL, &msg.LocationName, &msg.Latitude, &msg.Longitude, &msg.CreatedAt,
	); err != nil {
		return entity.Message{}, fmt.Errorf("insert message: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE conversations SET last_message_at = NOW() WHERE id = $1
	`, conversationID); err != nil {
		return entity.Message{}, fmt.Errorf("update last_message_at: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.Message{}, fmt.Errorf("commit: %w", err)
	}

	msg.SenderName, _ = r.userName(ctx, senderID)
	return msg, nil
}

func (r *MessageRepository) ListMessages(ctx context.Context, conversationID string, cursor *entity.MessageCursor, limit int) ([]entity.Message, *entity.MessageCursor, error) {
	var rows pgx.Rows
	var err error

	if cursor == nil {
		rows, err = r.postgres.Pool().Query(ctx, `
			SELECT id, conversation_id, sender_id, body, COALESCE(image_url, ''), COALESCE(location_name, ''), latitude, longitude, created_at,
			       (SELECT name FROM users WHERE id = sender_id)
			FROM messages
			WHERE conversation_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, conversationID, limit+1)
	} else {
		rows, err = r.postgres.Pool().Query(ctx, `
			SELECT id, conversation_id, sender_id, body, COALESCE(image_url, ''), COALESCE(location_name, ''), latitude, longitude, created_at,
			       (SELECT name FROM users WHERE id = sender_id)
			FROM messages
			WHERE conversation_id = $1
			  AND (created_at, id) < ($2, $3)
			ORDER BY created_at DESC
			LIMIT $4
		`, conversationID, cursor.CreatedAt, cursor.ID, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var messages []entity.Message
	for rows.Next() {
		var msg entity.Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Body, &msg.ImageURL, &msg.LocationName, &msg.Latitude, &msg.Longitude, &msg.CreatedAt, &msg.SenderName); err != nil {
			return nil, nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *entity.MessageCursor
	if len(messages) > limit {
		last := messages[limit-1]
		nextCursor = &entity.MessageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		messages = messages[:limit]
	}

	// Return in chronological order for display
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nextCursor, nil
}

func (r *MessageRepository) MarkRead(ctx context.Context, conversationID, userID string) error {
	_, err := r.postgres.Pool().Exec(ctx, `
		UPDATE conversation_participants
		SET last_read_at = NOW()
		WHERE conversation_id = $1 AND user_id = $2
	`, conversationID, userID)
	return err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (r *MessageRepository) listParticipants(ctx context.Context, conversationID string) ([]entity.ConversationParticipant, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT cp.user_id, u.name, COALESCE(u.avatar_url, ''), cp.last_read_at
		FROM conversation_participants cp
		JOIN users u ON u.id = cp.user_id
		WHERE cp.conversation_id = $1
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	defer rows.Close()

	var participants []entity.ConversationParticipant
	for rows.Next() {
		var p entity.ConversationParticipant
		if err := rows.Scan(&p.UserID, &p.Name, &p.AvatarURL, &p.LastReadAt); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (r *MessageRepository) lastMessage(ctx context.Context, conversationID string) (*entity.Message, error) {
	var msg entity.Message
	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id,
		       (SELECT name FROM users WHERE id = m.sender_id),
		       m.body, COALESCE(m.image_url, ''), COALESCE(m.location_name, ''), m.latitude, m.longitude, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1
		ORDER BY m.created_at DESC
		LIMIT 1
	`, conversationID).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.SenderName, &msg.Body, &msg.ImageURL, &msg.LocationName, &msg.Latitude, &msg.Longitude, &msg.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last message: %w", err)
	}
	return &msg, nil
}

func (r *MessageRepository) unreadCount(ctx context.Context, conversationID, userID string) (int, error) {
	var count int
	err := r.postgres.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM messages m
		JOIN conversation_participants cp
		  ON cp.conversation_id = m.conversation_id AND cp.user_id = $2
		WHERE m.conversation_id = $1
		  AND (cp.last_read_at IS NULL OR m.created_at > cp.last_read_at)
		  AND m.sender_id != $2
	`, conversationID, userID).Scan(&count)
	return count, err
}

func (r *MessageRepository) userName(ctx context.Context, userID string) (string, error) {
	var name string
	err := r.postgres.Pool().QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name)
	return name, err
}

func (r *MessageRepository) ListFriends(ctx context.Context, userID string) ([]entity.Friend, error) {
	rows, err := r.postgres.Pool().Query(ctx, `
		SELECT
			CASE WHEN fr.sender_id = $1 THEN fr.receiver_id ELSE fr.sender_id END AS friend_id,
			u.name,
			COALESCE(u.avatar_url, '')
		FROM friend_requests fr
		JOIN users u ON u.id = CASE WHEN fr.sender_id = $1 THEN fr.receiver_id ELSE fr.sender_id END
		WHERE (fr.sender_id = $1 OR fr.receiver_id = $1)
		  AND fr.status = 'accepted'
		ORDER BY u.name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}
	defer rows.Close()

	var friends []entity.Friend
	for rows.Next() {
		var f entity.Friend
		if err := rows.Scan(&f.ID, &f.Name, &f.AvatarURL); err != nil {
			return nil, err
		}
		friends = append(friends, f)
	}
	return friends, rows.Err()
}
