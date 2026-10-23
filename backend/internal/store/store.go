// Package store owns the SQLite database: schema, seed data and a write lock.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Store wraps the database. SQLite allows one writer at a time; instead of
// retrying SQLITE_BUSY, every write transaction goes through Write, which
// serialises them in-process.
type Store struct {
	DB *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Store, error) {
	dsn := path
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, err
		}
	} else {
		// A private in-memory database per Open call, shared by its connections.
		dsn = "file:" + NewID("mem") + "?mode=memory&cache=shared"
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, schema)
	return err
}

// Write runs fn in a transaction while holding the write lock.
func (s *Store) Write(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// tables in delete order (children first).
var tables = []string{
	"inbox", "jobs", "idempotency_keys", "refunds", "payments", "cards",
	"order_items", "orders", "prescriptions", "addresses", "products", "categories",
	"oauth_codes", "api_keys", "otp_codes", "refresh_tokens", "sessions", "users",
}

// Reset empties every table and seeds the demo data again.
func (s *Store) Reset(ctx context.Context, now int64) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		for _, t := range tables {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
				return fmt.Errorf("clear %s: %w", t, err)
			}
		}
		return seed(ctx, tx, now)
	})
}

// SeedIfEmpty seeds a fresh database and leaves an existing one alone.
func (s *Store) SeedIfEmpty(ctx context.Context, now int64) (bool, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	return true, s.Write(ctx, func(tx *sql.Tx) error { return seed(ctx, tx, now) })
}

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewID returns "<prefix>_<16 random base32 chars>".
func NewID(prefix string) string {
	return prefix + "_" + strings.ToLower(RandomString(10))
}

// RandomString returns n random bytes, base32 encoded.
func RandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b32.EncodeToString(b)
}

// Hash is the one-way form secrets are stored in (refresh tokens, API keys,
// cookies, OAuth codes). They are high-entropy, so a plain SHA-256 is enough.
func Hash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}
