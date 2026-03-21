package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSPostQueuePublisher publishes post-created events to an SQS queue.
type SQSPostQueuePublisher struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSPostQueuePublisher(ctx context.Context, queueURL string) (*SQSPostQueuePublisher, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SQSPostQueuePublisher{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

type postCreatedMessage struct {
	PostID string `json:"post_id"`
}

func (p *SQSPostQueuePublisher) PublishNewPost(ctx context.Context, postID string) error {
	body, err := json.Marshal(postCreatedMessage{PostID: postID})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}
