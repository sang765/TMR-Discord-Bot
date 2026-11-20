// TMR Discord Bot — announces new chapters/series of team The Mavericks
// from DocLn, MangaDex, Mòe Truyện and Cứu Truyện.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"tmr-bot/discord"
	"tmr-bot/sources"
	"tmr-bot/store"
)

var version = "1.2.2"

func main() {
	cfgPath := flag.String("config", "config/config.yml", "đường dẫn file config")
	once := flag.Bool("once", false, "chạy một lần poll rồi thoát")
	dry := flag.Bool("dry", false, "in thông báo ra stdout, không gửi Discord")
	forceSeed := flag.Bool("seed", false, "nạp lại dữ liệu nền (không thông báo mục cũ)")
	ver := flag.Bool("version", false, "in version")
	flag.Parse()

	if *ver {
		fmt.Println("tmr-bot", version)
		return
	}

	loadDotEnv(".env")

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.Debug = cfg.Debug || *dry

	st, err := store.Open("data/state.json")
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	if cfg.Proxy != "" {
		tr, perr := sources.NewTransport(cfg.Proxy)
		if perr != nil {
			log.Fatalf("proxy: %v", perr)
		}
		sources.HTTP.Transport = tr
		log.Printf("scrape qua proxy: %s", cfg.Proxy)
	} else {
		sources.HTTP.Transport = sources.NewDirectTransport()
	}
	dc := discord.New(cfg.DiscordToken)

	app := &app{
		cfg: cfg, st: st, dc: dc, dry: *dry,
		fetchers: buildFetchers(cfg),
	}

	if err := app.run(*once, *forceSeed); err != nil {
		log.Fatalf("run: %v", err)
	}
}

func buildFetchers(cfg *Config) []sources.Fetcher {
	var f []sources.Fetcher
	if cfg.EnableDocLn {
		f = append(f, &sources.DocLn{
			Base: cfg.DocLnBase, GroupID: cfg.DocLnGroupID,
			Pages: cfg.DocLnPages, Store: sharedStore{},
		})
	}
	if cfg.EnableMangaDex {
		f = append(f, &sources.MangaDex{
			GroupID: "1a653584-126f-4775-9af8-e06e250ad829",
			Limit:   cfg.MangaDexLimit, Store: sharedStore{},
		})
	}
	if cfg.EnableMoe {
		f = append(f, &sources.Moe{
			Base: cfg.MoeBase, TeamPath: cfg.MoeTeamPath, Store: sharedStore{},
		})
	}
	if cfg.EnableCuuTruyen {
		f = append(f, &sources.CuuTruyen{
			Base: cfg.CuuTruyenBase, TeamID: cfg.CuuTruyenTeamID,
			Store: sharedStore{}, UseJina: cfg.CuuTruyenJina,
		})
	}
	return f
}

// sharedStore adapts *store.Store to sources.SeenStore without import cycle.
var globalStore *store.Store

type sharedStore struct{}

func (sharedStore) Seen(k string) bool                { return globalStore.Seen(k) }
func (sharedStore) SeenAt(k string) (time.Time, bool) { return globalStore.SeenAt(k) }
func (sharedStore) Mark(k string, t time.Time)        { globalStore.Mark(k, t) }

type app struct {
	cfg      *Config
	st       *store.Store
	dc       *discord.Client
	dry      bool
	drySeq   int
	fetchers []sources.Fetcher
}

type pollResult struct {
	source string
	items  []sources.Item
	err    error
}

func (a *app) run(once, forceSeed bool) error {
	globalStore = a.st

	seedMode := forceSeed || !a.st.Seeded()

	log.Printf("tmr-bot %s — %d nguồn, poll mỗi %d phút", version, len(a.fetchers), a.cfg.PollIntervalMin)

	if seedMode {
		if serr := a.seed(); serr != nil {
			// đừng thoát bot — giữ sống, vòng lặp bên dưới sẽ tự seed lại
			log.Printf("seed lần đầu chưa xong: %v — sẽ tự thử lại sau", serr)
		}
		if once {
			_ = a.st.Save("data/state.json")
			return nil
		}
	} else if a.cfg.StatusNotices && !a.dry && a.cfg.Channels.Status != "" {
		msg := fmt.Sprintf("🤖 **TMR Bot online** (v%s) — poll mỗi %d phút. Theo dõi: %s.",
			version, a.cfg.PollIntervalMin, a.sourceList())
		if _, err := a.dc.SendMessage(a.cfg.Channels.Status, discord.Message{Content: msg}); err != nil {
			log.Printf("không gửi được thông báo khởi động: %v", err)
		}
	}

	interval := time.Duration(a.cfg.PollIntervalMin) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}

	if !seedMode {
		if err := a.poll(); err != nil {
			log.Printf("poll: %v", err)
		}
	}
	if once {
		_ = a.st.Save("data/state.json")
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			log.Println("tắt bot, lưu state...")
			_ = a.st.Save("data/state.json")
			return nil
		case <-ticker.C:
			if !a.st.Seeded() {
				// seed lần trước lỗi (nguồn bị chặn...) -> thử lại, xong thì poll luôn
				if serr := a.seed(); serr != nil {
					continue
				}
			}
			if err := a.poll(); err != nil {
				log.Printf("poll: %v", err)
			}
		}
	}
}

func (a *app) sourceList() string {
	var names []string
	for _, f := range a.fetchers {
		names = append(names, f.Name())
	}
	return strings.Join(names, ", ")
}

// seed marks current state without announcing.
// Nguồn nào lỗi thì thử lại ở các lần poll sau; nhưng nếu một nguồn lỗi >= 3 lần
// liên tiếp (vd bị Cloudflare chặn vĩnh viễn từ IP hosting) thì chấp nhận chạy
// thiếu nguồn đó — tránh kẹt trong vòng seed vĩnh viễn, không bao giờ thông báo.
const seedMaxFail = 3

func (a *app) seed() error {
	log.Println("nạp dữ liệu nền (không thông báo mục cũ)...")
	results := a.fetchAll(true)

	pending := 0
	var skipped []string
	for _, r := range results {
		if r.err == nil {
			a.st.ResetSeedFail(r.source)
			continue
		}
		n := a.st.IncSeedFail(r.source)
		if n < seedMaxFail {
			log.Printf("seed %s: %v (lần %d/%d)", r.source, r.err, n, seedMaxFail)
			pending++
		} else {
			log.Printf("seed %s: %v — đã lỗi %d lần, tạm bỏ qua nguồn này", r.source, r.err, n)
			skipped = append(skipped, r.source)
		}
	}
	_ = a.st.Save("data/state.json")

	if pending > 0 {
		log.Printf("seed chưa hoàn tất — sẽ thử lại ở lần poll sau")
		return fmt.Errorf("%d nguồn seed chưa xong", pending)
	}

	a.st.SetSeeded()
	if err := a.st.Save("data/state.json"); err != nil {
		return err
	}
	log.Printf("seed xong: %d mục đã biết, bot bắt đầu theo dõi từ bây giờ", a.st.CountSeen())

	if a.cfg.StatusNotices && !a.dry && a.cfg.Channels.Status != "" {
		msg := fmt.Sprintf("📦 **TMR Bot** (v%s) đã nạp dữ liệu nền lần đầu (%d mục đã biết). Từ giờ chỉ thông báo mục mới thôi!", version, a.st.CountSeen())
		if len(skipped) > 0 {
			msg += fmt.Sprintf("\n⚠️ Không seed được nguồn: **%s** (bị chặn) — bot vẫn chạy với các nguồn còn lại.", strings.Join(skipped, ", "))
		}
		if _, err := a.dc.SendMessage(a.cfg.Channels.Status, discord.Message{Content: msg}); err != nil {
			log.Printf("không gửi được thông báo seed: %v", err)
		}
	}
	return nil
}

func (a *app) fetchAll(seed bool) []pollResult {
	var wg sync.WaitGroup
	out := make([]pollResult, len(a.fetchers))
	for i, f := range a.fetchers {
		wg.Add(1)
		go func(i int, f sources.Fetcher) {
			defer wg.Done()
			items, err := f.Fetch(seed)
			out[i] = pollResult{source: f.Name(), items: items, err: err}
		}(i, f)
	}
	wg.Wait()
	return out
}

func (a *app) poll() error {
	start := time.Now()
	defer a.recoverSkippedSeeds()
	results := a.fetchAll(false)

	var all []sources.Item
	for _, r := range results {
		if r.err != nil {
			log.Printf("[!] %s: %v", r.source, r.err)
			a.maybeReportError(r.source, r.err)
			continue
		}
		log.Printf("[%s] %d mục mới", r.source, len(r.items))
		all = append(all, r.items...)
	}

	// oldest first so the channel reads naturally
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Published.Before(all[j].Published)
	})

	sent := 0
	for _, it := range all {
		if a.cfg.Debug {
			log.Printf("  [%s] %s | %s | %s | pub=%s",
				it.Source, it.Title, it.Chapter, it.URL, it.Published.Format("02/01 15:04"))
		}
		if err := a.announce(it); err != nil {
			log.Printf("announce: %v", err)
			continue
		}
		sent++
	}

	if err := a.st.Save("data/state.json"); err != nil {
		log.Printf("lưu state: %v", err)
	}
	log.Printf("poll xong trong %s — gửi %d thông báo", time.Since(start).Round(time.Second), sent)
	return nil
}

// recoverSkippedSeeds: nguồn nào khi seed từng bị bỏ qua (lỗi >= 3 lần) thì
// ở các poll sau vẫn phải "học nền" trước khi được phép thông báo — nếu không wiping
// state để nó tự seed sẽ flood toàn bộ chapter cũ ngay khi CF thả.
func (a *app) recoverSkippedSeeds() {
	for _, f := range a.fetchers {
		if a.st.SeedFailCount(f.Name()) == 0 {
			continue
		}
		_, err := f.Fetch(true) // seed-mode: chỉ đánh dấu, không phát item
		if err != nil {
			log.Printf("seed lại %s vẫn lỗi: %v", f.Name(), err)
			continue
		}
		a.st.ResetSeedFail(f.Name())
		_ = a.st.Save("data/state.json")
		log.Printf("nguồn %s đã hồi phục — đã nạp nền, bắt đầu thông báo từ giờ", f.Name())
		if a.cfg.StatusNotices && !a.dry && a.cfg.Channels.Status != "" {
			msg := fmt.Sprintf("✅ Nguồn **%s** đã truy cập lại được — đã nạp dữ liệu nền, bắt đầu thông báo mục mới.", f.Name())
			_, _ = a.dc.SendMessage(a.cfg.Channels.Status, discord.Message{Content: msg})
		}
	}
}

func (a *app) maybeReportError(source string, err error) {
	if a.dry || a.cfg.Channels.Status == "" || !a.st.CanPostError(source) {
		return
	}
	msg := fmt.Sprintf("⚠️ Nguồn **%s** poll lỗi: `%v`. Sẽ thử lại ở lần poll sau.", source, err)
	if _, e := a.dc.SendMessage(a.cfg.Channels.Status, discord.Message{Content: msg}); e != nil {
		log.Printf("không gửi được báo lỗi: %v", e)
	}
}

// announce filters and sends one item.
// Khi merge_links bật: nếu truyện+chap này đã được thông báo từ nguồn khác,
// bot EDIT tin nhắn cũ để gắn thêm link nguồn mới (kiểu "cuu - mgd - moe - fb").
func (a *app) announce(it sources.Item) error {
	cutoff := time.Now().Add(-time.Duration(a.cfg.MaxAgeHours) * time.Hour)
	if !it.Published.IsZero() && it.Published.Before(cutoff) {
		return nil // quá cũ, bỏ qua
	}

	dedupKey := sources.NormTitle(it.Title)
	if it.Chapter != "" {
		dedupKey += "|c" + sources.ChapterNorm(it.Chapter)
	}

	var channelID string
	switch {
	case it.Source == "docln" && it.Kind == sources.KindChapter:
		channelID = a.cfg.Channels.NovelChap
	case it.Source == "docln":
		channelID = a.cfg.Channels.NovelNew
	case it.Kind == sources.KindChapter:
		channelID = a.cfg.Channels.MangaChap
	default:
		channelID = a.cfg.Channels.MangaNew
	}
	if channelID == "" {
		return fmt.Errorf("chưa cấu hình kênh cho %s/%v", it.Source, it.Kind)
	}

	msg := buildMessage(a.cfg.RoleID, it)
	srcName := sourceNames[it.Source]

	if a.cfg.MergeLinks {
		if entry, ok := a.st.GetPosted(dedupKey); ok {
			return a.mergeInto(entry, dedupKey, srcName, it.URL, it.Title, it.Chapter)
		}
	}

	// legacy dedup (khi merge_links = false)
	if !a.cfg.MergeLinks {
		if a.st.AnnouncedWithin(dedupKey, time.Duration(a.cfg.DedupHours)*time.Hour) {
			log.Printf("  bỏ qua (trùng nguồn khác): %s %s", it.Title, it.Chapter)
			return nil
		}
		a.st.MarkAnnounced(dedupKey, time.Now())
	}

	if a.dry {
		id := fmt.Sprintf("dry-%d", a.drySeq)
		a.drySeq++
		fmt.Printf("DRY GỬI MỚI → #%s [%s]\n%s\n  embed: %s\n  cover: %s\n\n",
			channelID, id, msg.Content, embedSummary(msg), it.CoverURL)
		a.recordPosted(dedupKey, channelID, id, srcName, it.URL, msg)
		return nil
	}

	id, err := a.dc.SendMessage(channelID, msg)
	if err != nil {
		return err
	}
	a.recordPosted(dedupKey, channelID, id, srcName, it.URL, msg)
	time.Sleep(800 * time.Millisecond) // nhẹ nhàng với rate limit
	return nil
}

func (a *app) recordPosted(dedupKey, channelID, messageID, srcName, url string, msg discord.Message) {
	base, _ := json.Marshal(msg)
	a.st.SetPosted(dedupKey, store.PostedEntry{
		ChannelID: channelID,
		MessageID: messageID,
		Links:     map[string]string{srcName: url},
		Base:      base,
		UpdatedAt: time.Now(),
	})
}

// mergeInto edits the original message to append the new source link.
func (a *app) mergeInto(entry store.PostedEntry, dedupKey, srcName, url, title, chapter string) error {
	if entry.Links == nil {
		entry.Links = map[string]string{}
	}
	if _, seen := entry.Links[srcName]; seen {
		return nil // nguồn này đã có link rồi
	}
	entry.Links[srcName] = url

	var base discord.Message
	if len(entry.Base) > 0 {
		if err := json.Unmarshal(entry.Base, &base); err != nil {
			log.Printf("  merge: không đọc lại tin nhắn gốc: %v", err)
			return nil
		}
	}

	// dòng link: "🔗 Cũng có tại: [MangaDex](...) • [Mòe Truyện](...)"
	parts := make([]string, 0, len(entry.Links))
	for name, link := range entry.Links {
		parts = append(parts, fmt.Sprintf("[%s](%s)", name, link))
	}
	sort.Strings(parts)
	linksLine := "🔗 **Cũng có tại:** " + strings.Join(parts, " • ")

	if len(base.Embeds) > 0 {
		desc := strings.TrimSpace(base.Embeds[0].Description)
		// gỡ dòng linksLine cũ (nếu có) rồi thêm dòng mới
		if i := strings.Index(desc, "🔗 **Cũng có tại:**"); i >= 0 {
			desc = strings.TrimSpace(desc[:i])
		}
		base.Embeds[0].Description = joinNonEmpty("\n\n", desc, linksLine)
	} else {
		base.Content = strings.TrimSpace(base.Content) + "\n\n" + linksLine
	}

	if a.dry {
		fmt.Printf("DRY EDIT → #%s [%s] (%s %s)\n  thêm: %s\n  mô tả: %s\n\n",
			entry.ChannelID, entry.MessageID, title, chapter, linksLine,
			embedSummary(base))
	} else {
		if err := a.dc.PatchMessage(entry.ChannelID, entry.MessageID, base); err != nil {
			log.Printf("  merge vào tin cũ thất bại (%s %s): %v", title, chapter, err)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	a.st.SetPosted(dedupKey, entry)
	return nil
}

var sourceNames = map[string]string{
	"docln":     "DocLn",
	"mangadex":  "MangaDex",
	"moe":       "Mòe Truyện",
	"cuutruyen": "Cứu Truyện",
}

var sourceColors = map[string]int{
	"docln":     0x7C3AED,
	"mangadex":  0xFF67A0,
	"moe":       0xF39C12,
	"cuutruyen": 0x2ECC71,
}

func buildMessage(roleID string, it sources.Item) discord.Message {
	src := sourceNames[it.Source]
	color := sourceColors[it.Source]
	role := discord.MentionRole(roleID)
	ts := ""
	if !it.Published.IsZero() {
		ts = it.Published.UTC().Format(time.RFC3339)
	}

	if it.Kind == sources.KindNewSeries {
		content := fmt.Sprintf("📚 %s — **Truyện mới:** %s!", role, it.Title)
		if role == "" {
			content = fmt.Sprintf("📚 **Truyện mới:** %s!", it.Title)
		}
		emb := discord.Embed{
			Title:       "Đọc " + it.Title + " tại " + src,
			URL:         it.URL,
			Description: it.Subtitle,
			Color:       color,
			Timestamp:   ts,
			Image:       thumbOrNil(it.CoverURL),
			Footer:      &discord.EmbedFooter{Text: "The Mavericks • " + src},
		}
		if it.Chapter != "" {
			emb.Description = joinNonEmpty("\n", it.Chapter, it.Subtitle)
		}
		return discord.Message{Content: content, Embeds: []discord.Embed{emb}}
	}

	content := fmt.Sprintf("📖 %s — **%s** vừa ra %s!", role, it.Title, it.Chapter)
	if role == "" {
		content = fmt.Sprintf("📖 **%s** vừa ra %s!", it.Title, it.Chapter)
	}
	emb := discord.Embed{
		Title:       fmt.Sprintf("Đọc %s tại %s", it.Chapter, src),
		URL:         it.URL,
		Description: it.Subtitle,
		Color:       color,
		Timestamp:   ts,
		Thumbnail:   thumbOrNil(it.CoverURL),
		Footer:      &discord.EmbedFooter{Text: "The Mavericks • " + src},
	}
	return discord.Message{Content: content, Embeds: []discord.Embed{emb}}
}

func thumbOrNil(url string) *discord.EmbedThumb {
	if url == "" {
		return nil
	}
	return &discord.EmbedThumb{URL: url}
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func embedSummary(m discord.Message) string {
	if len(m.Embeds) == 0 {
		return "(none)"
	}
	e := m.Embeds[0]
	return fmt.Sprintf("%s -> %s", e.Title, e.URL)
}

// loadDotEnv reads simple KEY=VALUE lines into env (not overriding existing).
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
