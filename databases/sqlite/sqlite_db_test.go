package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ether/etherpad-proxy/models"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := NewSQLiteDB(path)
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSetAndGet(t *testing.T) {
	db := newTestDB(t)
	if err := db.Set("pad1", models.DBBackend{Backend: "b1"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := db.Get("pad1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Backend != "b1" {
		t.Fatalf("expected b1, got %s", got.Backend)
	}
}

func TestSetUpsert(t *testing.T) {
	db := newTestDB(t)
	_ = db.Set("pad1", models.DBBackend{Backend: "b1"})
	if err := db.Set("pad1", models.DBBackend{Backend: "b2"}); err != nil {
		t.Fatalf("Set upsert: %v", err)
	}
	got, _ := db.Get("pad1")
	if got.Backend != "b2" {
		t.Fatalf("expected b2 after upsert, got %s", got.Backend)
	}
}

func TestAssignIsAtomic(t *testing.T) {
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

func TestGetMissingReturnsNoRows(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Get("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestRecordClashAndGet(t *testing.T) {
	db := newTestDB(t)
	if err := db.RecordClash("pad1", "b1"); err != nil {
		t.Fatalf("RecordClash: %v", err)
	}
	if err := db.RecordClash("pad1", "b1"); err != nil {
		t.Fatalf("RecordClash dup: %v", err)
	}
	if err := db.RecordClash("pad1", "b2"); err != nil {
		t.Fatalf("RecordClash 2: %v", err)
	}
	clashes, err := db.GetClashByPadID("pad1")
	if err != nil {
		t.Fatalf("GetClashByPadID: %v", err)
	}
	if len(clashes) != 2 {
		t.Fatalf("expected 2 clashes, got %d", len(clashes))
	}
}

func TestCleanUpPads(t *testing.T) {
	db := newTestDB(t)
	_ = db.Set("keep", models.DBBackend{Backend: "b1"})
	_ = db.Set("stale", models.DBBackend{Backend: "b1"})
	if err := db.CleanUpPads([]string{"keep"}, "b1"); err != nil {
		t.Fatalf("CleanUpPads: %v", err)
	}
	all, _ := db.GetAllPads()
	if _, ok := all["stale"]; ok {
		t.Fatalf("stale pad should have been deleted")
	}
	if _, ok := all["keep"]; !ok {
		t.Fatalf("keep pad should remain")
	}
}
