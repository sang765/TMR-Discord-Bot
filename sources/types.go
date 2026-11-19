// Package sources implements feed scraping for the TMR bot.
package sources

import (
	"net/http"
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
	Source    string // "docln", "mangadex", "moe", "cuutruyen"
	Kind      Kind
	Title     string // series title
	Chapter   string // chapter label e.g. "Chương 58" / "Ch. 5" (empty for new series)
	URL       string // link to the chapter or series
	CoverURL  string // optional cover image
	Subtitle  string // optional extra text (chapter title, team note...)
	Published time.Time
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
var HTTP = &http.Client{Timeout: 45 * time.Second}
