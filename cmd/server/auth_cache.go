package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second

// CacheEntry holds the resolved identity for a token.
type CacheEntry struct {
	UserID   string
	Email    string
	CachedAt time.Time
}

// TokenCache is a thread-safe in-memory cache that sits in front of
// the auth service. A cache miss triggers a single HTTP call to
// /internal/validate; subsequent calls within cacheTTL are served
// from memory with no network round-trip.
type TokenCache struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry
	authURL string // e.g. "http://localhost:8081"
}

type validateResponse struct {
	Valid   bool   `json:"valid"`
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
}

// NewTokenCache creates a cache and starts a background sweeper goroutine.
func NewTokenCache(authURL string) *TokenCache {
	tc := &TokenCache{
		entries: make(map[string]CacheEntry),
		authURL: authURL,
	}
	go tc.sweep()
	return tc
}

// sweep removes expired cache entries every 5 minutes.
func (tc *TokenCache) sweep() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		tc.mu.Lock()
		for k, v := range tc.entries {
			if time.Since(v.CachedAt) > cacheTTL {
				delete(tc.entries, k)
			}
		}
		tc.mu.Unlock()
		log.Println("[auth_cache] swept expired entries")
	}
}

// Validate returns the CacheEntry for the given token.
// It checks the in-memory cache first; on a miss it calls the auth service.
func (tc *TokenCache) Validate(token string) (CacheEntry, error) {
	// Fast path — read lock only
	tc.mu.RLock()
	if entry, ok := tc.entries[token]; ok && time.Since(entry.CachedAt) < cacheTTL {
		tc.mu.RUnlock()
		return entry, nil
	}
	tc.mu.RUnlock()

	// Slow path — call auth service
	url := fmt.Sprintf("%s/internal/validate?token=%s", tc.authURL, token)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return CacheEntry{}, fmt.Errorf("auth service unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var vr validateResponse
	if err := json.Unmarshal(raw, &vr); err != nil {
		return CacheEntry{}, fmt.Errorf("bad auth response: %w", err)
	}
	if !vr.Valid {
		return CacheEntry{}, fmt.Errorf("invalid or expired token")
	}

	entry := CacheEntry{
		UserID:   vr.UserID,
		Email:    vr.Email,
		CachedAt: time.Now(),
	}

	// Write to cache
	tc.mu.Lock()
	tc.entries[token] = entry
	tc.mu.Unlock()

	return entry, nil
}
