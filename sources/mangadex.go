package sources

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"time"
)

// MangaDex scrapes the group feed via the official API.
type MangaDex struct {
	GroupID string // scanlation group UUID
	Limit   int
	Store   SeenStore
	Langs   []string // translated languages, default ["vi"]
}

func (m *MangaDex) Name() string { return "MangaDex" }

type mdChapter struct {
	ID string `json:"id"`
	Attributes struct {
		Chapter     *string `json:"chapter"`
		Title       string  `json:"title"`
		ExternalURL *string `json:"externalUrl"`
		ReadableAt  time.Time `json:"readableAt"`
	} `json:"attributes"`
	Relationships []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Attributes *struct {
			Title     map[string]string   `json:"title"`
			AltTitles []map[string]string `json:"altTitles"`
		} `json:"attributes"`
	} `json:"relationships"`
}

type mdResp struct {
	Result string     `json:"result"`
	Data   []mdChapter `json:"data"`
}

func (m *MangaDex) pickTitle(t map[string]string, alt []map[string]string) string {
	pick := func(candidates ...string) string {
		for _, lang := range candidates {
			if v, ok := t[lang]; ok && v != "" {
				return v
			}
			for _, a := range alt {
				if v, ok := a[lang]; ok && v != "" {
					return v
				}
			}
		}
		return ""
	}
	langs := m.Langs
	if len(langs) == 0 {
		langs = []string{"vi"}
	}
	for _, l := range langs {
		if s := pick(l); s != "" {
			return s
		}
	}
	if s := pick("en"); s != "" {
		return s
	}
	if s := pick("ja-ro", "ja"); s != "" {
		return s
	}
	for _, v := range t {
		if v != "" {
			return v
		}
	}
	return "Không rõ tiêu đề"
}

// Fetch implements Fetcher.
func (m *MangaDex) Fetch(seed bool) ([]Item, error) {
	limit := m.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	langs := m.Langs
	if len(langs) == 0 {
		langs = []string{"vi"}
	}
	q := url.Values{}
	q.Set("groups[]", m.GroupID)
	q.Set("limit", fmt.Sprint(limit))
	q.Set("order[readableAt]", "desc")
	q.Set("includes[]", "manga")
	for _, l := range langs {
		q.Add("translatedLanguage[]", l)
	}
	for _, cr := range []string{"safe", "suggestive", "erotica"} {
		q.Add("contentRating[]", cr)
	}
	api := "https://api.mangadex.org/chapter?" + q.Encode()

	// API của MangaDex CHẾ request giả browser (challenge) — dùng header API thẳng
	req, _ := newRequest("GET", api)
	body, code, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmtErr("MangaDex API HTTP %d: %.120s", code, body)
	}
	var resp mdResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}

	now := time.Now()
	var items []Item

	// process newest first; emit oldest first at the end
	for i := len(resp.Data) - 1; i >= 0; i-- {
		ch := resp.Data[i]
		chKey := "md:c:" + ch.ID

		var mangaID string
		var mangaAttrs *struct {
			Title     map[string]string   `json:"title"`
			AltTitles []map[string]string `json:"altTitles"`
		}
		var coverFile string
		for _, rel := range ch.Relationships {
			switch rel.Type {
			case "manga":
				mangaID = rel.ID
				mangaAttrs = rel.Attributes
			case "cover_art":
				type coverAttr struct {
					FileName string `json:"fileName"`
				}
				b, _ := json.Marshal(rel.Attributes)
				var ca coverAttr
				_ = json.Unmarshal(b, &ca)
				coverFile = ca.FileName
			}
		}
		if mangaID == "" {
			continue
		}
		mangaKey := "md:m:" + mangaID

		title := m.pickTitle(
			map[string]string{}, nil)
		if mangaAttrs != nil {
			title = m.pickTitle(mangaAttrs.Title, mangaAttrs.AltTitles)
		}
		chLabel := "Oneshot"
		if ch.Attributes.Chapter != nil && *ch.Attributes.Chapter != "" {
			chLabel = "Chương " + *ch.Attributes.Chapter
		}
		if t := ch.Attributes.Title; t != "" {
			chLabel += " — " + t
		}
		chURL := "https://mangadex.org/chapter/" + ch.ID
		if ch.Attributes.ExternalURL != nil && *ch.Attributes.ExternalURL != "" {
			chURL = *ch.Attributes.ExternalURL
		}
		cover := ""
		if coverFile != "" {
			cover = "https://uploads.mangadex.org/covers/" + mangaID + "/" + coverFile + ".512.jpg"
		}

		if !m.Store.Seen(mangaKey) {
			m.Store.Mark(mangaKey, now)
			m.Store.Mark(chKey, now)
			if !seed {
				items = append(items, Item{
					Source: "mangadex", Kind: KindNewSeries,
					Title: title, URL: "https://mangadex.org/title/" + mangaID,
					CoverURL:  cover,
					Chapter:   chLabel,
					Published: ch.Attributes.ReadableAt,
				})
			}
			continue
		}
		if m.Store.Seen(chKey) {
			continue
		}
		m.Store.Mark(chKey, now)
		if seed {
			continue
		}
		items = append(items, Item{
			Source: "mangadex", Kind: KindChapter,
			Title: title, Chapter: chLabel, URL: chURL,
			CoverURL:  cover,
			Published: ch.Attributes.ReadableAt,
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Published.Before(items[b].Published) })
	return items, nil
}
