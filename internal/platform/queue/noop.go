package queue

import "context"

// NoopPostQueuePublisher is used in environments where SQS is not configured.
type NoopPostQueuePublisher struct{}

func (NoopPostQueuePublisher) PublishNewPost(context.Context, string) error { return nil }
