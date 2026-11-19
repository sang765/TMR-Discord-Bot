// Package discord is a minimal REST client for posting bot messages.
package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client posts messages with a bot token.
type Client struct {
	Token string
	HTTP  *http.Client
}

// New creates a client with sane timeouts.
func New(token string) *Client {
	return &Client{Token: token, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// EmbedThumb is thumbnail/image object.
type EmbedThumb struct {
	URL string `json:"url,omitempty"`
}

// EmbedFooter is the embed footer.
type EmbedFooter struct {
	Text string `json:"text"`
}

// Embed is a rich message embed.
type Embed struct {
	Title       string      `json:"title,omitempty"`
	URL         string      `json:"url,omitempty"`
	Description string      `json:"description,omitempty"`
	Color       int         `json:"color,omitempty"`
	Timestamp   string      `json:"timestamp,omitempty"`
	Thumbnail   *EmbedThumb `json:"thumbnail,omitempty"`
	Image       *EmbedThumb `json:"image,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
}

// Message is the outgoing payload.
type Message struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// SendMessage posts to a channel, returns the message ID.
func (c *Client) SendMessage(channelID string, msg Message) (string, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bot "+c.Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return "", err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			var out struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(body, &out)
			return out.ID, nil
		case resp.StatusCode == 429:
			var rl struct {
				RetryAfter float64 `json:"retry_after"`
			}
			_ = json.Unmarshal(body, &rl)
			if rl.RetryAfter <= 0 {
				rl.RetryAfter = 1
			}
			time.Sleep(time.Duration(rl.RetryAfter*1000) * time.Millisecond)
			continue
		default:
			return "", fmt.Errorf("discord HTTP %d: %.200s", resp.StatusCode, body)
		}
	}
	return "", fmt.Errorf("discord rate limited sau 3 lần thử")
}

// PatchMessage edits an existing message (content + embeds).
func (c *Client) PatchMessage(channelID, messageID string, msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s", channelID, messageID)

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("PATCH", url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bot "+c.Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return nil
		case resp.StatusCode == 429:
			var rl struct {
				RetryAfter float64 `json:"retry_after"`
			}
			_ = json.Unmarshal(body, &rl)
			if rl.RetryAfter <= 0 {
				rl.RetryAfter = 1
			}
			time.Sleep(time.Duration(rl.RetryAfter*1000) * time.Millisecond)
			continue
		default:
			return fmt.Errorf("discord PATCH HTTP %d: %.200s", resp.StatusCode, body)
		}
	}
	return fmt.Errorf("discord rate limited sau 3 lần thử (PATCH)")
}

// MentionRole renders a role mention.
func MentionRole(roleID string) string {
	if roleID == "" {
		return ""
	}
	return "<@&" + roleID + ">"
}
