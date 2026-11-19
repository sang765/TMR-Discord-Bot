// Package store persists bot state (seen keys, announced dedup, error timing).
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is a JSON-backed key -> time map with helper semantics.
type Store struct {
	mu sync.Mutex
	st state
}

// PostedEntry records a message the bot sent, so later duplicates from
// other sources can be merged into it by editing.
type PostedEntry struct {
	ChannelID string            `json:"channel_id"`
	MessageID string            `json:"message_id"`
	Links     map[string]string `json:"links"` // source display name -> url
	Base      json.RawMessage   `json:"base"`  // original discord.Message JSON
	UpdatedAt time.Time         `json:"updated_at"`
}

type state struct {
	Seen      map[string]time.Time    `json:"seen"`
	Announced map[string]time.Time    `json:"announced"`
	Posted    map[string]PostedEntry  `json:"posted"`
	LastErr   map[string]time.Time    `json:"last_error_post"`
	LastPoll  map[string]time.Time    `json:"last_poll"`
	Seeded    bool                    `json:"seeded"`
}

// Open loads the store from path (creating parent dirs).
func Open(path string) (*Store, error) {
	s := &Store{st: state{
		Seen:      map[string]time.Time{},
		Announced: map[string]time.Time{},
		Posted:    map[string]PostedEntry{},
		LastErr:   map[string]time.Time{},
		LastPoll:  map[string]time.Time{},
	}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &s.st); err != nil {
		// corrupted store: start fresh rather than crash
		return &Store{st: state{
			Seen:      map[string]time.Time{},
			Announced: map[string]time.Time{},
			Posted:    map[string]PostedEntry{},
			LastErr:   map[string]time.Time{},
			LastPoll:  map[string]time.Time{},
		}}, nil
	}
	if s.st.Seen == nil {
		s.st.Seen = map[string]time.Time{}
	}
	if s.st.Announced == nil {
		s.st.Announced = map[string]time.Time{}
	}
	if s.st.LastErr == nil {
		s.st.LastErr = map[string]time.Time{}
	}
	if s.st.LastPoll == nil {
		s.st.LastPoll = map[string]time.Time{}
	}
	if s.st.Posted == nil {
		s.st.Posted = map[string]PostedEntry{}
	}
	return s, nil
}

// Save writes the store atomically and prunes old entries.
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-45 * 24 * time.Hour)
	for _, m := range []map[string]time.Time{s.st.Seen, s.st.Announced} {
		for k, t := range m {
			if t.Before(cutoff) {
				delete(m, k)
			}
		}
	}
	for k, e := range s.st.Posted {
		if e.UpdatedAt.Before(cutoff) {
			delete(s.st.Posted, k)
		}
	}
	data, err := json.MarshalIndent(&s.st, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Seen reports whether key exists.
func (s *Store) Seen(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.st.Seen[key]
	return ok
}

// SeenAt returns the stored time for key.
func (s *Store) SeenAt(key string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.st.Seen[key]
	return t, ok
}

// Mark records key at time t.
func (s *Store) Mark(key string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.st.Seen[key]; ok && t.Before(old) {
		return // keep newest
	}
	s.st.Seen[key] = t
}

// MarkAnnounced records a dedup key.
func (s *Store) MarkAnnounced(key string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Announced[key] = t
}

// GetPosted returns the posted message entry for a dedup key.
func (s *Store) GetPosted(key string) (PostedEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.st.Posted[key]
	return e, ok
}

// SetPosted records/updates the posted message entry for a dedup key.
func (s *Store) SetPosted(key string, e PostedEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Posted[key] = e
}

// AnnouncedWithin reports whether key was announced within window.
func (s *Store) AnnouncedWithin(key string, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.st.Announced[key]
	return ok && time.Since(t) < window
}

// CanPostError rate-limits error notices per source (1 / 6 giờ).
func (s *Store) CanPostError(source string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.st.LastErr[source]
	if ok && time.Since(t) < 6*time.Hour {
		return false
	}
	s.st.LastErr[source] = time.Now()
	return true
}

// SetSeeded marks the baseline as done.
func (s *Store) SetSeeded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Seeded = true
}

// Seeded reports baseline status.
func (s *Store) Seeded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.Seeded
}

// CountSeen returns number of seen keys.
func (s *Store) CountSeen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.st.Seen)
}
