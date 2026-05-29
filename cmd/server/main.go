package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var functionDir = "./functions"
var pool *PoolManager
var stats *StatsTracker
var tokenCache *TokenCache

// contextKey is a typed key to avoid collisions in request context.
type contextKey string

const (
	contextUserID contextKey = "user_id"
	contextEmail  contextKey = "email"
)

type RegisterRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// authMiddleware validates the Bearer token on every protected request.
// On success it injects user_id and email into the request context.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		entry, err := tokenCache.Validate(token)
		if err != nil {
			log.Printf("[auth] rejected token: %v", err)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextUserID, entry.UserID)
		ctx = context.WithValue(ctx, contextEmail, entry.Email)
		next(w, r.WithContext(ctx))
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextUserID).(string)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Code == "" {
		http.Error(w, "name and code required", http.StatusBadRequest)
		return
	}

	// Scope function directory to the authenticated user
	dir := filepath.Join(functionDir, userID, req.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "failed to create function dir", http.StatusInternalServerError)
		return
	}

	codePath := filepath.Join(dir, "handler.py")
	if err := os.WriteFile(codePath, []byte(req.Code), 0644); err != nil {
		http.Error(w, "failed to write function code", http.StatusInternalServerError)
		return
	}

	// Stats key is "userID:fnName" to avoid cross-user collisions
	statsKey := userID + ":" + req.Name
	stats.Reset(statsKey)

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"status":"registered","name":%q}`, req.Name)
	log.Printf("[register] user=%s function=%s saved", userID[:8], req.Name)
}

func invokeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextUserID).(string)

	name := strings.TrimPrefix(r.URL.Path, "/invoke/")
	if name == "" {
		http.Error(w, "function name required", http.StatusBadRequest)
		return
	}

	// Load code from the user-scoped directory
	codePath := filepath.Join(functionDir, userID, name, "handler.py")
	code, err := os.ReadFile(codePath)
	if err != nil {
		http.Error(w, "function not found", http.StatusNotFound)
		return
	}

	var invokeReq struct {
		Payload map[string]interface{} `json:"payload"`
	}
	json.NewDecoder(r.Body).Decode(&invokeReq)
	event := invokeReq.Payload
	if event == nil {
		event = map[string]interface{}{}
	}

	statsKey := userID + ":" + name
	avgMs := stats.AvgMs(statsKey)

	result, err := pool.Invoke(name, string(code), event, avgMs)
	errored := err != nil
	stats.Record(statsKey, result.Duration, errored)

	if errored {
		log.Printf("[invoke] error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"status":"error","error":%q}`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","function":%q,"output":%q,"duration_ms":%d,"warm":%v}`,
		name, result.Output, result.Duration.Milliseconds(), result.WarmHit)
	log.Printf("[invoke] user=%s fn=%s %dms (warm=%v)", userID[:8], name, result.Duration.Milliseconds(), result.WarmHit)
}

func main() {
	if err := os.MkdirAll(functionDir, 0755); err != nil {
		log.Fatal(err)
	}

	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}

	pool = NewPoolManager()
	stats = NewStatsTracker()
	tokenCache = NewTokenCache(authURL)

	log.Printf("[server] auth service: %s", authURL)
	log.Printf("[server] token cache TTL: %v", cacheTTL)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("[server] shutting down...")
		pool.Shutdown()
		os.Exit(0)
	}()

	http.HandleFunc("/register", authMiddleware(registerHandler))
	http.HandleFunc("/invoke/", authMiddleware(invokeHandler))
	http.HandleFunc("/stats", authMiddleware(stats.Handler))
	http.HandleFunc("/stats/reset", authMiddleware(stats.ResetHandler))

	log.Println("[server] listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
