package sources

import (
	"net/url"
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

// mangaMeta lấy từ trang manga: ảnh bìa (og:image), mô tả (og:description) và
// các nhóm dịch khác (translator cards). Panel "Hoạt động gần đây" không có
// mấy thứ này nên phải fetch thêm 1 request cho mỗi truyện có cập nhật mới.
// og:image thường có dạng CDN#media-fallback=<url thật> — ưu tiên fallback vì
// Discord MediaGallery không xử lý fragment '#' tốt.
var (
	moeOgRe       = regexp.MustCompile(`(?s)<meta[^>]+(?:og:image|twitter:image)[^>]+content="([^"]+)"`)
	moeDescRe     = regexp.MustCompile(`(?s)<meta[^>]+(?:og:description|name="description")[^>]+content="([^"]+)"`)
	moeFallbackRe = regexp.MustCompile(`[#&]media-fallback=([^"&]+)`)
	moeTeamCardRe = regexp.MustCompile(`(?s)<article class="[^"]*manga-translation-team-card[^"]*"[^>]*>(.*?)</article>`)
	moeTeamNameRe = regexp.MustCompile(`class="[^"]*manga-translation-team-card__title[^"]*"[^>]*title="([^"]+)"`)
	moeTeamLinkRe = regexp.MustCompile(`href="(/team/\d+[^"]*)"`)
	moeGenreRe    = regexp.MustCompile(`"genre":\s*\[([^\]]*)\]`)
)

type mangaMeta struct {
	Cover  string
	Desc   string
	Genres []string
	Collab []TranslationTeam
}

func (m *Moe) mangaInfo(mangaPath string) mangaMeta {
	var out mangaMeta
	page := strings.TrimRight(m.Base, "/") + mangaPath
	req, err := newBrowserRequest("GET", page)
	if err != nil {
		return out
	}
	body, code, err := doRequest(req)
	if err != nil || code != 200 {
		return out
	}
	if mg := moeOgRe.FindStringSubmatch(body); len(mg) >= 2 {
		img := mg[1]
		if f := moeFallbackRe.FindStringSubmatch(img); len(f) >= 2 {
			if decoded, err := url.QueryUnescape(f[1]); err == nil {
				img = decoded
			} else {
				img = f[1]
			}
		}
		out.Cover = img
	}
	if d := moeDescRe.FindStringSubmatch(body); len(d) >= 2 {
		out.Desc = strings.TrimSpace(StripTags(d[1]))
	}
	if g := moeGenreRe.FindStringSubmatch(body); len(g) >= 2 {
		for _, raw := range strings.Split(g[1], ",") {
			tag := strings.Trim(strings.TrimSpace(raw), `"'`)
			if tag != "" {
				out.Genres = append(out.Genres, tag)
			}
		}
	}
	// Loại nhóm của mình (theo TeamPath) khỏi danh sách collab.
	selfPath := strings.TrimRight(m.TeamPath, "/")
	for _, card := range moeTeamCardRe.FindAllStringSubmatch(body, -1) {
		name := ""
		if n := moeTeamNameRe.FindStringSubmatch(card[1]); len(n) >= 2 {
			name = n[1]
		}
		link := ""
		if l := moeTeamLinkRe.FindStringSubmatch(card[1]); len(l) >= 2 {
			link = AbsoluteURL(m.Base, l[1])
		}
		if name == "" || link == "" {
			continue
		}
		if strings.HasSuffix(link, selfPath) || strings.HasSuffix(selfPath, strings.TrimPrefix(link, m.Base)) {
			continue // chính nhóm mình — không hiện trong collab
		}
		out.Collab = append(out.Collab, TranslationTeam{Name: name, URL: link})
	}
	return out
}

// Fetch implements Fetcher.
func (m *Moe) Fetch(seed bool) ([]Item, error) {
	url := strings.TrimRight(m.Base, "/") + m.TeamPath
	req, err := newBrowserRequest("GET", url)
	if err != nil {
		return nil, err
	}
	body, code, err := doRequest(req)

	parsed := []moeItem{}
	if err == nil && code == 200 {
		parsed = m.parse(body)
	}

	// direct bị chặn / không có dữ liệu -> thử qua r.jina.ai
	if len(parsed) == 0 {
		if jb, jerr := JinaGet(url, "html"); jerr == nil {
			parsed = m.parse(jb)
		}
	}
	if len(parsed) == 0 {
		// kèm đoạn đầu body để lần sau nhìn log là biết server trả gì
		snippet := strings.TrimSpace(body)
		if len(snippet) > 160 {
			snippet = snippet[:160]
		}
		if err != nil {
			return nil, fmtErr("direct: %v (jina cũng lỗi)", err)
		}
		return nil, fmtErr("HTTP %d, không parse được cập nhật; body đầu: %.160s", code, snippet)
	}

	now := time.Now()
	var items []Item
	emitted := map[string]bool{}
	metaCache := map[string]mangaMeta{}
	metaOf := func(path string) mangaMeta {
		if v, ok := metaCache[path]; ok {
			return v
		}
		v := m.mangaInfo(path)
		metaCache[path] = v
		return v
	}

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
			meta := metaOf(it.mangaPath)
			items = append(items, Item{
				Source: "moe", Kind: KindNewSeries,
				Title:       it.title,
				URL:         AbsoluteURL(m.Base, it.mangaPath),
				CoverURL:    meta.Cover,
				Description: meta.Desc,
				Genres:      meta.Genres,
				Collabs:     meta.Collab,
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
			CoverURL:  metaOf(it.mangaPath).Cover,
			Published: pub,
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Published.Before(items[b].Published) })
	return items, nil
}
