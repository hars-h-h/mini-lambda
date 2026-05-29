package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
		return
	}

	var id, email string
	var createdAt time.Time

	err := h.db.QueryRow(r.Context(),
		`SELECT u.id, u.email, u.created_at
		 FROM users u
		 JOIN tokens t ON t.user_id = u.id
		 WHERE t.token = $1 AND t.expires_at > now()`,
		token,
	).Scan(&id, &email, &createdAt)

	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         id,
		"email":      email,
		"created_at": createdAt,
	})
}
