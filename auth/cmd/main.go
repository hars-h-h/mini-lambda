package main

import (
	"log"
	"net/http"
	"os"

	"faas/auth/db"
	"faas/auth/handlers"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("[auth] DATABASE_URL is required")
	}

	pool, err := db.Connect(dbURL)
	if err != nil {
		log.Fatalf("[auth] failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		log.Fatalf("[auth] migration failed: %v", err)
	}
	log.Println("[auth] database ready")

	h := handlers.New(pool)

	mux := http.NewServeMux()

	// Public auth endpoints
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("GET /auth/me", h.Me)

	// Internal — called only by the FaaS server, not exposed publicly
	mux.HandleFunc("GET /internal/validate", h.Validate)

	log.Println("[auth] listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
