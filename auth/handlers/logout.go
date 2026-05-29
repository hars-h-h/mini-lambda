package handlers

import (
	"encoding/json"
	"net/http"
)

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
		return
	}

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM tokens WHERE token = $1`,
		token,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
}
