package main

// Configuration for TMR Discord Bot.
// Values come from config/config.yml, with environment variable overrides.

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChannelIDs holds all Discord channel snowflakes.
type ChannelIDs struct {
	Status    string `yaml:"status"`
	NovelChap string `yaml:"novel_chap"`
	NovelNew  string `yaml:"novel_new"`
	MangaChap string `yaml:"manga_chap"`
	MangaNew  string `yaml:"manga_new"`
}

// Config is the full bot configuration.
type Config struct {
	DiscordToken string     `yaml:"discord_token"`
	GuildID      string     `yaml:"guild_id"`
	RoleID       string     `yaml:"role_id"`
	Channels     ChannelIDs `yaml:"channels"`

	// Poll interval in minutes (default 15).
	PollIntervalMin int `yaml:"poll_interval_min"`

	// Only announce items published within this many hours (default 72).
	MaxAgeHours int `yaml:"max_age_hours"`
	// Cross-source dedup window in hours (default 6).
	// Chỉ dùng khi merge_links = false.
	DedupHours int `yaml:"dedup_hours"`
	// Khi nguồn khác có cùng truyện+chap: edit tin nhắn cũ để gắn thêm link
	// thay vì gửi tin mới (mặc định bật).
	MergeLinks bool `yaml:"merge_links"`
	// DocLn group pages to scan (25 series per page, default 2).
	DocLnPages int `yaml:"docln_pages"`
	// MangaDex feed size (default 50).
	MangaDexLimit int `yaml:"mangadex_limit"`
	// DocLn group id (default 3474).
	DocLnGroupID string `yaml:"docln_group_id"`
	// Moe team path (default /team/256/the-mavericks).
	MoeTeamPath string `yaml:"moe_team_path"`
	// CuuTruyen team id (default +695).
	CuuTruyenTeamID string `yaml:"cuutruyen_team_id"`
	// CuuTruyen: skip direct fetch, always use r.jina.ai fallback.
	CuuTruyenJina bool `yaml:"cuutruyen_jina"`

	// Domains section allows overriding scraped endpoints (mirrors etc).
	CuuTruyenBase string `yaml:"cuutruyen_base"` // default https://cuutruyen.net
	DocLnBase     string `yaml:"docln_base"`     // default https://docln.sbs (net/hako.vn mirrors)
	MoeBase       string `yaml:"moe_base"`       // default https://truyen.moe

	// Sources toggles.
	EnableDocLn     bool `yaml:"enable_docln"`
	EnableMangaDex  bool `yaml:"enable_mangadex"`
	EnableMoe       bool `yaml:"enable_moe"`
	EnableCuuTruyen bool `yaml:"enable_cuutruyen"`

	// Send startup/baseline notices to the status channel.
	StatusNotices bool `yaml:"status_notices"`

	// Proxy cho cac request scrape: http://, https://, socks5://. Rong = di thang.
	Proxy string `yaml:"proxy"`

	// Debug prints scraped items without posting.
	Debug bool `yaml:"debug"`

	// Chat streak: cấp role sau N ngày lịch VN liên tiếp có ít nhất 1 tin nhắn.
	Streak struct {
		Enabled      bool     `yaml:"enabled"`
		RoleID       string   `yaml:"role_id"`
		Days         int      `yaml:"days"`          // default 3
		BackfillDays int      `yaml:"backfill_days"` // default 7
		Exclude      []string `yaml:"exclude"`       // channel ID bỏ qua (vd bot-setup)
		Announce     bool     `yaml:"announce"`      // báo khi cấp role
		AnnounceCh   string   `yaml:"announce_channel"`
	} `yaml:"streak"`
}

// LoadConfig reads config.yml, then applies env overrides.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		PollIntervalMin: 15,
		MaxAgeHours:     72,
		DedupHours:      6,
		MergeLinks:      true,
		DocLnPages:      2,
		MangaDexLimit:   50,
		DocLnGroupID:    "3474",
		MoeTeamPath:     "/team/256/the-mavericks",
		CuuTruyenTeamID: "+695",
		CuuTruyenBase:   "https://cuutruyen.net",
		DocLnBase:       "https://docln.sbs",
		MoeBase:         "https://truyen.moe",
		EnableDocLn:     true,
		EnableMangaDex:  true,
		EnableMoe:       true,
		EnableCuuTruyen: true,
		StatusNotices:   true,
	}
	cfg.Streak.Enabled = true
	cfg.Streak.RoleID = "1444365043667046522" // Con Dân Tích Cực
	cfg.Streak.Days = 3
	cfg.Streak.BackfillDays = 7
	cfg.Streak.Announce = true
	cfg.Streak.Exclude = []string{}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Env overrides (used by run.sh / .env).
	if v := os.Getenv("DISCORD_TOKEN"); v != "" {
		cfg.DiscordToken = v
	}
	if v := os.Getenv("GUILD_ID"); v != "" {
		cfg.GuildID = v
	}
	if v := os.Getenv("ROLE_ID"); v != "" {
		cfg.RoleID = v
	}
	if v := os.Getenv("STATUS_CHANNEL_ID"); v != "" {
		cfg.Channels.Status = v
	}
	if v := os.Getenv("NOVEL_CHAP_CHANNEL_ID"); v != "" {
		cfg.Channels.NovelChap = v
	}
	if v := os.Getenv("NOVEL_NEW_CHANNEL_ID"); v != "" {
		cfg.Channels.NovelNew = v
	}
	if v := os.Getenv("MANGA_CHAP_CHANNEL_ID"); v != "" {
		cfg.Channels.MangaChap = v
	}
	if v := os.Getenv("MANGA_NEW_CHANNEL_ID"); v != "" {
		cfg.Channels.MangaNew = v
	}
	if v := os.Getenv("POLL_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.PollIntervalMin = n
		}
	}
	if v := os.Getenv("CUUTRUYEN_BASE"); v != "" {
		cfg.CuuTruyenBase = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("DOCLN_BASE"); v != "" {
		cfg.DocLnBase = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("MOE_BASE"); v != "" {
		cfg.MoeBase = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("SCRAPE_PROXY"); v != "" {
		cfg.Proxy = v
	}
	if v := os.Getenv("STREAK_ROLE_ID"); v != "" {
		cfg.Streak.RoleID = v
	}
	if v := os.Getenv("STREAK_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Streak.Days = n
		}
	}
	if v := os.Getenv("DEBUG"); v != "" {
		cfg.Debug = v == "1" || strings.EqualFold(v, "true")
	}

	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("discord_token is required (config.yml or DISCORD_TOKEN env)")
	}
	return cfg, nil
}
