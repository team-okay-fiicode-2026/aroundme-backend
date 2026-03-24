package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

type NotificationIntentMessage struct {
	UserID            string            `json:"user_id"`
	Type              string            `json:"type"`
	Title             string            `json:"title"`
	Body              string            `json:"body"`
	EntityID          string            `json:"entity_id,omitempty"`
	Data              map[string]string `json:"data,omitempty"`
	RespectQuietHours bool              `json:"respect_quiet_hours"`
	DedupeKey         string            `json:"dedupe_key"`
}

type SQSNotificationIntentQueue struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSNotificationIntentQueue(ctx context.Context, queueURL string) (*SQSNotificationIntentQueue, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SQSNotificationIntentQueue{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

func (q *SQSNotificationIntentQueue) Publish(ctx context.Context, intents []entity.NotificationIntent) error {
	for _, intent := range intents {
		body, err := json.Marshal(NotificationIntentMessage{
			UserID:            intent.UserID,
			Type:              intent.Type,
			Title:             intent.Title,
			Body:              intent.Body,
			EntityID:          intent.EntityID,
			Data:              intent.Data,
			RespectQuietHours: intent.RespectQuietHours,
			DedupeKey:         intent.DedupeKey,
		})
		if err != nil {
			return fmt.Errorf("marshal notification intent: %w", err)
		}
		if _, err := q.client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(q.queueURL),
			MessageBody: aws.String(string(body)),
		}); err != nil {
			return fmt.Errorf("send notification intent: %w", err)
		}
	}
	return nil
}

type ReceivedNotificationIntent struct {
	MessageID     string
	ReceiptHandle string
	Intent        NotificationIntentMessage
}

func (q *SQSNotificationIntentQueue) Receive(ctx context.Context, maxMessages int32) ([]ReceivedNotificationIntent, error) {
	out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     10,
		VisibilityTimeout:   30,
	})
	if err != nil {
		return nil, fmt.Errorf("receive notification intents: %w", err)
	}

	items := make([]ReceivedNotificationIntent, 0, len(out.Messages))
	for _, msg := range out.Messages {
		var intent NotificationIntentMessage
		if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &intent); err != nil {
			return nil, fmt.Errorf("decode notification intent %s: %w", aws.ToString(msg.MessageId), err)
		}
		items = append(items, ReceivedNotificationIntent{
			MessageID:     aws.ToString(msg.MessageId),
			ReceiptHandle: aws.ToString(msg.ReceiptHandle),
			Intent:        intent,
		})
	}

	return items, nil
}

func (q *SQSNotificationIntentQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete notification intent: %w", err)
	}
	return nil
}
