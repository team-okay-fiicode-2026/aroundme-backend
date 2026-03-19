package http

import (
	"sync"

	"github.com/aroundme/aroundme-backend/internal/model"
)

type PostStreamHub = BroadcastHub[model.PostStreamEvent]
type MessageStreamHub = UserHub[model.MessageStreamEvent]
type NotificationStreamHub = UserHub[model.NotificationStreamEvent]

func NewPostStreamHub() *PostStreamHub         { return NewBroadcastHub[model.PostStreamEvent]() }
func NewMessageStreamHub() *MessageStreamHub    { return NewUserHub[model.MessageStreamEvent]() }
func NewNotificationStreamHub() *NotificationStreamHub {
	return NewUserHub[model.NotificationStreamEvent]()
}

// BroadcastHub broadcasts events to all connected clients.
type BroadcastHub[T any] struct {
	mu      sync.RWMutex
	nextID  uint64
	clients map[uint64]chan T
}

func NewBroadcastHub[T any]() *BroadcastHub[T] {
	return &BroadcastHub[T]{clients: make(map[uint64]chan T)}
}

func (h *BroadcastHub[T]) Publish(event T) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *BroadcastHub[T]) Subscribe() (uint64, <-chan T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := h.nextID
	h.nextID++
	ch := make(chan T, 32)
	h.clients[id] = ch
	return id, ch
}

func (h *BroadcastHub[T]) Unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch, exists := h.clients[id]
	if !exists {
		return
	}
	delete(h.clients, id)
	close(ch)
}

// UserHub routes events to subscribers for a specific user ID.
type UserHub[T any] struct {
	mu      sync.RWMutex
	nextID  uint64
	clients map[uint64]userClient[T]
}

type userClient[T any] struct {
	userID string
	ch     chan T
}

func NewUserHub[T any]() *UserHub[T] {
	return &UserHub[T]{clients: make(map[uint64]userClient[T])}
}

func (h *UserHub[T]) PublishToUser(userID string, event T) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		if c.userID != userID {
			continue
		}
		select {
		case c.ch <- event:
		default:
		}
	}
}

func (h *UserHub[T]) Subscribe(userID string) (uint64, <-chan T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := h.nextID
	h.nextID++
	ch := make(chan T, 32)
	h.clients[id] = userClient[T]{userID: userID, ch: ch}
	return id, ch
}

func (h *UserHub[T]) Unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	c, exists := h.clients[id]
	if !exists {
		return
	}
	delete(h.clients, id)
	close(c.ch)
}
