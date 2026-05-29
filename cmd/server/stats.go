package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type FunctionStats struct {
	Invocations int64     `json:"invocations"`
	Errors      int64     `json:"errors"`
	TotalMs     int64     `json:"total_ms"`
	AvgMs       float64   `json:"avg_ms"`
	LastInvoked time.Time `json:"last_invoked"`
}

type StatsTracker struct {
	mu    sync.Mutex
	funcs map[string]*FunctionStats
}

func NewStatsTracker() *StatsTracker {
	return &StatsTracker{
		funcs: make(map[string]*FunctionStats),
	}
}

func (s *StatsTracker) Record(name string, duration time.Duration, errored bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.funcs[name]; !ok {
		s.funcs[name] = &FunctionStats{}
	}

	f := s.funcs[name]
	f.Invocations++
	f.TotalMs += duration.Milliseconds()
	f.AvgMs = float64(f.TotalMs) / float64(f.Invocations)
	f.LastInvoked = time.Now()
	if errored {
		f.Errors++
	}
}

// AvgMs returns the historical average execution time for a function.
// Returns 0 if function has never been invoked (caller should treat as unknown).
func (s *StatsTracker) AvgMs(name string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.funcs[name]; ok && f.Invocations > 0 {
		return int64(f.AvgMs)
	}
	return 0
}

func (s *StatsTracker) Handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.funcs)
}

func (s *StatsTracker) Reset(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.funcs, name)
}

func (s *StatsTracker) ResetHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		s.mu.Lock()
		s.funcs = make(map[string]*FunctionStats)
		s.mu.Unlock()
		fmt.Fprintf(w, `{"status":"reset","scope":"all"}`)
		return
	}
	s.Reset(name)
	fmt.Fprintf(w, `{"status":"reset","name":"%s"}`, name)
}
