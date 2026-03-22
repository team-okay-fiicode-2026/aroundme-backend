package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("expo push API returned %d: %.200s", resp.StatusCode, responseBody)
	}

	if err := validateExpoPushResponse(responseBody); err != nil {
		return err
	}
	return nil
}

type expoPushTicket struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type expoPushResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func validateExpoPushResponse(body []byte) error {
	if len(body) == 0 {
		return nil
	}

	var response expoPushResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decode expo push response: %w", err)
	}
	for _, apiErr := range response.Errors {
		if apiErr.Message != "" {
			return fmt.Errorf("expo push error: %s", apiErr.Message)
		}
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return nil
	}

	var list []expoPushTicket
	if response.Data[0] == '[' {
		if err := json.Unmarshal(response.Data, &list); err != nil {
			return fmt.Errorf("decode expo push tickets: %w", err)
		}
	} else {
		var single expoPushTicket
		if err := json.Unmarshal(response.Data, &single); err != nil {
			return fmt.Errorf("decode expo push ticket: %w", err)
		}
		list = []expoPushTicket{single}
	}

	for _, ticket := range list {
		if ticket.Status == "" || ticket.Status == "ok" {
			continue
		}
		if ticket.Details.Error != "" {
			return fmt.Errorf("expo push ticket error: %s", ticket.Details.Error)
		}
		if ticket.Message != "" {
			return fmt.Errorf("expo push ticket error: %s", ticket.Message)
		}
		return fmt.Errorf("expo push ticket error: status=%s", ticket.Status)
	}

	return nil
}
