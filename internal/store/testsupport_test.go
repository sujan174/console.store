package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool returns an OWNER-role pool against a freshly migrated schema.
// It skips the test when CONSOLE_TEST_DB_DSN is unset so the suite stays
// green without a database.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CONSOLE_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CONSOLE_TEST_DB_DSN unset; skipping Postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect owner pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// brokerPool returns a console_broker-role pool (RLS enforced). Migrate must
// already have created and granted the role (Migrate does).
func brokerPool(t *testing.T, owner *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CONSOLE_TEST_DB_DSN")
	bdsn, err := brokerDSN(dsn)
	if err != nil {
		t.Fatalf("derive broker dsn: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), bdsn)
	if err != nil {
		t.Fatalf("connect broker pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
