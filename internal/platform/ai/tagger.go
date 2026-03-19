package ai

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

const (
	claudeAPIURL   = "https://api.anthropic.com/v1/messages"
	claudeModel    = "claude-haiku-4-5-20251001"
	anthropicVersion = "2023-06-01"
)

// validLabels is the closed vocabulary the prompt constrains Claude to.
var validLabels = map[string]struct{}{
	"medical": {}, "transport": {}, "childcare": {}, "pets": {},
	"food": {}, "tools": {}, "repair": {}, "power": {},
	"internet": {}, "shelter": {}, "cleanup": {}, "translation": {},
	"moving": {}, "tech": {}, "water": {}, "clothing": {},
	"books": {}, "garden": {}, "sports": {},
}

// Tagger extracts match labels from a post's title and body.
type Tagger interface {
	ExtractTags(ctx context.Context, title, body string) ([]string, error)
}

// ClaudeTagger calls the Claude API with a closed-set classification prompt.
type ClaudeTagger struct {
	apiKey     string
	httpClient *http.Client
}

func NewClaudeTagger(apiKey string) *ClaudeTagger {
	return &ClaudeTagger{
		apiKey: apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (t *ClaudeTagger) ExtractTags(ctx context.Context, title, body string) ([]string, error) {
	prompt := fmt.Sprintf(
		"You are a classifier for a community help app.\n"+
			"Given a post, return ONLY the labels from this list that are relevant:\n"+
			"[medical, transport, childcare, pets, food, tools, repair, power, "+
			"internet, shelter, cleanup, translation, moving, tech, water, clothing, books, garden, sports]\n\n"+
			"Post title: %s\n"+
			"Post body: %s\n\n"+
			"Respond with a JSON array of matching labels only. "+
			"If none match, respond with []. Example: [\"medical\",\"transport\"]",
		title, body,
	)

	payload, err := json.Marshal(claudeRequest{
		Model:     claudeModel,
		MaxTokens: 128,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal claude request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create claude request: %w", err)
	}
	req.Header.Set("x-api-key", t.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call claude api: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read claude response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude api status %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBytes, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal claude response: %w", err)
	}
	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("claude response has no content")
	}

	return parseTagArray(claudeResp.Content[0].Text), nil
}

// parseTagArray extracts a JSON string array from Claude's response text and
// filters it to only labels in validLabels.
func parseTagArray(text string) []string {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start == -1 || end <= start {
		return nil
	}

	var raw []string
	if err := json.Unmarshal([]byte(text[start:end+1]), &raw); err != nil {
		return nil
	}

	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, label := range raw {
		label = strings.ToLower(strings.TrimSpace(label))
		if _, ok := validLabels[label]; !ok {
			continue
		}
		if _, dup := seen[label]; dup {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
