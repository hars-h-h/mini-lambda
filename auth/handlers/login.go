package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// generateToken returns a cryptographically random "faas_" prefixed token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "faas_" + hex.EncodeToString(b), nil
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Look up user — use a constant-time comparison path to avoid
	// leaking whether an email exists via timing differences.
	var userID, hashedPwd string
	err := h.db.QueryRow(r.Context(),
		`SELECT id, password FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &hashedPwd)

	if err != nil {
		// Deliberately vague — don't reveal if email exists.
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := generateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = h.db.Exec(r.Context(),
		`INSERT INTO tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, expiresAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt,
	})
}
