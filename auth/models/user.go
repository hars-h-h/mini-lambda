package models

import "time"

// User represents a registered account in the system.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // never serialised
	CreatedAt time.Time `json:"created_at"`
}

// Token represents an opaque API token issued on login.
type Token struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
