// Package sources implements feed scraping for the TMR bot.
package sources

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Kind classifies an update.
type Kind int

const (
	// KindChapter is a new chapter of an existing series.
	KindChapter Kind = iota
	// KindNewSeries is a brand new series by the team.
	KindNewSeries
)

// Item is one scraped update from any source.
type Item struct {
	Source   string // "docln", "mangadex", "moe", "cuutruyen"
	Kind     Kind
	Title    string // series title
	Chapter  string // chapter label e.g. "Chương 58" / "Ch. 5" (empty for new series)
	URL      string // link to the chapter or series
	CoverURL string // optional cover image
	Subtitle string // optional extra text (chapter title, team note...)

	// New-series card: mô tả bộ truyện + các nhóm dịch cùng làm (collab).
	Description string            `json:"description,omitempty"`
	Genres      []string          `json:"genres,omitempty"` // thể loại / tag
	Collabs     []TranslationTeam `json:"collabs,omitempty"` // không tính nhóm mình

	Published time.Time
}

// TranslationTeam là một nhóm dịch khác cùng làm bộ truyện (collab).
type TranslationTeam struct {
	Name string
	URL  string
}

// Fetcher is implemented by each source.
type Fetcher interface {
	Name() string
	// Fetch returns items that are new relative to the store.
	// In seed mode it only records the current state and returns nothing,
	// so the bot does not spam old chapters on its first run.
	Fetch(seed bool) ([]Item, error)
}

// SeenStore is the persistence interface the fetchers need.
type SeenStore interface {
	Seen(key string) bool
	SeenAt(key string) (time.Time, bool)
	Mark(key string, t time.Time)
}

// HTTP client shared by all scrapers.
// Proxy (vd http://127.0.0.1:40000 do warp-cli/wireproxy mở) áp cho mọi nguồn HTML.
var HTTP = &http.Client{Timeout: 45 * time.Second}

// NewTransport tao http.Transport cho bot, co the di qua proxy:
//   - "" : di thang
//   - "http://u:p@host:port" hoac "https://..." : HTTP CONNECT proxy
//   - "socks5://u:p@host:port" : SOCKS5 (dung dialSocks5 trong socks5.go)
func NewTransport(proxyRaw string) (*http.Transport, error) {
	tr := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if proxyRaw == "" {
		return tr, nil
	}
	u, err := url.Parse(proxyRaw)
	if err != nil {
		return nil, fmt.Errorf("proxy khong hop le %q: %w", proxyRaw, err)
	}
	switch u.Scheme {
	case "http", "https":
		tr.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		host := u.Host
		if !strings.Contains(host, ":") {
			host += ":1080"
		}
		user, pass := "", ""
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}
		tr.Proxy = nil
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSocks5(ctx, host, user, pass, addr)
		}
	default:
		return nil, fmt.Errorf("proxy scheme %q khong ho tro (dung http/https/socks5)", u.Scheme)
	}
	return tr, nil
}

// NewDirectTransport tao transport di thang, ép IPv4 để né DNS bị chặn về
// ::1/127.0.0.1 (một số mạng VN poison DNS các trang truyện).
func NewDirectTransport() *http.Transport {
	d := &net.Dialer{Timeout: 20 * time.Second}
	return &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp4", addr)
		},
	}
}
