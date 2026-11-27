package main

import (
	"testing"

	"tmr-bot/sources"
)

// countingFetcher giả lập fetcher để test auto-seed không gọi Fetch(false) cho nhóm mới.
type countingFetcher struct {
	key    string
	seedN  int
	fetchN int
	store  sources.SeenStore
}

func (c *countingFetcher) Name() string    { return "test" }
func (c *countingFetcher) SeedKey() string { return c.key }
func (c *countingFetcher) Fetch(seed bool) ([]sources.Item, error) {
	if seed {
		c.seedN++
		c.store.Mark(c.key+":item", nowUTC())
	} else {
		c.fetchN++
	}
	return nil, nil
}

func TestAutoSeedNewGroup(t *testing.T) {
	st := newTestStore(t)
	f := &countingFetcher{key: "ct:+999", store: st}
	app := &app{st: st}
	app.cfg.Store(defaultTestConfig())
	app.fetchers = []sources.Fetcher{f}

	app.autoSeedNewGroups()

	if f.seedN != 1 {
		t.Fatalf("want 1 seed call for new group, got %d", f.seedN)
	}
	if !st.IsGroupSeeded("ct:+999") {
		t.Fatal("group not marked seeded")
	}
}

func TestAutoSeedSkipsKnownGroup(t *testing.T) {
	st := newTestStore(t)
	st.MarkGroupSeeded("ct:+999")
	f := &countingFetcher{key: "ct:+999", store: st}
	app := &app{st: st}
	app.cfg.Store(defaultTestConfig())
	app.fetchers = []sources.Fetcher{f}

	app.autoSeedNewGroups()

	if f.seedN != 0 {
		t.Fatalf("known group should not be re-seeded, got %d seed calls", f.seedN)
	}
}
