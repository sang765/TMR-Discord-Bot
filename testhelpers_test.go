package main

import (
	"path/filepath"
	"testing"
	"time"

	"tmr-bot/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func defaultTestConfig() *Config {
	cfg := &Config{
		DiscordToken:    "test",
		PollIntervalMin: 5,
		MaxAgeHours:     72,
		EnableDocLn:     false,
		EnableMangaDex:  false,
		EnableMoe:       false,
		EnableCuuTruyen: false,
	}
	cfg.Streak.Enabled = false
	return cfg
}

func nowUTC() time.Time { return time.Now().UTC() }
