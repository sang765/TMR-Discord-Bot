package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHotReloadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	base := `discord_token: "test"
guild_id: "1201419657282863104"
moe_team_paths:
  - "/team/256/the-mavericks"
`
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{cfgPath: path, fetchers: buildFetchers(cfg)}
	a.cfg.Store(cfg)
	if st, err := os.Stat(path); err == nil {
		a.cfgMtime = st.ModTime()
	}
	n0 := len(a.currentFetchers())
	// LoadConfig có defaults enable tất cả 4 nguồn → 4 fetcher lúc đầu
	if n0 != 4 {
		t.Fatalf("want 4 fetchers initially, got %d", n0)
	}

	// đổi config: thêm MangaDex + Moe team 74
	updated := base + `mangadex_group_ids:
  - "1a653584-126f-4775-9af8-e06e250ad829"
  - "9759e8b8-c76a-4d1e-b672-c88e9d5f67e6"
`
	// đảm bảo mtime mới hơn (filesystem có độ phân giải giây)
	future := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	a.maybeReloadConfig()
	n1 := len(a.currentFetchers())
	// thêm MangaDex (1) vào config → 5 fetcher (DocLn 1 + MD 2 + Moe 1 + CT 1)
	if n1 != 5 {
		t.Fatalf("want 5 fetchers after reload, got %d", n1)
	}
	if got := a.currentCfg().PollIntervalMin; got != 15 {
		t.Fatalf("poll interval changed unexpectedly: %d", got)
	}
}

func TestHotReloadSkipsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	valid := "discord_token: \"test\"\n"
	os.WriteFile(path, []byte(valid), 0o644)
	cfg, _ := LoadConfig(path)
	a := &app{cfgPath: path, fetchers: buildFetchers(cfg)}
	a.cfg.Store(cfg)
	st, _ := os.Stat(path)
	a.cfgMtime = st.ModTime()
	n0 := len(a.currentFetchers())

	// config hỏng (yaml sai) → phải giữ config cũ
	future := time.Now().Add(2 * time.Second)
	os.WriteFile(path, []byte("discord_token: [unclosed\n"), 0o644)
	os.Chtimes(path, future, future)
	a.maybeReloadConfig()
	if n1 := len(a.currentFetchers()); n1 != n0 {
		t.Fatalf("invalid config replaced fetchers: %d -> %d", n0, n1)
	}
}
