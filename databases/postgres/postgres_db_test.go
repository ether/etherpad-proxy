package postgres

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/ether/etherpad-proxy/models"
)

func newTestDB(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN not set; skipping Postgres integration test")
	}
	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("NewPostgresDB: %v", err)
	}
	// Start from a clean slate so reruns are deterministic.
	if _, err := db.Conn.Exec("TRUNCATE pad, clashes"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPostgresAssignIsAtomic(t *testing.T) {
	db := newTestDB(t)
	first, err := db.Assign("pad1", "b1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if first != "b1" {
		t.Fatalf("expected b1, got %s", first)
	}
	second, err := db.Assign("pad1", "b2")
	if err != nil {
		t.Fatalf("Assign 2: %v", err)
	}
	if second != "b1" {
		t.Fatalf("expected assign to keep b1, got %s", second)
	}
}

func TestPostgresSetGetUpsert(t *testing.T) {
	db := newTestDB(t)
	if err := db.Set("pad1", models.DBBackend{Backend: "b1"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := db.Set("pad1", models.DBBackend{Backend: "b2"}); err != nil {
		t.Fatalf("Set upsert: %v", err)
	}
	got, err := db.Get("pad1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Backend != "b2" {
		t.Fatalf("expected b2, got %s", got.Backend)
	}
	if _, err := db.Get("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestPostgresClashes(t *testing.T) {
	db := newTestDB(t)
	_ = db.RecordClash("pad1", "b1")
	_ = db.RecordClash("pad1", "b1")
	_ = db.RecordClash("pad1", "b2")
	clashes, err := db.GetClashByPadID("pad1")
	if err != nil {
		t.Fatalf("GetClashByPadID: %v", err)
	}
	if len(clashes) != 2 {
		t.Fatalf("expected 2 clashes, got %d", len(clashes))
	}
}
