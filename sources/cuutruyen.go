package sources

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CuuTruyen scrapes the team "new-mangas" page on cuutruyen.net (or a mirror).
// The page is often JS-rendered, so direct fetch falls back to r.jina.ai.
type CuuTruyen struct {
	Base    string // https://cuutruyen.net or mirror
	TeamID  string // e.g. "+695"
	Store   SeenStore
	UseJina bool // force jina fallback
}

func (c *CuuTruyen) Name() string { return "Cứu Truyện" }

type ctItem struct {
	mangaID   string
	mangaURL  string
	title     string
	cover     string
	chapID    string
	chapURL   string
	chapLabel string // "C. 8 - 2 ngày trước"
}

// markdown structure from r.jina.ai:
// ### [Title](https://cuutruyen.net/mangas/4760)
// #### [C. 8 - 2 ngày trước](https://cuutruyen.net/mangas/4760/chapters/99590)
var ctMDRe = regexp.MustCompile(
	`(?s)###\s*\[([^\]]+)\]\((https?://[^)]*?/mangas/(\d+))[^)]*\)\s*` +
		`(?:!\[[^\]]*\]\((https?://[^)]+)\)\s*)?` +
		`####\s*\[([^\]]+)\]\((https?://[^)]*?/mangas/\d+/chapters/(\d+))[^)]*\)`)

// html structure (if SSR): <a href=".../mangas/ID">title</a> ... <a href=".../mangas/ID/chapters/N">C. N - time</a>
var ctMangaRe = regexp.MustCompile(`href="(https?://[^"]*?/mangas/(\d+))"[^>]*>([^<]{2,200})<`)
var ctChapterRe = regexp.MustCompile(`href="(https?://[^"]*?/mangas/(\d+)/chapters/(\d+))"[^>]*>([^<]{0,120})<`)

func (c *CuuTruyen) pageURL() string {
	return fmt.Sprintf("%s/t/%s/new-mangas", strings.TrimRight(c.Base, "/"), c.TeamID)
}

func (c *CuuTruyen) getDirect(url string) (string, int, error) {
	req, err := newBrowserRequest("GET", url)
	if err != nil {
		return "", 0, err
	}
	return doRequest(req)
}

func (c *CuuTruyen) getJina(url string) (string, error) {
	return JinaGet(url, "markdown")
}

func (c *CuuTruyen) fetchPage() (string, bool, error) {
	// returns (body, isMarkdown, error)
	if !c.UseJina {
		body, code, err := c.getDirect(c.pageURL())
		if err == nil && code == 200 && strings.Contains(body, "/mangas/") && strings.Contains(body, "chapters/") {
			return body, false, nil
		}
		// fall through to jina
	}
	md, err := c.getJina(c.pageURL())
	return md, true, err
}

var ctImgRe = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)]+)\)`)

func (c *CuuTruyen) parseMarkdown(md string) []ctItem {
	var out []ctItem
	imgs := ctImgRe.FindAllStringSubmatchIndex(md, -1)
	for _, m := range ctMDRe.FindAllStringSubmatchIndex(md, -1) {
		g := ctMDRe.FindStringSubmatch(md[m[0]:m[1]])
		if len(g) < 8 {
			continue
		}
		label := StripTags(g[5])
		// cover: ảnh gần nhất trước heading truyện (ảnh bìa markdown đứng trước ###)
		cover := g[4]
		if cover == "" {
			for _, im := range imgs {
				if im[0] >= m[0] {
					break
				}
				u := md[im[2]:im[3]]
				if strings.Contains(u, "cover") {
					cover = u
				}
			}
		}
		out = append(out, ctItem{
			title:     StripTags(g[1]),
			mangaURL:  g[2],
			mangaID:   g[3],
			cover:     cover,
			chapLabel: label,
			chapURL:   g[6],
			chapID:    g[7],
		})
	}
	return out
}

func (c *CuuTruyen) parseHTML(raw string) []ctItem {
	var out []ctItem
	type mangaAnchor struct {
		pos   int
		id    string
		url   string
		title string
	}
	var mangas []mangaAnchor
	for _, g := range ctMangaRe.FindAllStringSubmatch(raw, -1) {
		mangas = append(mangas, mangaAnchor{
			id:    g[2],
			url:   g[1],
			title: StripTags(g[3]),
		})
	}
	// anchor positions: find each anchor's byte offset
	posRe := regexp.MustCompile(regexp.QuoteMeta("") /* placeholder, positions computed below */)
	_ = posRe
	// simpler: for each chapter match, locate its manga anchor by id nearest before it
	for _, cm := range ctChapterRe.FindAllStringSubmatch(raw, -1) {
		mangaID := cm[2]
		chapPos := strings.Index(raw, cm[1])
		var title, mangaURL string
		for i := len(mangas) - 1; i >= 0; i-- {
			if mangas[i].id != mangaID {
				continue
			}
			pos := strings.Index(raw[:chapPos], mangas[i].url)
			if pos >= 0 {
				mangaURL = mangas[i].url
				title = mangas[i].title
				break
			}
		}
		out = append(out, ctItem{
			title:    title,
			mangaURL: mangaURL,
			mangaID:  mangaID,
			chapURL:  cm[1],
			chapID:   cm[3],
		})
	}
	return out
}

// Fetch implements Fetcher.
func (c *CuuTruyen) Fetch(seed bool) ([]Item, error) {
	body, isMD, err := c.fetchPage()
	if err != nil {
		return nil, err
	}
	var parsed []ctItem
	if isMD {
		parsed = c.parseMarkdown(body)
	} else {
		parsed = c.parseHTML(body)
	}
	if len(parsed) == 0 {
		return nil, fmtErr("không parse được mục cập nhật nào (direct=%v)", !isMD)
	}

	now := time.Now()
	var items []Item

	for _, it := range parsed {
		mangaKey := "ct:m:" + it.mangaID
		chapKey := "ct:c:" + it.chapID

		// chapter label: "C. 8 - 2 ngày trước" / "C. Oneshot - 3 tháng trước"
		label, timePart := it.chapLabel, ""
		if i := strings.Index(label, " - "); i >= 0 {
			label, timePart = strings.TrimSpace(label[:i]), strings.TrimSpace(label[i+3:])
		}
		chLabel := label
		if strings.HasPrefix(strings.ToLower(chLabel), "c. ") {
			chLabel = strings.TrimSpace(chLabel[3:])
		}
		if chLabel != "Oneshot" && chLabel != "" {
			chLabel = "Chương " + chLabel
		}
		pub, _ := ParseRelVN(timePart, now)

		if !c.Store.Seen(mangaKey) {
			c.Store.Mark(mangaKey, now)
			c.Store.Mark(chapKey, now)
			if seed {
				continue
			}
			items = append(items, Item{
				Source: "cuutruyen", Kind: KindNewSeries,
				Title: it.title, URL: it.mangaURL,
				CoverURL:  it.cover,
				Chapter:   chLabel,
				Published: pub,
			})
			continue
		}
		if c.Store.Seen(chapKey) {
			continue
		}
		c.Store.Mark(chapKey, now)
		if seed {
			continue
		}
		items = append(items, Item{
			Source: "cuutruyen", Kind: KindChapter,
			Title: it.title, Chapter: chLabel, URL: it.chapURL,
			CoverURL:  it.cover,
			Published: pub,
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Published.Before(items[b].Published) })
	return items, nil
}
