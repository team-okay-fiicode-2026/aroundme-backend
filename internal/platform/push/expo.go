package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"

type ExpoClient struct {
	accessToken string
	httpClient  *http.Client
}

func NewExpoClient(accessToken string) *ExpoClient {
	return &ExpoClient{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

type expoPushMessage struct {
	To    string `json:"to"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Data  any    `json:"data,omitempty"`
}

func (c *ExpoClient) Send(ctx context.Context, tokens []string, title, body string, data any) error {
	if len(tokens) == 0 {
		return nil
	}

	messages := make([]expoPushMessage, 0, len(tokens))
	for _, token := range tokens {
		if !strings.HasPrefix(token, "ExponentPushToken") && !strings.HasPrefix(token, "ExpoPushToken") {
			continue
		}
		messages = append(messages, expoPushMessage{
			To:    token,
			Title: title,
			Body:  body,
			Data:  data,
		})
	}
	if len(messages) == 0 {
		return nil
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoPushURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("expo push API returned %d", resp.StatusCode)
	}

	return nil
}
