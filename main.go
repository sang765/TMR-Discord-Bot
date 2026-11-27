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
	"sync/atomic"
	"syscall"
	"time"

	"tmr-bot/discord"
	"tmr-bot/sources"
	"tmr-bot/store"
)

var version = "1.5.0"

func main() {
	cfgPath := flag.String("config", "config/config.yml", "đường dẫn file config")
	once := flag.Bool("once", false, "chạy một lần poll rồi thoát")
	dry := flag.Bool("dry", false, "in thông báo ra stdout, không gửi Discord")
	forceSeed := flag.Bool("seed", false, "nạp lại dữ liệu nền (không thông báo mục cũ)")
	check := flag.Bool("check", false, "probe cấu trúc 4 nguồn (không đụng state, không gửi Discord)")
	streakOnly := flag.Bool("streak", false, "quét chat streak 1 lần rồi thoát (dùng với --dry để test)")
	gateway := flag.Bool("gateway", false, "chỉ chạy gateway realtime (debug)")
	ver := flag.Bool("version", false, "in version")
	flag.Parse()

	if *ver {
		fmt.Println("tmr-bot", version)
		return
	}
	if *check {
		os.Exit(runCheck(*cfgPath))
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
		st: st, dc: dc, dry: *dry,
		fetchers: buildFetchers(cfg),
		cfgPath:  *cfgPath,
	}
	app.cfg.Store(cfg)
	if st, err := os.Stat(*cfgPath); err == nil {
		app.cfgMtime = st.ModTime()
	}

	if *streakOnly {
		app.runStreakCheck(*dry)
		return
	}
	if *gateway {
		app.runGateway()
		return
	}

	if err := app.run(*once, *forceSeed); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// runCheck chỉ probe cấu trúc các nguồn. Dùng cho auto-task bảo trì: chạy định
// kỳ phát hiện site đổi markup trước khi bot âm thầm không cào được nữa.
// Không mở store, không gửi Discord. Exit 1 nếu có nguồn lỗi.
func runCheck(cfgPath string) int {
	loadDotEnv(".env")
	cfg, err := LoadConfig(cfgPath)
	if err != nil && os.Getenv("DISCORD_TOKEN") == "" {
		// check mode không cần token — chỉ đọc domain overrides từ config.yml
		os.Setenv("DISCORD_TOKEN", "check-only")
		cfg, err = LoadConfig(cfgPath)
	}
	if err != nil {
		log.Printf("config: %v", err)
		return 1
	}
	if cfg.Proxy != "" {
		tr, perr := sources.NewTransport(cfg.Proxy)
		if perr != nil {
			log.Printf("proxy: %v", perr)
			return 1
		}
		sources.HTTP.Transport = tr
	} else {
		sources.HTTP.Transport = sources.NewDirectTransport()
	}

	fetchers := buildFetchers(cfg)
	fmt.Printf("tmr-bot %s — probe cấu trúc %d nguồn\n", version, len(fetchers))
	failed := 0
	for _, f := range fetchers {
		p, ok := f.(interface{ Probe() (int, error) })
		if !ok {
			fmt.Printf("  %-12s SKIP  (fetcher không hỗ trợ probe)\n", f.Name())
			continue
		}
		start := time.Now()
		n, perr := p.Probe()
		status := "OK"
		if perr != nil {
			status, failed = "FAIL", failed+1
		}
		fmt.Printf("  %-12s %-4s %3d mục  %-8s %v\n",
			f.Name(), status, n, time.Since(start).Round(time.Millisecond), perr)
	}
	if failed > 0 {
		fmt.Printf("KẾT LUẬN: %d/%d nguồn lỗi\n", failed, len(fetchers))
		return 1
	}
	fmt.Println("KẾT LUẬN: cả 4 nguồn vẫn parse được")
	return 0
}

func buildFetchers(cfg *Config) []sources.Fetcher {
	var f []sources.Fetcher
	if cfg.EnableDocLn {
		for _, gid := range cfg.DocLnGroupIDs {
			f = append(f, &sources.DocLn{
				Base: cfg.DocLnBase, GroupID: gid,
				Pages: cfg.DocLnPages, Store: sharedStore{},
			})
		}
	}
	if cfg.EnableMangaDex {
		for _, gid := range cfg.MangaDexGroupIDs {
			f = append(f, &sources.MangaDex{
				GroupID: gid, Limit: cfg.MangaDexLimit, Store: sharedStore{},
			})
		}
	}
	if cfg.EnableMoe {
		for _, tp := range cfg.MoeTeamPaths {
			f = append(f, &sources.Moe{
				Base: cfg.MoeBase, TeamPath: tp, Store: sharedStore{},
			})
		}
	}
	if cfg.EnableCuuTruyen {
		for _, tid := range cfg.CuuTruyenTeamIDs {
			f = append(f, &sources.CuuTruyen{
				Base: cfg.CuuTruyenBase, TeamID: tid,
				Store: sharedStore{}, UseJina: cfg.CuuTruyenJina,
			})
		}
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
	cfg    atomic.Pointer[Config] // hot-reload an toàn (không data race)
	st     *store.Store
	dc     *discord.Client
	dry    bool
	drySeq int

	// Hot-reload config: mỗi poll kiểm tra mtime file; nếu đổi thì load lại
	// + rebuild fetchers, không cần restart bot.
	fetchersMu sync.RWMutex
	fetchers   []sources.Fetcher
	cfgPath    string
	cfgMtime   time.Time
}

// maybeReloadConfig kiểm tra mtime config.yml; nếu đổi thì load lại config,
// rebuild fetchers và cập nhật mtime. Chỉ reload cấu hình scrape/đích gửi —
// không đụng token/gateway đang chạy. An toàn vì cfg là atomic.Pointer.
func (a *app) maybeReloadConfig() {
	if a.cfgPath == "" {
		return
	}
	st, err := os.Stat(a.cfgPath)
	if err != nil || !st.ModTime().After(a.cfgMtime) {
		return
	}
	cfg, err := LoadConfig(a.cfgPath)
	if err != nil {
		log.Printf("reload config thất bại (giữ config cũ): %v", err)
		return
	}
	newFetchers := buildFetchers(cfg)
	a.fetchersMu.Lock()
	a.fetchers = newFetchers
	a.cfgMtime = st.ModTime()
	a.fetchersMu.Unlock()
	a.cfg.Store(cfg)
	log.Printf("config changed — reloaded %d fetchers, poll mỗi %d phút", len(newFetchers), cfg.PollIntervalMin)
}

// currentCfg trả về config hiện tại (atomic load).
func (a *app) currentCfg() *Config { return a.cfg.Load() }

// currentFetchers trả về slice fetcher hiện tại (đọc an toàn).
func (a *app) currentFetchers() []sources.Fetcher {
	a.fetchersMu.RLock()
	defer a.fetchersMu.RUnlock()
	return a.fetchers
}

type pollResult struct {
	source string
	items  []sources.Item
	err    error
}

func (a *app) run(once, forceSeed bool) error {
	globalStore = a.st

	seedMode := forceSeed || !a.st.Seeded()

	log.Printf("tmr-bot %s — %d nguồn, poll mỗi %d phút", version, len(a.fetchers), a.currentCfg().PollIntervalMin)

	if seedMode {
		if serr := a.seed(); serr != nil {
			// đừng thoát bot — giữ sống, vòng lặp bên dưới sẽ tự seed lại
			log.Printf("seed lần đầu chưa xong: %v — sẽ tự thử lại sau", serr)
		}
		if once {
			_ = a.st.Save("data/state.json")
			return nil
		}
	} else if a.currentCfg().StatusNotices && !a.dry && a.currentCfg().Channels.Status != "" {
		msg := fmt.Sprintf("🤖 **TMR Bot online** (v%s) — poll mỗi %d phút. Theo dõi: %s.",
			version, a.currentCfg().PollIntervalMin, a.sourceList())
		if _, err := a.dc.SendMessage(a.currentCfg().Channels.Status, discord.Message{Content: msg}); err != nil {
			log.Printf("không gửi được thông báo khởi động: %v", err)
		}
	}

	interval := time.Duration(a.currentCfg().PollIntervalMin) * time.Minute
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

	// Chat streak: nối gateway TRƯỚC để realtime không bị block bởi REST backfill
	// (backfill có thể mất vài phút do 429). Backfill chạy nền sau đó + mỗi 6h.
	go a.runGateway()
	go func() {
		a.runStreakCheck(a.dry)
		a.streakBackfillTicker(a.dry)
	}()

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
			// Hot-reload: nếu config.yml đổi thì nạp lại trước khi poll.
			a.maybeReloadConfig()
			newInterval := time.Duration(a.currentCfg().PollIntervalMin) * time.Minute
			if newInterval < time.Minute {
				newInterval = time.Minute
			}
			if newInterval != interval {
				ticker.Reset(newInterval)
				interval = newInterval
				log.Printf("poll interval đổi thành %s", interval)
			}
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
	for _, f := range a.currentFetchers() {
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

	if a.currentCfg().StatusNotices && !a.dry && a.currentCfg().Channels.Status != "" {
		msg := fmt.Sprintf("📦 **TMR Bot** (v%s) đã nạp dữ liệu nền lần đầu (%d mục đã biết). Từ giờ chỉ thông báo mục mới thôi!", version, a.st.CountSeen())
		if len(skipped) > 0 {
			msg += fmt.Sprintf("\n⚠️ Không seed được nguồn: **%s** (bị chặn) — bot vẫn chạy với các nguồn còn lại.", strings.Join(skipped, ", "))
		}
		if _, err := a.dc.SendMessage(a.currentCfg().Channels.Status, discord.Message{Content: msg}); err != nil {
			log.Printf("không gửi được thông báo seed: %v", err)
		}
	}
	return nil
}

func (a *app) fetchAll(seed bool) []pollResult {
	var wg sync.WaitGroup
	fetchers := a.currentFetchers()
	out := make([]pollResult, len(fetchers))
	for i, f := range fetchers {
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
		if a.currentCfg().Debug {
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
	for _, f := range a.currentFetchers() {
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
		if a.currentCfg().StatusNotices && !a.dry && a.currentCfg().Channels.Status != "" {
			msg := fmt.Sprintf("✅ Nguồn **%s** đã truy cập lại được — đã nạp dữ liệu nền, bắt đầu thông báo mục mới.", f.Name())
			_, _ = a.dc.SendMessage(a.currentCfg().Channels.Status, discord.Message{Content: msg})
		}
	}
}

func (a *app) maybeReportError(source string, err error) {
	if a.dry || a.currentCfg().Channels.Status == "" || !a.st.CanPostError(source) {
		return
	}
	msg := fmt.Sprintf("⚠️ Nguồn **%s** poll lỗi: `%v`. Sẽ thử lại ở lần poll sau.", source, err)
	if _, e := a.dc.SendMessage(a.currentCfg().Channels.Status, discord.Message{Content: msg}); e != nil {
		log.Printf("không gửi được báo lỗi: %v", e)
	}
}

// announce filters and sends one item.
// Khi merge_links bật: nếu truyện+chap này đã được thông báo từ nguồn khác,
// bot EDIT tin nhắn cũ để gắn thêm link nguồn mới (kiểu "cuu - mgd - moe - fb").
func (a *app) announce(it sources.Item) error {
	cutoff := time.Now().Add(-time.Duration(a.currentCfg().MaxAgeHours) * time.Hour)
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
		channelID = a.currentCfg().Channels.NovelChap
	case it.Source == "docln":
		channelID = a.currentCfg().Channels.NovelNew
	case it.Kind == sources.KindChapter:
		channelID = a.currentCfg().Channels.MangaChap
	default:
		channelID = a.currentCfg().Channels.MangaNew
	}
	if channelID == "" {
		return fmt.Errorf("chưa cấu hình kênh cho %s/%v", it.Source, it.Kind)
	}

	msg := buildMessage(a.currentCfg().RoleID, it)
	srcName := sourceNames[it.Source]

	if a.currentCfg().MergeLinks {
		if entry, ok := a.st.GetPosted(dedupKey); ok {
			return a.mergeInto(entry, dedupKey, srcName, it.URL, it.Title, it.Chapter)
		}
	}

	// legacy dedup (khi merge_links = false)
	if !a.currentCfg().MergeLinks {
		if a.st.AnnouncedWithin(dedupKey, time.Duration(a.currentCfg().DedupHours)*time.Hour) {
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
	linksLine := linksLineFrom(entry.Links)

	switch {
	case len(base.Components) > 0: // Components V2
		mergeIntoComponents(&base, entry.Links)
	case len(base.Embeds) > 0: // embed cũ (tin nhắn trước v1.3)
		desc := strings.TrimSpace(base.Embeds[0].Description)
		if i := strings.Index(desc, "🔗 **Cũng có tại:**"); i >= 0 {
			desc = strings.TrimSpace(desc[:i])
		}
		base.Embeds[0].Description = joinNonEmpty("\n\n", desc, linksLine)
	default:
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

// buildMessage renders a Components V2 message (embed V2):
//
// buildMessage dựng card Components V2.
//
// Chapter mới (giữ mẫu cũ):
//
//	│ @role / # Tên / Đã upload Chương N / [bìa] / nút Đọc tại / footer
//
// Truyện mới (mẫu mới theo Sáng):
//
//	│ # Truyện mới đã cập bến rồi đây
//	│ **🏷️ Tên:** ...
//	│ **📃 Mô tả:** ...
//	│ **🤝 Collab với:** (nếu có) — [Nhóm](link)
//	│ @role
//	│ [bìa] / nút Đọc tại / The Mavericks | timestamp
func buildMessage(roleID string, it sources.Item) discord.Message {
	src := sourceNames[it.Source]
	color := sourceColors[it.Source]
	role := discord.MentionRole(roleID)

	var head string
	if it.Kind == sources.KindNewSeries {
		head = "# Truyện mới đã cập bến rồi đây\n" +
			"**🏷️ Tên:** " + it.Title + "\n" +
			"**📃 Mô tả:**\n" + Truncate(it.Description, 400)
		if len(it.Genres) > 0 {
			tags := make([]string, 0, len(it.Genres))
			for _, g := range it.Genres {
				tags = append(tags, "`"+g+"`")
			}
			head += "\n**📋 Thể loại:**\n" + strings.Join(tags, ", ")
		}
		if len(it.Collabs) > 0 {
			var b strings.Builder
			b.WriteString("**🤝 Collab với:**\n")
			for _, t := range it.Collabs {
				b.WriteString("- **[" + t.Name + "](" + t.URL + ")**\n")
			}
			head += strings.TrimRight(b.String(), "\n")
		}
	} else {
		head = "# " + it.Title + "\nĐã upload **" + it.Chapter + "**"
	}
	text := joinNonEmpty("\n", head, Truncate(it.Subtitle, 200))
	// Mention đặt CUỐI phần text (theo mẫu); chapter cũ vẫn để đầu cho quen.
	if it.Kind == sources.KindNewSeries {
		text = joinNonEmpty("\n", text, role)
	} else {
		text = joinNonEmpty("\n", role, text)
	}

	inner := []discord.Component{
		{Type: discord.CompTextDisplay, Content: text},
	}
	if it.CoverURL != "" {
		inner = append(inner, discord.Component{
			Type:  discord.CompMediaGallery,
			Items: []discord.MediaItem{{Media: discord.MediaSource{URL: it.CoverURL}}},
		})
	}
	inner = append(inner,
		discord.Component{
			Type: discord.CompActionRow,
			Components: []discord.Component{{
				Type:  discord.CompButton,
				Style: discord.ButtonStyleLink,
				Label: Truncate("Đọc tại "+src, 80),
				URL:   it.URL,
			}},
		},
		discord.Component{Type: discord.CompTextDisplay, Content: footerLine(src, it)},
	)
	return discord.Message{
		AllowedMentions: &discord.AllowedMentions{Parse: []string{}, Roles: []string{roleID}},
		Flags:           discord.FlagComponentsV2,
		Components:      []discord.Component{{Type: discord.CompContainer, AccentColor: color, Components: inner}},
	}
}

func footerLine(src string, it sources.Item) string {
	sep := " • "
	if it.Kind == sources.KindNewSeries {
		sep = " | " // mẫu mới: The Mavericks | timestamp
	}
	line := "-# The Mavericks" + sep + src
	if !it.Published.IsZero() {
		line += sep + it.Published.In(tzVN).Format("02/01 15:04")
	}
	return line
}

var tzVN = time.FixedZone("+07:00", 7*3600)

// linksLineFrom dựng dòng "🔗 Cũng có tại: [Nguồn](url) • ..." (sorted cho ổn định).
func linksLineFrom(links map[string]string) string {
	parts := make([]string, 0, len(links))
	for name, link := range links {
		parts = append(parts, fmt.Sprintf("[%s](%s)", name, link))
	}
	sort.Strings(parts)
	return "🔗 **Cũng có tại:** " + strings.Join(parts, " • ")
}

// mergeIntoComponents chèn/cập nhật dòng link trong message Components V2.
func mergeIntoComponents(base *discord.Message, links map[string]string) {
	line := linksLineFrom(links)
	for ci := range base.Components {
		c := &base.Components[ci]
		if c.Type != discord.CompContainer {
			continue
		}
		linksIdx, rowIdx := -1, -1
		for i := range c.Components {
			switch c.Components[i].Type {
			case discord.CompTextDisplay:
				if strings.HasPrefix(c.Components[i].Content, "🔗") {
					linksIdx = i
				}
			case discord.CompActionRow:
				rowIdx = i
			}
		}
		if linksIdx < 0 {
			ins := len(c.Components)
			if rowIdx >= 0 {
				ins = rowIdx
			}
			c.Components = append(c.Components, discord.Component{})
			copy(c.Components[ins+1:], c.Components[ins:])
			c.Components[ins] = discord.Component{Type: discord.CompTextDisplay, Content: line}
			linksIdx = ins
		}
		c.Components[linksIdx].Content = line
	}
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
	if len(m.Embeds) > 0 {
		return fmt.Sprintf("%s -> %s", m.Embeds[0].Title, m.Embeds[0].URL)
	}
	for _, c := range m.Components {
		if c.Type != discord.CompContainer {
			continue
		}
		var first, btn string
		for _, sub := range c.Components {
			switch sub.Type {
			case discord.CompTextDisplay:
				if first == "" && !strings.HasPrefix(sub.Content, "🔗") && !strings.HasPrefix(sub.Content, "-#") {
					first = strings.SplitN(sub.Content, "\n", 2)[0]
				}
			case discord.CompActionRow:
				if len(sub.Components) > 0 {
					btn = sub.Components[0].Label + " -> " + sub.Components[0].URL
				}
			}
		}
		return strings.TrimSpace(first) + " | " + btn
	}
	return "(trống)"
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

// Truncate rut gon chuoi toi max ki tu.
func Truncate(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}
