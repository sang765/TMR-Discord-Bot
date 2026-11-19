package sources

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Moe scrapes truyen.moe team page ("Hoạt động gần đây" panel).
type Moe struct {
	Base     string // https://truyen.moe
	TeamPath string // /team/256/the-mavericks
	Store    SeenStore
}

func (m *Moe) Name() string { return "Mòe Truyện" }

var moeItemRe = regexp.MustCompile(
	`(?s)<a class="team-update-item[^"]*" href="(/manga/[^"]+?/chapters/\d+)">\s*` +
		`<span class="team-update-item__main">\s*<strong>([^<]+)</strong>\s*<span>([^<]*)</span>\s*</span>\s*` +
		`<span class="team-update-item__time">([^<]*)</span>`)

type moeItem struct {
	mangaPath string // /manga/1660-slug
	chapPath  string // /manga/1660-slug/chapters/5
	title     string
	chapLabel string
	timeText  string
}

func (m *Moe) parse(raw string) []moeItem {
	var out []moeItem
	for _, g := range moeItemRe.FindAllStringSubmatch(raw, -1) {
		out = append(out, moeItem{
			mangaPath: chapToManga(g[1]),
			chapPath:  g[1],
			title:     StripTags(g[2]),
			chapLabel: StripTags(g[3]),
			timeText:  StripTags(g[4]),
		})
	}
	return out
}

var moeChapRe = regexp.MustCompile(`^(.*?)/chapters/\d+$`)

func chapToManga(path string) string {
	if g := moeChapRe.FindStringSubmatch(path); g != nil {
		return g[1]
	}
	return path
}

// Fetch implements Fetcher.
func (m *Moe) Fetch(seed bool) ([]Item, error) {
	url := strings.TrimRight(m.Base, "/") + m.TeamPath
	req, err := newRequest("GET", url)
	if err != nil {
		return nil, err
	}
	body, code, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmtErr("HTTP %d cho %s", code, url)
	}
	parsed := m.parse(body)
	if len(parsed) == 0 {
		return nil, fmtErr("không tìm thấy mục cập nhật nào trên trang nhóm")
	}

	now := time.Now()
	var items []Item
	emitted := map[string]bool{}

	for _, it := range parsed {
		mangaKey := "moe:m:" + it.mangaPath
		chapKey := "moe:c:" + it.chapPath
		pub, _ := ParseRelVN(it.timeText, now)

		if !m.Store.Seen(mangaKey) {
			// first time we see this manga -> new series (chapter 1 included)
			m.Store.Mark(mangaKey, now)
			m.Store.Mark(chapKey, now)
			if seed {
				continue
			}
			items = append(items, Item{
				Source: "moe", Kind: KindNewSeries,
				Title: it.title,
				URL:   AbsoluteURL(m.Base, it.mangaPath),
				Chapter: func() string {
					if strings.HasPrefix(it.chapLabel, "Ch ") {
						return "Chương " + strings.TrimSpace(strings.TrimPrefix(it.chapLabel, "Ch "))
					}
					return it.chapLabel
				}(),
				Published: pub,
			})
			continue
		}
		if m.Store.Seen(chapKey) || emitted[chapKey] {
			continue
		}
		m.Store.Mark(chapKey, now)
		if seed {
			continue
		}
		label := it.chapLabel
		if strings.HasPrefix(label, "Ch ") {
			label = "Chương " + strings.TrimSpace(strings.TrimPrefix(label, "Ch "))
		}
		emitted[chapKey] = true
		items = append(items, Item{
			Source: "moe", Kind: KindChapter,
			Title: it.title, Chapter: label,
			URL:       AbsoluteURL(m.Base, it.chapPath),
			Published: pub,
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Published.Before(items[b].Published) })
	return items, nil
}
