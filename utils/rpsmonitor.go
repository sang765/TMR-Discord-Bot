package utils

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// API call types tracked by the RPS monitor
const (
	APIGuildEdit        = "GuildEdit"
	APIChannelMessageSend = "ChannelMessageSend"
	APIChannelMessageEdit = "ChannelMessageEdit"
	APIInteractionRespond = "InteractionRespond"
	APIGuildMember      = "GuildMember"
	APIGuild            = "Guild"
	APIStateRole        = "StateRole"
	APIStateMember      = "StateMember"
)

// RPSMonitor tracks Discord API calls and detects high RPS.
type RPSMonitor struct {
	counters  map[string]*atomic.Int64
	history   []rpsSnapshot
	historyMu sync.Mutex
	threshold float64
	stopCh    chan struct{}
}

type rpsSnapshot struct {
	timestamp  time.Time
	totalRPS   float64
	breakdown  map[string]int64
}

// NewRPSMonitor creates a monitor with the given RPS threshold warning level.
func NewRPSMonitor(threshold float64) *RPSMonitor {
	return &RPSMonitor{
		counters: map[string]*atomic.Int64{
			APIGuildEdit:          {},
			APIChannelMessageSend: {},
			APIChannelMessageEdit: {},
			APIInteractionRespond: {},
			APIGuildMember:        {},
			APIGuild:              {},
			APIStateRole:          {},
			APIStateMember:        {},
		},
		threshold: threshold,
		stopCh:    make(chan struct{}),
	}
}

// Record increments the counter for an API call type. Call this after each Discord API call.
func (m *RPSMonitor) Record(callType string) {
	if c, ok := m.counters[callType]; ok {
		c.Add(1)
	}
}

// Start begins the background RPS monitor. It checks every second and logs warnings.
func (m *RPSMonitor) Start() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var prevTotal int64
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			current := m.total()
			rps := float64(current - prevTotal)
			prevTotal = current

			snapshot := m.snapshot(rps)

			// Keep last 60 snapshots (1 minute window)
			m.historyMu.Lock()
			m.history = append(m.history, snapshot)
			if len(m.history) > 60 {
				m.history = m.history[1:]
			}
			m.historyMu.Unlock()

			if rps >= m.threshold {
				m.logWarning(rps, snapshot.breakdown)
			}
		}
	}
}

// Stop stops the monitor.
func (m *RPSMonitor) Stop() {
	close(m.stopCh)
}

// GetRPS returns the current RPS and a breakdown by call type over the last N seconds.
func (m *RPSMonitor) GetRPS(window int) (float64, map[string]float64) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	if len(m.history) == 0 {
		return 0, nil
	}

	if window > len(m.history) {
		window = len(m.history)
	}

	recent := m.history[len(m.history)-window:]
	breakdown := make(map[string]float64)

	for _, snap := range recent {
		for k, v := range snap.breakdown {
			breakdown[k] += float64(v)
		}
	}

	// Average RPS over window
	var total float64
	for _, snap := range recent {
		total += snap.totalRPS
	}
	avgRPS := total / float64(window)

	// Convert totals to per-second averages
	for k := range breakdown {
		breakdown[k] /= float64(window)
	}

	return avgRPS, breakdown
}

func (m *RPSMonitor) total() int64 {
	var total int64
	for _, c := range m.counters {
		total += c.Load()
	}
	return total
}

func (m *RPSMonitor) snapshot(rps float64) rpsSnapshot {
	breakdown := make(map[string]int64)
	for name, c := range m.counters {
		v := c.Load()
		if v > 0 {
			breakdown[name] = v
		}
	}
	return rpsSnapshot{
		timestamp: time.Now(),
		totalRPS:  rps,
		breakdown:  breakdown,
	}
}

func (m *RPSMonitor) logWarning(rps float64, breakdown map[string]int64) {
	// Build breakdown string, sorted by count descending
	type kv struct {
		key   string
		value int64
	}
	var sorted []kv
	for k, v := range breakdown {
		if v > 0 {
			sorted = append(sorted, kv{k, v})
		}
	}
	// Simple bubble sort (small slice)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].value > sorted[i].value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	detail := ""
	for _, kv := range sorted {
		detail += fmt.Sprintf("  %s: %d calls\n", kv.key, kv.value)
	}

	slog.Warn("⚠️ High Discord API RPS detected",
		slog.Float64("rps", rps),
		slog.Float64("threshold", m.threshold),
		slog.String("breakdown", "\n"+detail),
	)
}

// GetSummary returns a human-readable summary of the last window.
func (m *RPSMonitor) GetSummary(window int) string {
	avgRPS, breakdown := m.GetRPS(window)
	if avgRPS == 0 {
		return "RPS: 0 (no API calls tracked)"
	}

	summary := fmt.Sprintf("RPS: %.1f (threshold: %.0f)\n", avgRPS, m.threshold)
	summary += "Breakdown (avg/sec):\n"
	for name, count := range breakdown {
		if count > 0 {
			summary += fmt.Sprintf("  %s: %.1f\n", name, count)
		}
	}
	return summary
}
