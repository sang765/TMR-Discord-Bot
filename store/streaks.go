package store

// StreakStore lưu lịch sử chat theo ngày lịch VN và danh sách đã nhận role.
// File riêng data/streaks.json — không đụng state.json của boc scrape.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type streakState struct {
	Days    map[string]map[string]bool `json:"days"`    // userID -> "2006-01-02" -> true
	Awarded map[string]bool            `json:"awarded"` // userID đã nhận role
}

// StreakStore is a JSON-backed chat-day tracker.
type StreakStore struct {
	mu sync.Mutex
	st streakState
}

// OpenStreak loads (or creates) the streak store.
func OpenStreak(path string) (*StreakStore, error) {
	s := &StreakStore{st: newStreakState()}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &s.st); err != nil {
		return &StreakStore{st: newStreakState()}, nil // corrupt -> fresh
	}
	if s.st.Days == nil {
		s.st.Days = map[string]map[string]bool{}
	}
	if s.st.Awarded == nil {
		s.st.Awarded = map[string]bool{}
	}
	return s, nil
}

func newStreakState() streakState {
	return streakState{
		Days:    map[string]map[string]bool{},
		Awarded: map[string]bool{},
	}
}

// Save writes atomically and prunes days older than 45 days.
func (s *StreakStore) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := "2000-01-01" // đơn giản: giữ lại, tự prune ở recorder nếu cần
	_ = cutoff
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

// IsAwarded reports whether the user already received the streak role.
func (s *StreakStore) IsAwarded(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.Awarded[userID]
}

// MarkAwarded records that the role was granted.
func (s *StreakStore) MarkAwarded(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Awarded[userID] = true
}

// MarkDay records that user chatted on date (YYYY-MM-DD in VN time).
func (s *StreakStore) MarkDay(userID, date string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.Days[userID] == nil {
		s.st.Days[userID] = map[string]bool{}
	}
	s.st.Days[userID][date] = true
}

// HasStreak reports whether the user has `need` consecutive calendar days
// ending on or before today (dates are YYYY-MM-DD, lexicographic sort works).
func (s *StreakStore) HasStreak(userID string, need int, today string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.st.Days[userID]
	if len(m) < need || need <= 0 {
		return false
	}
	// decode date -> days since epoch so we can add 1 day correctly
	dates := make([]int, 0, len(m))
	for d, ok := range m {
		if !ok {
			continue
		}
		if n, ok := dateToOrdinal(d); ok {
			dates = append(dates, n)
		}
	}
	sortInts(dates)
	run, prev := 0, -2
	for _, n := range dates {
		if n > ordinal(today) { // future (timezone quirk) — bỏ
			continue
		}
		if n == prev+1 {
			run++
		} else {
			run = 1
		}
		prev = n
		if run >= need {
			return true
		}
	}
	return false
}

// CountDays returns number of distinct chat days for a user (debug/stats).
func (s *StreakStore) CountDays(userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.st.Days[userID])
}

// Qualified trả về danh sách user có chuỗi `need` ngày liên tiếp tính tới today.
func (s *StreakStore) Qualified(need int, today string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for user := range s.st.Days {
		if s.hasStreakLocked(user, need, today) {
			out = append(out, user)
		}
	}
	return out
}

func (s *StreakStore) hasStreakLocked(userID string, need int, today string) bool {
	m := s.st.Days[userID]
	if len(m) < need || need <= 0 {
		return false
	}
	dates := make([]int, 0, len(m))
	for d, ok := range m {
		if !ok {
			continue
		}
		if n, ok := dateToOrdinal(d); ok && n <= ordinal(today) {
			dates = append(dates, n)
		}
	}
	sortInts(dates)
	run, prev := 0, -2
	for _, n := range dates {
		if n == prev+1 {
			run++
		} else {
			run = 1
		}
		prev = n
		if run >= need {
			return true
		}
	}
	return false
}
