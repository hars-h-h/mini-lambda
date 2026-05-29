package handlers

import (
	"encoding/json"
	"net/http"
)

// Validate is an internal endpoint called by the FaaS server to check
// whether a token is valid. Not exposed publicly — only reachable
// service-to-service.
func (h *Handlers) Validate(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	w.Header().Set("Content-Type", "application/json")

	if token == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
		return
	}

	var userID, email string
	err := h.db.QueryRow(r.Context(),
		`SELECT u.id, u.email
		 FROM users u
		 JOIN tokens t ON t.user_id = u.id
		 WHERE t.token = $1 AND t.expires_at > now()`,
		token,
	).Scan(&userID, &email)

	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"user_id": userID,
		"email":   email,
	})
}
