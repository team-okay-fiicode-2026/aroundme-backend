package worker

import (
	"context"
	"log"
	"time"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/platform/queue"
)

type NotificationDispatchUseCase interface {
	DispatchIntent(ctx context.Context, intent entity.NotificationIntent) error
}

type NotificationIntentQueue interface {
	Receive(ctx context.Context, maxMessages int32) ([]queue.ReceivedNotificationIntent, error)
	Delete(ctx context.Context, receiptHandle string) error
}

type NotificationDispatchWorker struct {
	queue      NotificationIntentQueue
	dispatcher NotificationDispatchUseCase
}

func NewNotificationDispatchWorker(queue NotificationIntentQueue, dispatcher NotificationDispatchUseCase) *NotificationDispatchWorker {
	return &NotificationDispatchWorker{
		queue:      queue,
		dispatcher: dispatcher,
	}
}

func (w *NotificationDispatchWorker) Run(ctx context.Context) {
	if w == nil || w.queue == nil || w.dispatcher == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		items, err := w.queue.Receive(ctx, 10)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("notification dispatcher: receive intents: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if len(items) == 0 {
			continue
		}

		for _, item := range items {
			intent := entity.NotificationIntent{
				UserID:            item.Intent.UserID,
				Type:              item.Intent.Type,
				Title:             item.Intent.Title,
				Body:              item.Intent.Body,
				EntityID:          item.Intent.EntityID,
				Data:              item.Intent.Data,
				RespectQuietHours: item.Intent.RespectQuietHours,
				DedupeKey:         item.Intent.DedupeKey,
			}
			if err := w.dispatcher.DispatchIntent(ctx, intent); err != nil {
				log.Printf("notification dispatcher: dispatch intent %s: %v", item.MessageID, err)
				continue
			}
			if err := w.queue.Delete(ctx, item.ReceiptHandle); err != nil {
				log.Printf("notification dispatcher: delete intent %s: %v", item.MessageID, err)
			}
		}
	}
}
