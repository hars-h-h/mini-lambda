package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pgx connection pool to the given PostgreSQL URL.
func Connect(url string) (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), url)
}

// Migrate runs the schema migrations. Safe to call on every startup
// (all statements use IF NOT EXISTS).
func Migrate(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS users (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email       TEXT UNIQUE NOT NULL,
			password    TEXT NOT NULL,
			created_at  TIMESTAMPTZ DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS tokens (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
			token       TEXT UNIQUE NOT NULL,
			expires_at  TIMESTAMPTZ NOT NULL,
			created_at  TIMESTAMPTZ DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);
	`)
	return err
}
