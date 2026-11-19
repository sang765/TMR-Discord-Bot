package sources

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// DocLn chặn crawl quá nhanh (429) — giới hạn ~1.7 req/s và retry khi 429.
var (
	doclnMu   sync.Mutex
	doclnLast time.Time
)

const doclnGap = 600 * time.Millisecond

// DocLn scrapes the team page on DocLn / Hako (light novels).
type DocLn struct {
	Base    string // e.g. https://docln.sbs
	GroupID string // e.g. 3474
	Pages   int    // group listing pages to scan
	Store   SeenStore
}

func (d *DocLn) Name() string { return "DocLn" }

type doclnSeries struct {
	ID         string
	URL        string
	Title      string
	LastUpdate time.Time
}

var (
	doclnSeriesRe = regexp.MustCompile(`(?s)<(?:span|h\d)[^>]*class="series-name"[^>]*>\s*<a href="(/truyen/(\d+)[^"]*)"[^>]*>([^<]+)</a>`)
	doclnLastRe   = regexp.MustCompile(`Lần cuối:</span>\s*<span class="status-value">\s*<time class="timeago"[^>]*datetime="([^"]+)"`)
	doclnChapRe   = regexp.MustCompile(`(?s)href="([^"]*?/c(\d+)-[^"]*)"[^>]*?title="([^"]*)"`)
	doclnTimeRe   = regexp.MustCompile(`chapter-time">\s*(\d{2}/\d{2}/\d{4})`)
)

func (d *DocLn) get(url string) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		doclnMu.Lock()
		if wait := doclnGap - time.Since(doclnLast); wait > 0 {
			time.Sleep(wait)
		}
		doclnLast = time.Now()
		doclnMu.Unlock()

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		resp, err := HTTP.Do(req)
		if err != nil {
			return "", err
		}
		if resp.StatusCode == 429 && attempt == 0 {
			ra := resp.Header.Get("Retry-After")
			resp.Body.Close()
			time.Sleep(3 * time.Second)
			if ra != "" {
				var secs int
				if _, err := fmt.Sscanf(ra, "%d", &secs); err == nil && secs > 0 && secs < 60 {
					time.Sleep(time.Duration(secs) * time.Second)
				}
			}
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", fmtErr("HTTP %d cho %s", resp.StatusCode, url)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmtErr("HTTP 429 sau 2 lần thử: %s", url)
}

// fetchSeries lists series on the group page (newest update first).
func (d *DocLn) fetchSeries() ([]doclnSeries, error) {
	pages := d.Pages
	if pages < 1 {
		pages = 2
	}
	var out []doclnSeries
	for p := 1; p <= pages; p++ {
		url := fmt.Sprintf("%s/nhom-dich/%s-the-mavericks?page=%d", d.Base, d.GroupID, p)
		raw, err := d.get(url)
		if err != nil {
			return out, err
		}
		matches := doclnSeriesRe.FindAllStringSubmatchIndex(raw, -1)
		for i, m := range matches {
			// find the matching "Lần cuối" before the next series card
			end := len(raw)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			last := doclnLastRe.FindStringSubmatch(raw[m[0]:end])
			var lu time.Time
			if last != nil {
				if t, err := time.Parse(time.RFC3339, last[1]); err == nil {
					lu = t
				}
			}
			out = append(out, doclnSeries{
				ID:         raw[m[4]:m[5]],
				URL:        AbsoluteURL(d.Base, raw[m[2]:m[3]]),
				Title:      StripTags(raw[m[6]:m[7]]),
				LastUpdate: lu,
			})
		}
		if len(matches) == 0 {
			break
		}
	}
	return out, nil
}

type doclnChapter struct {
	ID    string
	URL   string
	Label string
	Date  time.Time
}

func (d *DocLn) fetchDetail(s doclnSeries) (cover, desc string, chaps []doclnChapter, err error) {
	raw, err := d.get(s.URL)
	if err != nil {
		return "", "", nil, err
	}
	cover = metaContent(raw, "og:image")
	desc = metaContent(raw, "og:description")

	links := doclnChapRe.FindAllStringSubmatchIndex(raw, -1)
	for i, m := range links {
		end := len(raw)
		if i+1 < len(links) {
			end = links[i+1][0]
		}
		tm := doclnTimeRe.FindStringSubmatch(raw[m[0]:end])
		if tm == nil {
			continue
		}
		date, ok := ParseDDMMYYYY(tm[1])
		if !ok {
			continue
		}
		chaps = append(chaps, doclnChapter{
			ID:    raw[m[4]:m[5]],
			URL:   AbsoluteURL(d.Base, raw[m[2]:m[3]]),
			Label: StripTags(raw[m[6]:m[7]]),
			Date:  date,
		})
	}
	// newest last; we emit oldest first
	sort.Slice(chaps, func(a, b int) bool { return chaps[a].Date.Before(chaps[b].Date) })
	return cover, desc, chaps, nil
}

// Fetch implements Fetcher.
func (d *DocLn) Fetch(seed bool) ([]Item, error) {
	series, err := d.fetchSeries()
	if err != nil && len(series) == 0 {
		return nil, err
	}
	now := time.Now()
	var items []Item
	var lastErr error

	for _, s := range series {
		sKey := "docln:s:" + s.ID
		seenSeries := d.Store.Seen(sKey)

		changed := !s.LastUpdate.IsZero() &&
			(!seenSeries || d.storeTimeAfter(sKey, s.LastUpdate))

		if seenSeries && !changed {
			continue
		}
		if seenSeries && !s.LastUpdate.IsZero() && !d.storeTimeAfter(sKey, s.LastUpdate) {
			continue // nothing new for this series
		}

		cover, desc, chaps, derr := d.fetchDetail(s)
		if derr != nil {
			// đánh dấu series để không quét lại mỗi poll, rồi chạy tiếp
			if !seenSeries {
				d.Store.Mark(sKey, now)
			}
			lastErr = fmtErr("%s: %v", s.Title, derr)
			continue
		}

		if !seenSeries {
			if !seed {
				items = append(items, Item{
					Source: "docln", Kind: KindNewSeries,
					Title: s.Title, URL: s.URL,
					CoverURL: cover,
					Subtitle: Truncate(desc, 250),
					// dùng lần cập nhật cuối làm mốc tuổi; series cũ quá sẽ bị filter
					Published: s.LastUpdate,
				})
			}
			d.Store.Mark(sKey, now)
			// seed all current chapters silently
			for _, c := range chaps {
				d.Store.Mark("docln:c:"+c.ID, now)
			}
			if !s.LastUpdate.IsZero() {
				d.Store.Mark(sKey, s.LastUpdate)
			}
			continue
		}

		// existing series with an update: emit unseen chapters (oldest first)
		for _, c := range chaps {
			cKey := "docln:c:" + c.ID
			if d.Store.Seen(cKey) {
				continue
			}
			d.Store.Mark(cKey, now)
			if seed {
				continue
			}
			items = append(items, Item{
				Source: "docln", Kind: KindChapter,
				Title: s.Title, Chapter: c.Label, URL: c.URL,
				CoverURL:  cover,
				Published: c.Date,
			})
		}
		if !s.LastUpdate.IsZero() {
			d.Store.Mark(sKey, s.LastUpdate)
		}
	}
	return items, lastErr
}

func (d *DocLn) storeTimeAfter(key string, t time.Time) bool {
	st, ok := d.Store.SeenAt(key)
	if !ok {
		return true
	}
	return t.After(st)
}

var _ = strings.TrimSpace
