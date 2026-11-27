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

func (m *MangaDex) SeedKey() string { return "md:" + m.GroupID }

type mdChapter struct {
	ID         string `json:"id"`
	Attributes struct {
		Chapter     *string   `json:"chapter"`
		Title       string    `json:"title"`
		ExternalURL *string   `json:"externalUrl"`
		ReadableAt  time.Time `json:"readableAt"`
	} `json:"attributes"`
	Relationships []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes *struct {
			Title     map[string]string   `json:"title"`
			AltTitles []map[string]string `json:"altTitles"`
			Desc      json.RawMessage     `json:"description"` // map (manga) hoặc string (group)
			Tags      json.RawMessage     `json:"tags"`        // manga: []tag
			Name      json.RawMessage     `json:"name"`        // string (group) hoặc map (tag)
		} `json:"attributes"`
	} `json:"relationships"`
}

type mdResp struct {
	Result string      `json:"result"`
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

// apiURL dựng URL feed — dùng chung cho Fetch và Probe để không bị lệch tham số.
func (m *MangaDex) apiURL() string {
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
	q.Add("includes[]", "cover_art")        // cần để có fileName dựng ảnh bìa
	q.Add("includes[]", "scanlation_group") // cần để hiện collab (nhóm dịch khác)
	for _, l := range langs {
		q.Add("translatedLanguage[]", l)
	}
	for _, cr := range []string{"safe", "suggestive", "erotica"} {
		q.Add("contentRating[]", cr)
	}
	return "https://api.mangadex.org/chapter?" + q.Encode()
}

// Fetch implements Fetcher.
func (m *MangaDex) Fetch(seed bool) ([]Item, error) {
	// API của MangaDex CHẾ request giả browser (challenge) — dùng header API thẳng
	req, _ := newRequest("GET", m.apiURL())
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
			Desc      json.RawMessage     `json:"description"`
			Tags      json.RawMessage     `json:"tags"`
			Name      json.RawMessage     `json:"name"`
		}
		var coverFile string
		var groups []TranslationTeam
		var desc string
		var genres []string
		for _, rel := range ch.Relationships {
			switch rel.Type {
			case "manga":
				mangaID = rel.ID
				mangaAttrs = rel.Attributes
				if rel.Attributes != nil {
					desc = mdDescription(rel.Attributes.Desc)
					genres = append(genres, mdTags(rel.Attributes.Tags)...)
				}
			case "scanlation_group":
				if rel.Attributes != nil && rel.ID != m.GroupID {
					gname := mdName(rel.Attributes.Name)
					if gname == "" {
						continue
					}
					groups = append(groups, TranslationTeam{
						Name: gname,
						URL:  "https://mangadex.org/group/" + rel.ID,
					})
				}
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
					Source: "mangadex", Team: TeamName(m.GroupID), Kind: KindNewSeries,
					Title: title, URL: "https://mangadex.org/title/" + mangaID,
					CoverURL:    cover,
					Chapter:     chLabel,
					Description: desc,
					Genres:      genres,
					Collabs:     groups,
					Published:   ch.Attributes.ReadableAt,
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
			Source: "mangadex", Team: TeamName(m.GroupID), Kind: KindChapter,
			Title: title, Chapter: chLabel, URL: chURL,
			CoverURL:  cover,
			Published: ch.Attributes.ReadableAt,
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Published.Before(items[b].Published) })
	return items, nil
}

// mdDescription xử lý field description của MangaDex: manga trả map[lang]text,
// scanlation_group trả string. Chỉ lấy vi rồi en.
func mdDescription(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err == nil {
		if v := m["vi"]; v != "" {
			return StripTags(v)
		}
		return StripTags(m["en"])
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return StripTags(s)
	}
	return ""
}

// mdName xử lý field name: string cho scanlation_group, map[lang]string cho tag.
func mdName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) == nil {
		if v := m["vi"]; v != "" {
			return v
		}
		return m["en"]
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// mdTags parse manga.attributes.tags — mỗi tag có attributes.name map[lang]string.
func mdTags(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []struct {
		Attributes struct {
			Name map[string]string `json:"name"`
		} `json:"attributes"`
	}
	if json.Unmarshal(raw, &tags) != nil {
		return nil
	}
	var out []string
	for _, t := range tags {
		if v := t.Attributes.Name["vi"]; v != "" {
			out = append(out, v)
		} else if v := t.Attributes.Name["en"]; v != "" {
			out = append(out, v)
		}
	}
	return out
}
