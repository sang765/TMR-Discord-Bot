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
	Title       string       `json:"title,omitempty"`
	URL         string       `json:"url,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Thumbnail   *EmbedThumb  `json:"thumbnail,omitempty"`
	Image       *EmbedThumb  `json:"image,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
}

// Message is the outgoing payload.
type Message struct {
	Content    string      `json:"content,omitempty"`
	Embeds     []Embed     `json:"embeds,omitempty"`
	Flags      int         `json:"flags,omitempty"`
	Components []Component `json:"components,omitempty"`
}

// FlagComponentsV2 bật chế độ Components V2 (text display / container / button).
const FlagComponentsV2 = 1 << 15

// Các loại component (Components V2).
const (
	CompActionRow    = 1
	CompButton       = 2
	CompSection      = 9
	CompTextDisplay  = 10
	CompThumbnail    = 11
	CompMediaGallery = 12
	CompSeparator    = 14
	CompContainer    = 17
)

// ButtonStyleLink là style nút điều hướng URL.
const ButtonStyleLink = 5

// Component mô tả chung mọi loại component của Components V2
// (các field không dùng sẽ bị omitempty bỏ khỏi JSON).
type Component struct {
	Type        int          `json:"type"`
	Content     string       `json:"content,omitempty"`      // TextDisplay
	Style       int          `json:"style,omitempty"`        // Button
	Label       string       `json:"label,omitempty"`        // Button
	URL         string       `json:"url,omitempty"`          // Button link
	Disabled    bool         `json:"disabled,omitempty"`     // Button
	Items       []MediaItem  `json:"items,omitempty"`        // MediaGallery
	Media       *MediaSource `json:"media,omitempty"`        // Thumbnail
	Accessory   *Component   `json:"accessory,omitempty"`    // Section
	Components  []Component  `json:"components,omitempty"`   // ActionRow/Section/Container
	AccentColor int          `json:"accent_color,omitempty"` // Container
	Spoiler     bool         `json:"spoiler,omitempty"`      // Container/Thumbnail
}

// MediaItem là một ảnh trong MediaGallery.
type MediaItem struct {
	Media MediaSource `json:"media"`
}

// MediaSource là tham chiếu URL media.
type MediaSource struct {
	URL string `json:"url"`
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

// DeleteMessage xoá một tin nhắn của bot.
func (c *Client) DeleteMessage(channelID, messageID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s", channelID, messageID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("discord DELETE HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

// MentionRole renders a role mention.
func MentionRole(roleID string) string {
	if roleID == "" {
		return ""
	}
	return "<@&" + roleID + ">"
}
