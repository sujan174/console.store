package store

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Migrate applies the schema idempotently. Run as the schema OWNER role; the
// SECURITY DEFINER function and the console_broker grants depend on owner privs.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schemaSQL)
	return err
}
