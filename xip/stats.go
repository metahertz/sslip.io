package xip

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

// UnknownCountry is the bucket used when the source IP can't be geolocated
// (no GeoIP database loaded, private/unroutable IP, or unmapped IP).
const UnknownCountry = "ZZ"

// Stats holds the usage data powering the dashboard: a per-query-type
// breakdown, a per-country breakdown, and a per-day time series. Unlike
// Metrics (which is a copy-able struct of lock-free counters), Stats is
// shared by pointer and is safe for concurrent use.
type Stats struct {
	mu        sync.Mutex
	start     time.Time
	total     uint64
	byType    map[string]uint64 // DNS query type (e.g. "TypeA") -> count
	byCountry map[string]uint64 // ISO 3166 country code (e.g. "US") -> count
	byDay     map[string]uint64 // day bucket "2006-01-02" (UTC) -> count
}

// NewStats returns an empty, ready-to-use Stats whose clock starts now.
func NewStats() *Stats {
	return &Stats{
		start:     time.Now(),
		byType:    map[string]uint64{},
		byCountry: map[string]uint64{},
		byDay:     map[string]uint64{},
	}
}

// Record increments the counters for a single query. qType is the DNS query
// type (e.g. "TypeA"), country is an ISO country code ("" is treated as
// unknown), and day is a "2006-01-02" bucket. A nil *Stats is a no-op so the
// rest of the server keeps working when stats are disabled.
func (s *Stats) Record(qType, country, day string) {
	if s == nil {
		return
	}
	if country == "" {
		country = UnknownCountry
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	s.byType[qType]++
	s.byCountry[country]++
	s.byDay[day]++
}

// CountStat is one (key, count) pair in a Snapshot.
type CountStat struct {
	Key   string `json:"key"`
	Count uint64 `json:"count"`
}

// Snapshot is a point-in-time, JSON-serializable view of the Stats, with the
// breakdowns pre-sorted for display.
type Snapshot struct {
	Start         time.Time   `json:"start"`
	UptimeSeconds float64     `json:"uptime_seconds"`
	Total         uint64      `json:"total"`
	PerSecond     float64     `json:"per_second"`
	ByType        []CountStat `json:"by_type"`    // sorted by count, descending
	ByCountry     []CountStat `json:"by_country"` // sorted by count, descending
	ByDay         []CountStat `json:"by_day"`     // sorted by day, ascending (chronological)
}

// Snapshot returns a consistent copy of the current counters. by_type and
// by_country are sorted by count (descending); by_day is sorted chronologically.
func (s *Stats) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	uptime := time.Since(s.start).Seconds()
	perSecond := 0.0
	if uptime > 0 {
		perSecond = float64(s.total) / uptime
	}
	snap := Snapshot{
		Start:         s.start,
		UptimeSeconds: uptime,
		Total:         s.total,
		PerSecond:     perSecond,
		ByType:        sortedByCount(s.byType),
		ByCountry:     sortedByCount(s.byCountry),
		ByDay:         sortedByKey(s.byDay),
	}
	return snap
}

func sortedByCount(m map[string]uint64) []CountStat {
	out := make([]CountStat, 0, len(m))
	for k, v := range m {
		out = append(out, CountStat{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count // higher counts first
		}
		return out[i].Key < out[j].Key // stable tie-break
	})
	return out
}

func sortedByKey(m map[string]uint64) []CountStat {
	out := make([]CountStat, 0, len(m))
	for k, v := range m {
		out = append(out, CountStat{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// statsData is the on-disk representation of Stats (no mutex, plain maps).
type statsData struct {
	Start     time.Time         `json:"start"`
	Total     uint64            `json:"total"`
	ByType    map[string]uint64 `json:"by_type"`
	ByCountry map[string]uint64 `json:"by_country"`
	ByDay     map[string]uint64 `json:"by_day"`
}

// Save atomically writes the Stats to path as JSON (temp file + rename).
func (s *Stats) Save(path string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	data := statsData{
		Start:     s.start,
		Total:     s.total,
		ByType:    cloneMap(s.byType),
		ByCountry: cloneMap(s.byCountry),
		ByDay:     cloneMap(s.byDay),
	}
	s.mu.Unlock()
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadStats reads a Stats previously written by Save. If the file doesn't
// exist it returns a fresh Stats and no error, so first-run startup is clean.
func LoadStats(path string) (*Stats, error) {
	encoded, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewStats(), nil
	}
	if err != nil {
		return nil, err
	}
	var data statsData
	if err := json.Unmarshal(encoded, &data); err != nil {
		return nil, err
	}
	s := NewStats()
	if !data.Start.IsZero() {
		s.start = data.Start
	}
	s.total = data.Total
	if data.ByType != nil {
		s.byType = data.ByType
	}
	if data.ByCountry != nil {
		s.byCountry = data.ByCountry
	}
	if data.ByDay != nil {
		s.byDay = data.ByDay
	}
	return s, nil
}

func cloneMap(m map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
