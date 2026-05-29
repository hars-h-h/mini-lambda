package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	db *pgxpool.Pool
}

// New creates a Handlers instance backed by the given DB pool.
func New(db *pgxpool.Pool) *Handlers {
	return &Handlers{db: db}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// extractToken pulls the Bearer token from the Authorization header.
func extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
