package store

import (
	"context"
	"testing"
)

func TestMigrateCreatesTablesAndRole(t *testing.T) {
	pool := testPool(t) // skips if no DB; runs Migrate
	ctx := context.Background()
	for _, tbl := range []string{"accounts", "ssh_pubkeys", "swiggy_tokens"} {
		var ok bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)`, tbl).Scan(&ok)
		if err != nil || !ok {
			t.Fatalf("table %s missing (err=%v)", tbl, err)
		}
	}
	var roleOK bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='console_broker')`).Scan(&roleOK); err != nil || !roleOK {
		t.Fatalf("console_broker role missing (err=%v)", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}
