// Package discord is a minimal REST client for posting bot messages.
package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
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

// AllowedMentions kiểm soát ai được ping. Với Components V2, role mention nằm
// trong text display component CHỈ ping khi role ID có trong Roles.
type AllowedMentions struct {
	Parse []string `json:"parse,omitempty"` // vd [] để chặn @everyone
	Roles []string `json:"roles,omitempty"`
}

// Message is the outgoing payload.
type Message struct {
	Content         string           `json:"content,omitempty"`
	Embeds          []Embed          `json:"embeds,omitempty"`
	Flags           int              `json:"flags,omitempty"`
	Components      []Component      `json:"components,omitempty"`
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
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

// GetUsername lấy username theo user ID (dùng cho REST backfill, nơi không có
// MESSAGE_CREATE payload). Trả ID nếu lỗi.
func (c *Client) GetUsername(userID string) string {
	body, _, err := c.doAPI("GET", "https://discord.com/api/v10/users/"+userID)
	if err != nil {
		return userID
	}
	var u struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(body, &u) == nil && u.Username != "" {
		return u.Username
	}
	return userID
}

// GetGatewayURL trả URL websocket của Discord gateway (REST /gateway/bot).
func (c *Client) GetGatewayURL() (string, error) {
	body, _, err := c.doAPI("GET", "https://discord.com/api/v10/gateway/bot")
	if err != nil {
		return "", err
	}
	var r struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.URL == "" {
		return "", fmt.Errorf("gateway URL trống")
	}
	return r.URL, nil
}

// ---- Chat streak support (REST-only, không cần gateway intent) ----

// GuildChannel là kênh trong guild (chỉ lấy field cần thiết).
type GuildChannel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   int    `json:"type"` // 0 = text, 5 = announcement, 4/11 = category/thread
	NSFW   bool   `json:"nsfw"`
	Parent string `json:"parent_id"`
}

// GuildMessage là một tin nhắn trong kênh (chỉ lấy field cần thiết).
type GuildMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"` // RFC3339
	Author    struct {
		ID     string `json:"id"`
		Bot    bool   `json:"bot"`
		System bool   `json:"system"`
	} `json:"author"`
}

func (c *Client) doAPI(method, url string) ([]byte, int, error) {
	return c.doAPIWithReason(method, url, "")
}

// newAPIRequest dựng request với header chuẩn + audit-log reason (nếu có).
// Discord yêu cầu reason được URL-encoded trong header X-Audit-Log-Reason.
func (c *Client) newAPIRequest(method, url, reason string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DiscordBot (https://github.com/sang765/TMR-Discord-Bot, v1.5)")
	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", strings.ReplaceAll(neturl.PathEscape(reason), "+", "%2B"))
	}
	return req, nil
}

func (c *Client) doAPIWithReason(method, url, reason string) ([]byte, int, error) {
	req, err := c.newAPIRequest(method, url, reason)
	if err != nil {
		return nil, 0, err
	}

	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			// tạo lại request vì body đã đọc ở lần trước (cần cho PUT role)
			req, err = c.newAPIRequest(method, url, reason)
			if err != nil {
				return nil, 0, err
			}
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, 0, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return body, resp.StatusCode, nil
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
		case resp.StatusCode >= 500:
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		default:
			return body, resp.StatusCode, fmt.Errorf("discord API %s HTTP %d: %.200s", method, resp.StatusCode, body)
		}
	}
	return nil, 0, fmt.Errorf("discord API %s: retry exhausted", method)
}

// ListGuildChannels trả về các kênh text (type 0) của guild.
func (c *Client) ListGuildChannels(guildID string) ([]GuildChannel, error) {
	body, _, err := c.doAPI("GET", "https://discord.com/api/v10/guilds/"+guildID+"/channels")
	if err != nil {
		return nil, err
	}
	var all []GuildChannel
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, err
	}
	var out []GuildChannel
	for _, ch := range all {
		if ch.Type == 0 {
			out = append(out, ch)
		}
	}
	return out, nil
}

// ListMessages lấy tối đa limit tin nhắn của kênh. `before`/`after` là snowflake;
// để trống thì lấy mới nhất. limit <= 0 hoặc > 100 thì dùng 100.
// Trả về theo thứ tự Discord (mới nhất trước).
func (c *Client) ListMessages(channelID, before, after string, limit int) ([]GuildMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages?limit=%d", channelID, limit)
	if before != "" {
		url += "&before=" + before
	}
	if after != "" {
		url += "&after=" + after
	}
	body, _, err := c.doAPI("GET", url)
	if err != nil {
		return nil, err
	}
	var msgs []GuildMessage
	if err := json.Unmarshal(body, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// AddRole gán role cho member. Discord trả 204 No Content khi thành công.
func (c *Client) AddRole(guildID, userID, roleID, reason string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/members/%s/roles/%s", guildID, userID, roleID)
	_, code, err := c.doAPIWithReason("PUT", url, reason)
	if err != nil {
		return err
	}
	if code != 204 && code != 200 {
		return fmt.Errorf("add role HTTP %d", code)
	}
	return nil
}
