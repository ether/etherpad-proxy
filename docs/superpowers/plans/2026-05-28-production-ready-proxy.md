# Production-Ready Etherpad Proxy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make etherpad-proxy reliable for multi-instance production use — working Postgres-backed shared state with race-free pad assignment, no data races, observability/operability, tests + CI, and non-Docker installation docs with prebuilt binaries.

**Architecture:** A fleet of stateless proxy instances shares `padId → backend` routing through a shared database (SQLite for single-instance, Postgres for multi-instance). Pad assignment is atomic (`INSERT … ON CONFLICT DO NOTHING` + read-back) so concurrent proxies never split-brain a new pad. Per-instance state (which backends are up/available) stays in-memory behind an `RWMutex`. The process exposes health, readiness, and Prometheus metrics on a management port and shuts down gracefully.

**Tech Stack:** Go 1.24+, `database/sql` + Masterminds/squirrel, `jackc/pgx/v5` (Postgres driver via stdlib), `modernc.org/sqlite`, `prometheus/client_golang`, zap, GoReleaser + GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-05-28-production-ready-proxy-design.md`

---

## File Structure

**Create:**
- `metrics/metrics.go` — Prometheus collectors, updated by handlers and loops.
- `databases/sqlite/sqlite_db_test.go` — DB contract tests against temp-file SQLite.
- `databases/postgres/postgres_db_test.go` — same contract, gated on `PG_TEST_DSN`.
- `models/backendstate_test.go` — concurrency-safe state tests.
- `models/settings_test.go` — config validation tests.
- `proxyHandler_test.go` — routing decision tests with a fake `IDB`.
- `checkAvailability_test.go` — availability check against `httptest` backends.
- `support/etherpad-proxy.service` — sample systemd unit.
- `.goreleaser.yaml` — cross-platform release build config.
- `.github/workflows/test.yml` — vet + race tests + Postgres service.
- `.github/workflows/release.yml` — tag-triggered GoReleaser release.

**Modify:**
- `databases/interfaces/iDB.go` — add `Assign`.
- `databases/sqlite/sqlite_db.go` — dialect-correct upserts, `Assign`, placeholder format.
- `databases/postgres/postgres_db.go` — pgx driver, `$N` placeholders, upserts, `Assign`.
- `models/AvailableBackends.go` — replace bare struct with `RWMutex`-guarded `BackendState`.
- `models/Settings.go` — add `ManagementPort`, add `Validate()`.
- `main.go` — call `Validate()`.
- `proxyHandler.go` — extract `chooseBackend`, guard static resources, use `BackendState`, metrics.
- `runtime.go` — `BackendState` wiring, graceful shutdown, health/readiness/metrics endpoints, structured logging.
- `go.mod` / `go.sum` — add pgx + prometheus, drop lib/pq.
- `README.md` — "Installation without Docker" + Postgres + systemd + binaries.

---

## Task 1: Database layer — pgx driver, dialect-correct SQL, atomic `Assign`

**Files:**
- Modify: `databases/interfaces/iDB.go`
- Modify: `databases/sqlite/sqlite_db.go`
- Modify: `databases/postgres/postgres_db.go`
- Create: `databases/sqlite/sqlite_db_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add pgx dependency, drop lib/pq**

Run:
```bash
go get github.com/jackc/pgx/v5@latest
go mod tidy
```
Expected: `github.com/jackc/pgx/v5` appears in `go.mod` require block; `github.com/lib/pq` is removed after the postgres file stops importing it (Step 5). Re-run `go mod tidy` after Step 5.

- [ ] **Step 2: Add `Assign` to the IDB interface**

Replace the contents of `databases/interfaces/iDB.go`:
```go
package interfaces

import "github.com/ether/etherpad-proxy/models"

type IDB interface {
	Close() error
	Get(id string) (*models.DBBackend, error)
	CleanUpPads(padIds []string, padPrefix string) error
	RecordClash(id string, data string) error
	Set(id string, backend models.DBBackend) error
	GetAllPads() (map[string]string, error)
	GetClashByPadID(id string) ([]string, error)
	// Assign stores candidate as the backend for padId only if no backend is
	// already stored, and returns the backend now authoritative for padId (the
	// pre-existing one if there was a race, otherwise candidate).
	Assign(padId string, candidate string) (string, error)
}
```

- [ ] **Step 3: Write the failing SQLite test**

Create `databases/sqlite/sqlite_db_test.go`:
```go
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
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./databases/sqlite/ -run TestAssignIsAtomic -v`
Expected: compile failure — `db.Assign undefined` (method not implemented yet).

- [ ] **Step 5: Rewrite the SQLite implementation**

Replace the contents of `databases/sqlite/sqlite_db.go`:
```go
package sqlite

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	Conn *sql.DB
	sb   sq.StatementBuilderType
}

var _ interfaces.IDB = (*DB)(nil)

func NewSQLiteDB(filename string) (*DB, error) {
	conn, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Conn: conn,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}

	if _, err = db.Conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT, backend TEXT, PRIMARY KEY (id))"); err != nil {
		return nil, err
	}
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS clashes (id TEXT, data TEXT, PRIMARY KEY (id, data))"); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}

func (db *DB) Get(id string) (*models.DBBackend, error) {
	sqlGet, args, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	var data string
	if err = db.Conn.QueryRow(sqlGet, args...).Scan(&data); err != nil {
		return nil, err
	}
	return &models.DBBackend{Backend: data}, nil
}

func (db *DB) CleanUpPads(padIds []string, padPrefix string) error {
	sqlDelete, args, err := db.sb.Delete("pad").
		Where(sq.And{sq.NotEq{"id": padIds}, sq.Like{"backend": padPrefix}}).ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlDelete, args...)
	return err
}

func (db *DB) RecordClash(id string, data string) error {
	sqlStr, args, err := db.sb.Insert("clashes").Columns("id", "data").Values(id, data).
		Suffix("ON CONFLICT (id, data) DO NOTHING").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *DB) Set(id string, dbModel models.DBBackend) error {
	sqlStr, args, err := db.sb.Insert("pad").Columns("id", "backend").Values(id, dbModel.Backend).
		Suffix("ON CONFLICT (id) DO UPDATE SET backend = EXCLUDED.backend").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *DB) Assign(padId string, candidate string) (string, error) {
	insSQL, insArgs, err := db.sb.Insert("pad").Columns("id", "backend").Values(padId, candidate).
		Suffix("ON CONFLICT (id) DO NOTHING").ToSql()
	if err != nil {
		return "", err
	}
	if _, err = db.Conn.Exec(insSQL, insArgs...); err != nil {
		return "", err
	}
	selSQL, selArgs, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return "", err
	}
	var backend string
	if err = db.Conn.QueryRow(selSQL, selArgs...).Scan(&backend); err != nil {
		return "", err
	}
	return backend, nil
}

func (db *DB) GetAllPads() (map[string]string, error) {
	padIDMap := make(map[string]string)
	sqlGet, args, err := db.sb.Select("id", "backend").From("pad").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var padID, backend string
		if err := rows.Scan(&padID, &backend); err != nil {
			return nil, err
		}
		padIDMap[padID] = backend
	}
	return padIDMap, rows.Err()
}

func (db *DB) GetClashByPadID(padId string) ([]string, error) {
	sqlGet, args, err := db.sb.Select("data").From("clashes").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	data := make([]string, 0)
	for rows.Next() {
		var clash string
		if err := rows.Scan(&clash); err != nil {
			return nil, err
		}
		data = append(data, clash)
	}
	return data, rows.Err()
}
```

- [ ] **Step 6: Rewrite the Postgres implementation (pgx + `$N` + upserts)**

Replace the contents of `databases/postgres/postgres_db.go`:
```go
package postgres

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/models"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct {
	Conn *sql.DB
	sb   sq.StatementBuilderType
}

var _ interfaces.IDB = (*Postgres)(nil)

func NewPostgresDB(connstr string) (*Postgres, error) {
	conn, err := sql.Open("pgx", connstr)
	if err != nil {
		return nil, err
	}

	db := &Postgres{
		Conn: conn,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT, backend TEXT, PRIMARY KEY (id))"); err != nil {
		return nil, err
	}
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS clashes (id TEXT, data TEXT, PRIMARY KEY (id, data))"); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *Postgres) Close() error {
	return db.Conn.Close()
}

func (db *Postgres) Get(id string) (*models.DBBackend, error) {
	sqlGet, args, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	var data string
	if err = db.Conn.QueryRow(sqlGet, args...).Scan(&data); err != nil {
		return nil, err
	}
	return &models.DBBackend{Backend: data}, nil
}

func (db *Postgres) CleanUpPads(padIds []string, padPrefix string) error {
	sqlDelete, args, err := db.sb.Delete("pad").
		Where(sq.And{sq.NotEq{"id": padIds}, sq.Like{"backend": padPrefix}}).ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlDelete, args...)
	return err
}

func (db *Postgres) RecordClash(id string, data string) error {
	sqlStr, args, err := db.sb.Insert("clashes").Columns("id", "data").Values(id, data).
		Suffix("ON CONFLICT (id, data) DO NOTHING").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *Postgres) Set(id string, dbModel models.DBBackend) error {
	sqlStr, args, err := db.sb.Insert("pad").Columns("id", "backend").Values(id, dbModel.Backend).
		Suffix("ON CONFLICT (id) DO UPDATE SET backend = EXCLUDED.backend").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *Postgres) Assign(padId string, candidate string) (string, error) {
	insSQL, insArgs, err := db.sb.Insert("pad").Columns("id", "backend").Values(padId, candidate).
		Suffix("ON CONFLICT (id) DO NOTHING").ToSql()
	if err != nil {
		return "", err
	}
	if _, err = db.Conn.Exec(insSQL, insArgs...); err != nil {
		return "", err
	}
	selSQL, selArgs, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return "", err
	}
	var backend string
	if err = db.Conn.QueryRow(selSQL, selArgs...).Scan(&backend); err != nil {
		return "", err
	}
	return backend, nil
}

func (db *Postgres) GetAllPads() (map[string]string, error) {
	padIDMap := make(map[string]string)
	sqlGet, args, err := db.sb.Select("id", "backend").From("pad").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var padID, backend string
		if err := rows.Scan(&padID, &backend); err != nil {
			return nil, err
		}
		padIDMap[padID] = backend
	}
	return padIDMap, rows.Err()
}

func (db *Postgres) GetClashByPadID(padId string) ([]string, error) {
	sqlGet, args, err := db.sb.Select("data").From("clashes").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	data := make([]string, 0)
	for rows.Next() {
		var clash string
		if err := rows.Scan(&clash); err != nil {
			return nil, err
		}
		data = append(data, clash)
	}
	return data, rows.Err()
}
```

- [ ] **Step 7: Tidy modules and run the tests**

Run:
```bash
go mod tidy
go test ./databases/... -v
```
Expected: `lib/pq` gone from `go.mod`; all SQLite tests PASS. (Postgres tests are added in Task 2 and skip without a DSN.)

- [ ] **Step 8: Commit**

```bash
git add databases/ go.mod go.sum
git commit -m "fix: make Postgres functional and pad assignment atomic (#54)

- swap lib/pq for jackc/pgx/v5 (driver was never registered)
- use dialect-correct placeholders and ON CONFLICT upserts
- add atomic Assign to prevent multi-proxy split-brain
- add SQLite DB layer tests

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Postgres integration test (gated on `PG_TEST_DSN`)

**Files:**
- Create: `databases/postgres/postgres_db_test.go`

- [ ] **Step 1: Write the gated integration test**

Create `databases/postgres/postgres_db_test.go`:
```go
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
```

- [ ] **Step 2: Verify it skips without a DSN**

Run: `go test ./databases/postgres/ -v`
Expected: tests report `--- SKIP` (PG_TEST_DSN not set).

- [ ] **Step 3: Commit**

```bash
git add databases/postgres/postgres_db_test.go
git commit -m "test: add gated Postgres integration tests

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Concurrency-safe `BackendState`

**Files:**
- Modify: `models/AvailableBackends.go`
- Create: `models/backendstate_test.go`

- [ ] **Step 1: Write the failing test**

Create `models/backendstate_test.go`:
```go
package models

import (
	"sync"
	"testing"
)

func TestBackendStateSnapshotIsolated(t *testing.T) {
	var s BackendState
	s.SetState([]string{"a", "b"}, []string{"a"})
	avail := s.SnapshotAvailable()
	avail[0] = "mutated"
	if got := s.SnapshotAvailable(); got[0] != "a" {
		t.Fatalf("snapshot should be a copy, got %v", got)
	}
}

func TestBackendStateIsUp(t *testing.T) {
	var s BackendState
	s.SetState(nil, []string{"a"})
	if !s.IsUp("a") {
		t.Fatal("a should be up")
	}
	if s.IsUp("b") {
		t.Fatal("b should not be up")
	}
}

func TestBackendStateConcurrent(t *testing.T) {
	var s BackendState
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); s.SetState([]string{"a"}, []string{"a"}) }()
		go func() { defer wg.Done(); _ = s.SnapshotUp() }()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./models/ -run TestBackendState -v`
Expected: compile failure — `undefined: BackendState`.

- [ ] **Step 3: Replace `AvailableBackends.go` with `BackendState`**

Replace the contents of `models/AvailableBackends.go`:
```go
package models

import (
	"slices"
	"sync"
)

// BackendState holds the per-instance view of which backends are up and which
// still have capacity (available). It is safe for concurrent use.
type BackendState struct {
	mu        sync.RWMutex
	up        []string
	available []string
}

// SetState atomically replaces both lists with copies of the provided slices.
func (b *BackendState) SetState(available, up []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.available = append([]string(nil), available...)
	b.up = append([]string(nil), up...)
}

// SnapshotAvailable returns a copy of the available list.
func (b *BackendState) SnapshotAvailable() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.available...)
}

// SnapshotUp returns a copy of the up list.
func (b *BackendState) SnapshotUp() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.up...)
}

// IsUp reports whether backend is currently up.
func (b *BackendState) IsUp(backend string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Contains(b.up, backend)
}
```

- [ ] **Step 4: Run the test (race detector) to verify it passes**

Run: `go test ./models/ -run TestBackendState -race -v`
Expected: all PASS, no race warnings. (Package `main` won't compile yet — that's fixed in Tasks 7–8.)

- [ ] **Step 5: Commit**

```bash
git add models/AvailableBackends.go models/backendstate_test.go
git commit -m "refactor: add RWMutex-guarded BackendState

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Prometheus metrics package

**Files:**
- Create: `metrics/metrics.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the Prometheus dependency**

Run: `go get github.com/prometheus/client_golang@latest`
Expected: `github.com/prometheus/client_golang` added to `go.mod`.

- [ ] **Step 2: Create the metrics package**

Create `metrics/metrics.go`:
```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts proxy requests by outcome:
	// proxied, no_backend, clash, resource_redirect.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "etherpad_proxy_requests_total",
		Help: "Total proxy requests by outcome.",
	}, []string{"outcome"})

	// BackendsUp is the number of backends currently reachable.
	BackendsUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "etherpad_proxy_backends_up",
		Help: "Number of backends currently up.",
	})

	// BackendsAvailable is the number of up backends still under capacity.
	BackendsAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "etherpad_proxy_backends_available",
		Help: "Number of backends currently available (up and under capacity).",
	})

	// PadAssignmentsTotal counts new pad-to-backend assignments.
	PadAssignmentsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_pad_assignments_total",
		Help: "Total number of new pad assignments.",
	})

	// ClashesTotal counts recorded pad clashes.
	ClashesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_clashes_total",
		Help: "Total number of recorded pad clashes.",
	})

	// DBErrorsTotal counts database operation errors.
	DBErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_db_errors_total",
		Help: "Total number of database errors.",
	})
)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./metrics/`
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add metrics/ go.mod go.sum
git commit -m "feat: add Prometheus metrics package

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Settings — management port + startup validation

**Files:**
- Modify: `models/Settings.go`
- Modify: `main.go`
- Create: `models/settings_test.go`

- [ ] **Step 1: Write the failing validation test**

Create `models/settings_test.go`:
```go
package models

import "testing"

func strPtr(s string) *string { return &s }

func validBackend() Backend {
	return Backend{Host: "localhost", Port: 9001}
}

func baseSettings() Settings {
	return Settings{
		Port:       9000,
		Backends:   map[string]Backend{"b1": validBackend()},
		DBSettings: DBSettings{Filename: "db/x.db"},
	}
}

func TestValidateOK(t *testing.T) {
	if err := baseSettings().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateBadPort(t *testing.T) {
	s := baseSettings()
	s.Port = 0
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestValidateNoBackends(t *testing.T) {
	s := baseSettings()
	s.Backends = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for no backends")
	}
}

func TestValidateBothDBSet(t *testing.T) {
	s := baseSettings()
	s.DBSettings = DBSettings{Filename: "db/x.db", Connstr: "postgres://x"}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when both filename and connstr set")
	}
}

func TestValidateNeitherDBSet(t *testing.T) {
	s := baseSettings()
	s.DBSettings = DBSettings{}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when neither filename nor connstr set")
	}
}

func TestValidatePartialBasicAuth(t *testing.T) {
	s := baseSettings()
	b := validBackend()
	b.Username = strPtr("admin") // password missing
	s.Backends = map[string]Backend{"b1": b}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for username without password")
	}
}

func TestValidatePartialOAuth(t *testing.T) {
	s := baseSettings()
	b := validBackend()
	b.ClientId = strPtr("id") // secret + tokenURL missing
	s.Backends = map[string]Backend{"b1": b}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for partial oauth config")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./models/ -run TestValidate -v`
Expected: compile failure — `s.Validate undefined`.

- [ ] **Step 3: Add `ManagementPort` and `Validate` to Settings**

Replace the contents of `models/Settings.go`:
```go
package models

import (
	"errors"
	"fmt"
)

type Settings struct {
	Port               int                `json:"port"`
	ManagementPort     int                `json:"managementPort"`
	Backends           map[string]Backend `json:"backends"`
	MaxPadsPerInstance int                `json:"maxPadsPerInstance"`
	CheckInterval      int64              `json:"checkInterval"`
	DBSettings         DBSettings         `json:"dbSettings"`
}

type DBSettings struct {
	Filename string `json:"filename"`
	Connstr  string `json:"postgresConnstr"`
}

type Backend struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	ClientId     *string  `json:"clientId"`
	ClientSecret *string  `json:"clientSecret"`
	Scopes       []string `json:"scopes"`
	TokenURL     *string  `json:"tokenUrl"`
	Username     *string  `json:"username"`
	Password     *string  `json:"password"`
}

func validPort(p int) bool { return p > 0 && p <= 65535 }

// Validate checks that the settings are internally consistent and usable.
func (s Settings) Validate() error {
	if !validPort(s.Port) {
		return fmt.Errorf("port must be between 1 and 65535, got %d", s.Port)
	}
	if s.ManagementPort != 0 && !validPort(s.ManagementPort) {
		return fmt.Errorf("managementPort must be between 1 and 65535, got %d", s.ManagementPort)
	}
	if len(s.Backends) == 0 {
		return errors.New("at least one backend must be configured")
	}
	for name, b := range s.Backends {
		if b.Host == "" {
			return fmt.Errorf("backend %q is missing host", name)
		}
		if !validPort(b.Port) {
			return fmt.Errorf("backend %q has invalid port %d", name, b.Port)
		}
		if (b.Username != nil) != (b.Password != nil) {
			return fmt.Errorf("backend %q must set both username and password", name)
		}
		hasOAuth := b.ClientId != nil && b.ClientSecret != nil && b.TokenURL != nil
		if (b.ClientId != nil || b.ClientSecret != nil || b.TokenURL != nil) && !hasOAuth {
			return fmt.Errorf("backend %q must set clientId, clientSecret and tokenUrl together", name)
		}
	}
	hasFile := s.DBSettings.Filename != ""
	hasConn := s.DBSettings.Connstr != ""
	if hasFile == hasConn {
		return errors.New("exactly one of dbSettings.filename or dbSettings.postgresConnstr must be set")
	}
	return nil
}
```

- [ ] **Step 4: Call `Validate` in `main`**

In `main.go`, after the `json.Unmarshal` block that fills `settingsData` (the `if err = json.Unmarshal(...)` block ending at the closing brace), and before `StartServer(settingsData, sugar)`, insert:
```go
	if err = settingsData.Validate(); err != nil {
		sugar.Fatalf("Invalid settings: %v", err)
	}

```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./models/ -v`
Expected: all `TestValidate*` PASS.

- [ ] **Step 6: Commit**

```bash
git add models/Settings.go models/settings_test.go main.go
git commit -m "feat: add managementPort setting and startup config validation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Refactor `proxyHandler.go` — testable routing, guarded statics, metrics

**Files:**
- Modify: `proxyHandler.go`
- Create: `proxyHandler_test.go`

- [ ] **Step 1: Rewrite `proxyHandler.go`**

Replace the entire contents of `proxyHandler.go`:
```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/metrics"
	"github.com/ether/etherpad-proxy/models"
	"github.com/ether/etherpad-proxy/ui"
	"go.uber.org/zap"
)

type StaticResource struct {
	Backend  string
	FullPath string
}

// staticResources is a concurrency-safe map of scraped static resource names.
type staticResources struct {
	mu sync.RWMutex
	m  map[string]StaticResource
}

func newStaticResources() *staticResources {
	return &staticResources{m: make(map[string]StaticResource)}
}

func (s *staticResources) set(name string, r StaticResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = r
}

func (s *staticResources) get(name string) (StaticResource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.m[name]
	return r, ok
}

func (s *staticResources) anyPath() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.m {
		return r.FullPath, true
	}
	return "", false
}

type ProxyHandler struct {
	p      map[string]httputil.ReverseProxy
	logger *zap.SugaredLogger
	db     interfaces.IDB
	state  *models.BackendState
	static *staticResources
}

type ResourceNotFound struct {
	newPath string
}

func (m *ResourceNotFound) Error() string { return "Resource not found" }

type ClashInPadId struct {
	padId string
}

func (m *ClashInPadId) Error() string { return "Pad clash" }

func ScrapeJSFiles(settings models.Settings, static *staticResources, logger *zap.SugaredLogger) {
	go func() {
		for {
			for key, backend := range settings.Backends {
				response, err := http.Get("http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/p/test")
				if err != nil {
					logger.Warnf("Error while scraping JS files: %v", err)
					continue
				}
				doc, err := goquery.NewDocumentFromReader(response.Body)
				if err != nil {
					logger.Warnf("Error parsing scraped document: %v", err)
					_ = response.Body.Close()
					continue
				}
				doc.Find("script").Each(func(_ int, s *goquery.Selection) {
					src, ok := s.Attr("src")
					if ok && strings.Contains(src, "padbootstrap") {
						parts := strings.Split(src, "/")
						name := parts[len(parts)-1]
						static.set(name, StaticResource{
							Backend:  key,
							FullPath: "http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/" + name,
						})
					}
				})
				if err = response.Body.Close(); err != nil {
					logger.Warnf("Error while closing response body: %v", err)
				}
			}
			time.Sleep(10 * time.Minute)
		}
	}()
}

// chooseBackend returns the backend key a request should be routed to, or an
// error (ResourceNotFound carries a redirect path; ClashInPadId signals an
// unresolved pad clash).
func (ph *ProxyHandler) chooseBackend(padId *string, r *http.Request) (string, error) {
	available := ph.state.SnapshotAvailable()
	up := ph.state.SnapshotUp()

	if padId == nil {
		if len(available) == 0 {
			return "", errors.New("no backends available")
		}
		if strings.Contains(r.URL.Path, "padbootstrap") {
			parts := strings.Split(r.URL.Path, "/")
			name := parts[len(parts)-1]
			if res, ok := ph.static.get(name); ok && slices.Contains(up, res.Backend) {
				return res.Backend, nil
			}
			if path, ok := ph.static.anyPath(); ok {
				return "", &ResourceNotFound{newPath: path}
			}
			return "", &ResourceNotFound{}
		}
		return available[rand.IntN(len(available))], nil
	}

	if len(available) == 0 {
		return "", errors.New("no backends available")
	}

	stored, err := ph.db.Get(*padId)
	if errors.Is(err, sql.ErrNoRows) {
		clashes, cerr := ph.db.GetClashByPadID(*padId)
		if cerr != nil && !errors.Is(cerr, sql.ErrNoRows) {
			metrics.DBErrorsTotal.Inc()
			return "", cerr
		}
		if len(clashes) == 0 {
			candidate := available[rand.IntN(len(available))]
			backend, aerr := ph.db.Assign(*padId, candidate)
			if aerr != nil {
				metrics.DBErrorsTotal.Inc()
				return "", aerr
			}
			metrics.PadAssignmentsTotal.Inc()
			return backend, nil
		}
		ph.logger.Warnf("Pad %s is in a clash with backends: %v", *padId, clashes)
		return "", &ClashInPadId{padId: *padId}
	} else if err != nil {
		metrics.DBErrorsTotal.Inc()
		return "", err
	}

	if slices.Contains(up, stored.Backend) {
		return stored.Backend, nil
	}
	if len(up) == 0 {
		return "", errors.New("no backends available")
	}
	newBackend := up[rand.IntN(len(up))]
	if serr := ph.db.Set(*padId, models.DBBackend{Backend: newBackend}); serr != nil {
		metrics.DBErrorsTotal.Inc()
		ph.logger.Info("Error while setting padId in DB: ", serr)
	}
	return newBackend, nil
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ph.logger.Debugf("%s %s", r.Method, r.URL)
	var padId *string

	if strings.Contains(r.URL.Path, "/p/") {
		afterP := strings.Split(r.URL.Path, "/p/")[1]
		beforeQuery := strings.Split(afterP, "?")[0]
		first := strings.Split(beforeQuery, "/")[0]
		padId = &first
		ph.logger.Infof("Initial request to /p/%s", first)
	}

	if padId == nil {
		if q := r.URL.Query().Get("padId"); q != "" {
			padId = &q
		}
	}

	backendKey, err := ph.chooseBackend(padId, r)
	if err != nil {
		var resourceNotFound *ResourceNotFound
		if errors.As(err, &resourceNotFound) && resourceNotFound.newPath != "" {
			metrics.RequestsTotal.WithLabelValues("resource_redirect").Inc()
			http.Redirect(w, r, resourceNotFound.newPath, http.StatusTemporaryRedirect)
			return
		}
		var clash *ClashInPadId
		if errors.As(err, &clash) {
			metrics.RequestsTotal.WithLabelValues("clash").Inc()
		} else {
			metrics.RequestsTotal.WithLabelValues("no_backend").Inc()
		}
		ph.logger.Error("Error while creating route: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		template := ui.Error()
		if rerr := template.Render(context.Background(), w); rerr != nil {
			ph.logger.Error("Error while rendering template: ", rerr)
		}
		return
	}

	proxy := ph.p[backendKey]
	metrics.RequestsTotal.WithLabelValues("proxied").Inc()
	proxy.ServeHTTP(w, r)
}
```

- [ ] **Step 2: Write the routing tests**

Create `proxyHandler_test.go`:
```go
package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httputil"
	"testing"

	"github.com/ether/etherpad-proxy/models"
	"go.uber.org/zap"
)

type fakeDB struct {
	pads        map[string]string
	clashes     map[string][]string
	assignCalls int
}

func newFakeDB() *fakeDB {
	return &fakeDB{pads: map[string]string{}, clashes: map[string][]string{}}
}

func (f *fakeDB) Close() error { return nil }
func (f *fakeDB) Get(id string) (*models.DBBackend, error) {
	if b, ok := f.pads[id]; ok {
		return &models.DBBackend{Backend: b}, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeDB) CleanUpPads(_ []string, _ string) error { return nil }
func (f *fakeDB) RecordClash(id string, data string) error {
	f.clashes[id] = append(f.clashes[id], data)
	return nil
}
func (f *fakeDB) Set(id string, backend models.DBBackend) error {
	f.pads[id] = backend.Backend
	return nil
}
func (f *fakeDB) GetAllPads() (map[string]string, error) { return f.pads, nil }
func (f *fakeDB) GetClashByPadID(id string) ([]string, error) {
	return f.clashes[id], nil
}
func (f *fakeDB) Assign(id string, candidate string) (string, error) {
	f.assignCalls++
	if b, ok := f.pads[id]; ok {
		return b, nil
	}
	f.pads[id] = candidate
	return candidate, nil
}

func newTestHandler(db *fakeDB, available, up []string) *ProxyHandler {
	state := &models.BackendState{}
	state.SetState(available, up)
	return &ProxyHandler{
		p:      map[string]httputil.ReverseProxy{},
		logger: zap.NewNop().Sugar(),
		db:     db,
		state:  state,
		static: newStaticResources(),
	}
}

func mustReq(path string) *http.Request {
	r, _ := http.NewRequest("GET", path, nil)
	return r
}

func TestChooseBackendNoBackends(t *testing.T) {
	h := newTestHandler(newFakeDB(), nil, nil)
	pad := "pad1"
	if _, err := h.chooseBackend(&pad, mustReq("/p/pad1")); err == nil {
		t.Fatal("expected error when no backends available")
	}
}

func TestChooseBackendAssignsNewPad(t *testing.T) {
	db := newFakeDB()
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b1" {
		t.Fatalf("expected b1, got %s", got)
	}
	if db.assignCalls != 1 {
		t.Fatalf("expected Assign called once, got %d", db.assignCalls)
	}
}

func TestChooseBackendUsesStoredWhenUp(t *testing.T) {
	db := newFakeDB()
	db.pads["pad1"] = "b2"
	h := newTestHandler(db, []string{"b1"}, []string{"b1", "b2"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b2" {
		t.Fatalf("expected stored backend b2, got %s", got)
	}
}

func TestChooseBackendReassignsWhenStoredDown(t *testing.T) {
	db := newFakeDB()
	db.pads["pad1"] = "down"
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b1" {
		t.Fatalf("expected reassign to b1, got %s", got)
	}
	if db.pads["pad1"] != "b1" {
		t.Fatalf("expected DB updated to b1, got %s", db.pads["pad1"])
	}
}

func TestChooseBackendClash(t *testing.T) {
	db := newFakeDB()
	db.clashes["pad1"] = []string{"b1", "b2"}
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	_, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	var clash *ClashInPadId
	if !errors.As(err, &clash) {
		t.Fatalf("expected ClashInPadId, got %v", err)
	}
}
```

- [ ] **Step 3: Verify package main still does not compile (runtime.go not yet updated)**

Run: `go build ./...`
Expected: failure referencing `runtime.go` (old `AvailableBackends` global / old `ScrapeJSFiles` signature). This is expected — Task 7 fixes `runtime.go`. Do NOT commit yet.

- [ ] **Step 4: (Deferred) tests run after Task 7**

Note: `proxyHandler_test.go` is committed together with the `runtime.go` refactor in Task 7 so the package compiles. Leave changes staged-but-uncommitted, or proceed directly to Task 7 and commit both together.

---

## Task 7: Refactor `runtime.go` — wiring, graceful shutdown, health, metrics endpoint

**Files:**
- Modify: `runtime.go`

- [ ] **Step 1: Rewrite `runtime.go`**

Replace the entire contents of `runtime.go`:
```go
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ether/etherpad-proxy/databases"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/metrics"
	"github.com/ether/etherpad-proxy/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const defaultManagementPort = 8081

func checkAvailabilityLoop(settings models.Settings, state *models.BackendState, _ *zap.SugaredLogger) {
	timerP := time.Duration(settings.CheckInterval) * time.Millisecond
	go func() {
		for {
			response := checkAvailability(settings)
			state.SetState(response.Available, response.Up)
			metrics.BackendsAvailable.Set(float64(len(response.Available)))
			metrics.BackendsUp.Set(float64(len(response.Up)))
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpadsLoop(settings models.Settings, logger *zap.SugaredLogger, db interfaces.IDB, state *models.BackendState) {
	timerP := time.Duration(settings.CheckInterval) * time.Second
	timerBefore := 5 * time.Second
	go func() {
		for {
			time.Sleep(timerBefore)
			cleanUpEtherpads(settings, logger, db, state)
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpads(settings models.Settings, logger *zap.SugaredLogger, db interfaces.IDB, state *models.BackendState) {
	upBackends := state.SnapshotUp()
	mapOfPadsToBackends := make(map[string]string)
	for _, backend := range upBackends {
		foundBackend := settings.Backends[backend]
		var authorizationHeader string
		if foundBackend.Username != nil && foundBackend.Password != nil {
			authorizationHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(*foundBackend.Username+":"+*foundBackend.Password))
		} else if foundBackend.ClientId != nil && foundBackend.ClientSecret != nil {
			conf := &clientcredentials.Config{
				ClientID:     *foundBackend.ClientId,
				ClientSecret: *foundBackend.ClientSecret,
				Scopes:       foundBackend.Scopes,
				TokenURL:     *foundBackend.TokenURL,
				AuthStyle:    oauth2.AuthStyleInHeader,
			}
			token, err := conf.Token(context.Background())
			if err != nil {
				logger.Warnf("Error getting token: %v", err)
				continue
			}
			authorizationHeader = "Bearer " + token.AccessToken
		} else {
			logger.Info("No authentication method found for backend: ", backend)
			continue
		}

		client := &http.Client{}
		req, _ := http.NewRequest("GET", "http://"+foundBackend.Host+":"+strconv.Itoa(foundBackend.Port)+"/api/1.3.0/listAllPads", nil)
		req.Header.Set("Authorization", authorizationHeader)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			logger.Warnf("Error retrieving etherpads: %v from %s", err, backend)
			continue
		}
		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			logger.Warnf("Error reading response body: %v", err)
			_ = res.Body.Close()
			continue
		}
		var response models.ListAllPadsModel
		if err = json.Unmarshal(bytes, &response); err != nil {
			logger.Warnf("Error unmarshalling response: %v", err)
			_ = res.Body.Close()
			continue
		}
		if response.Code != 0 {
			logger.Warnf("Error retrieving etherpads: %v", response.Message)
			_ = res.Body.Close()
			continue
		}
		for _, pad := range response.Data.PadIds {
			if entry, ok := mapOfPadsToBackends[pad]; ok {
				logger.Warnf("Pad %s already exists on backend %s", pad, entry)
				if err = db.RecordClash(pad, backend); err != nil {
					metrics.DBErrorsTotal.Inc()
					logger.Warnf("Error recording clash: %v", err)
				} else {
					metrics.ClashesTotal.Inc()
				}
				continue
			}
			mapOfPadsToBackends[pad] = backend
		}
		if err = res.Body.Close(); err != nil {
			logger.Warnf("Error closing response body: %v", err)
		}
	}

	backendToPads := make(map[string][]string)
	for pad, backend := range mapOfPadsToBackends {
		backendToPads[backend] = append(backendToPads[backend], pad)
	}
	for backend, pads := range backendToPads {
		if err := db.CleanUpPads(pads, backend); err != nil {
			metrics.DBErrorsTotal.Inc()
			logger.Warnf("Error cleaning up etherpads: %v", err)
		}
	}
	for pad, backend := range mapOfPadsToBackends {
		if err := db.Set(pad, models.DBBackend{Backend: backend}); err != nil {
			metrics.DBErrorsTotal.Inc()
			logger.Warnf("Error setting etherpad: %v", err)
		}
	}
}

func StartServer(settings models.Settings, logger *zap.SugaredLogger) {
	db, err := databases.CreateNewDatabase(settings)
	if err != nil {
		logger.Fatalf("Error opening database: %v", err)
	}

	state := &models.BackendState{}
	static := newStaticResources()

	checkAvailabilityLoop(settings, state, logger)
	cleanUpEtherpadsLoop(settings, logger, db, state)
	ScrapeJSFiles(settings, static, logger)

	proxies := make(map[string]httputil.ReverseProxy)
	for key, backend := range settings.Backends {
		proxyURL, perr := url.Parse("http://" + backend.Host + ":" + strconv.Itoa(backend.Port))
		if perr != nil {
			logger.Fatalf("Error parsing backend URL for %s: %v", key, perr)
		}
		proxies[key] = *httputil.NewSingleHostReverseProxy(proxyURL)
	}

	handler := &ProxyHandler{
		p:      proxies,
		logger: logger,
		db:     db,
		state:  state,
		static: static,
	}

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", handler.ServeHTTP)
	proxySrv := &http.Server{Addr: ":" + strconv.Itoa(settings.Port), Handler: proxyMux}

	managementPort := settings.ManagementPort
	if managementPort == 0 {
		managementPort = defaultManagementPort
	}
	adminMux := http.NewServeMux()
	adminMux.Handle("/pads", &AdminPanel{DB: db, logger: logger})
	adminMux.Handle("/metrics", promhttp.Handler())
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	adminMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if len(state.SnapshotUp()) > 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mgmtSrv := &http.Server{Addr: ":" + strconv.Itoa(managementPort), Handler: adminMux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting management server on port ", managementPort)
		if serr := mgmtSrv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			logger.Fatalf("Error starting management server: %v", serr)
		}
	}()
	go func() {
		logger.Info("Starting server on port ", settings.Port)
		if serr := proxySrv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			logger.Fatalf("Error starting server: %v", serr)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutdown signal received, draining...")
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if serr := proxySrv.Shutdown(shutdownCtx); serr != nil {
		logger.Warnf("Proxy server shutdown error: %v", serr)
	}
	if serr := mgmtSrv.Shutdown(shutdownCtx); serr != nil {
		logger.Warnf("Management server shutdown error: %v", serr)
	}
	if cerr := db.Close(); cerr != nil {
		logger.Warnf("Database close error: %v", cerr)
	}
	logger.Info("Shutdown complete")
}
```

- [ ] **Step 2: Build the whole module**

Run: `go build ./...`
Expected: success (no output).

- [ ] **Step 3: Run the full test suite with the race detector**

Run: `go test ./... -race`
Expected: all PASS (Postgres tests SKIP without `PG_TEST_DSN`), no race warnings.

- [ ] **Step 4: Vet**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 5: Commit (includes Task 6 changes)**

```bash
git add proxyHandler.go proxyHandler_test.go runtime.go
git commit -m "feat: race-free routing, graceful shutdown, health/metrics endpoints

- guard static resource map and use BackendState snapshots (no data races)
- extract testable chooseBackend; use atomic Assign for new pads
- http.Server with signal-driven graceful shutdown
- /healthz, /readyz and /metrics on the management port
- structured zap logging; record metrics on routing and DB outcomes

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `checkAvailability` test

**Files:**
- Create: `checkAvailability_test.go`

- [ ] **Step 1: Write the test using `httptest` backends**

Create `checkAvailability_test.go`:
```go
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/ether/etherpad-proxy/models"
)

// newStatsBackend starts a test server replying to /stats with the given
// activePads count, and returns its host and port.
func newStatsBackend(t *testing.T, activePads int) (string, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stats" {
			fmt.Fprintf(w, `{"activePads": %d}`, activePads)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return u.Hostname(), port
}

func TestCheckAvailabilityCapacityAndDown(t *testing.T) {
	okHost, okPort := newStatsBackend(t, 1)   // under capacity -> up + available
	fullHost, fullPort := newStatsBackend(t, 99) // over capacity -> up but not available

	settings := models.Settings{
		MaxPadsPerInstance: 5,
		Backends: map[string]models.Backend{
			"ok":   {Host: okHost, Port: okPort},
			"full": {Host: fullHost, Port: fullPort},
			"down": {Host: "127.0.0.1", Port: 1}, // nothing listening -> not up
		},
	}

	got := checkAvailability(settings)

	if !slices.Contains(got.Up, "ok") || !slices.Contains(got.Up, "full") {
		t.Fatalf("expected ok and full to be up, got %v", got.Up)
	}
	if slices.Contains(got.Up, "down") {
		t.Fatalf("down backend should not be up, got %v", got.Up)
	}
	if !slices.Contains(got.Available, "ok") {
		t.Fatalf("ok should be available, got %v", got.Available)
	}
	if slices.Contains(got.Available, "full") {
		t.Fatalf("full backend should not be available, got %v", got.Available)
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./ -run TestCheckAvailability -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add checkAvailability_test.go
git commit -m "test: cover backend availability checks

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: CI test workflow

**Files:**
- Create: `.github/workflows/test.yml`

- [ ] **Step 1: Create the workflow**

Create `.github/workflows/test.yml`:
```yaml
name: Test
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: proxy
          POSTGRES_PASSWORD: proxy
          POSTGRES_DB: etherpad_proxy
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U proxy"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Vet
        run: go vet ./...
      - name: Test (race)
        env:
          PG_TEST_DSN: postgres://proxy:proxy@localhost:5432/etherpad_proxy?sslmode=disable
        run: go test -race ./...
```

- [ ] **Step 2: Validate YAML locally**

Run: `python -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml')); print('OK')"`
Expected: `OK` (YAML parses). If Python/PyYAML is unavailable, visually confirm indentation matches the block above.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "ci: run go vet and race tests with a Postgres service

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Installation-without-Docker docs + systemd unit (#52)

**Files:**
- Create: `support/etherpad-proxy.service`
- Modify: `README.md`

- [ ] **Step 1: Create the systemd unit**

Create `support/etherpad-proxy.service`:
```ini
[Unit]
Description=Etherpad sharding reverse proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=etherpad-proxy
Group=etherpad-proxy
WorkingDirectory=/opt/etherpad-proxy
Environment=SETTINGS_FILE=/opt/etherpad-proxy/settings.json
ExecStart=/opt/etherpad-proxy/etherpad-proxy
Restart=on-failure
RestartSec=5
# Hardening
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Add the docs section to README**

In `README.md`, insert the following block immediately after the `## Getting started for production` section (i.e. before `## Settings`):

````markdown
## Installation without Docker

The proxy is a single static Go binary. You can run it directly on a host
without Docker.

### Option A: download a prebuilt binary

1. Go to the [Releases page](https://github.com/ether/etherpad-proxy/releases)
   and download the archive for your OS/architecture.
2. Extract it: `tar xzf etherpad-proxy_*.tar.gz` (or unzip on Windows).
3. Continue with **Configure** below.

### Option B: build from source

Requires Go 1.24 or newer.

```bash
git clone https://github.com/ether/etherpad-proxy.git
cd etherpad-proxy
go build -o etherpad-proxy .
```

### Configure

1. Copy the template and edit it:
   ```bash
   cp settings.json.template settings.json
   ```
2. Set `port` (proxy listen port), optionally `managementPort`
   (default `8081`, serves `/pads`, `/metrics`, `/healthz`, `/readyz`), your
   `backends`, and a database (see **Database** below).
3. The settings path defaults to `./settings.json`; override it with the
   `SETTINGS_FILE` environment variable.

### Run

```bash
SETTINGS_FILE=/path/to/settings.json ./etherpad-proxy
```

The proxy listens on `port`; management/metrics/health endpoints listen on
`managementPort`.

### Database

- **SQLite (default, single instance):** set `dbSettings.filename`, e.g.
  `"db/etherpad-proxy.db"`. No external services required. Create the directory
  first: `mkdir -p db`.
- **Postgres (recommended for multiple proxy instances):** set
  `dbSettings.postgresConnstr`. With a shared Postgres, several proxy instances
  share routing state and assign new pads atomically, so they never route the
  same pad to different backends.

  Provision a database and user, for example:
  ```sql
  CREATE USER proxy WITH PASSWORD 'changeme';
  CREATE DATABASE etherpad_proxy OWNER proxy;
  ```
  Then set:
  ```json
  "dbSettings": {
    "postgresConnstr": "postgres://proxy:changeme@db-host:5432/etherpad_proxy?sslmode=disable"
  }
  ```
  The proxy creates its tables automatically on first start. Set exactly one of
  `filename` or `postgresConnstr`.

### Run as a systemd service (Linux)

A sample unit is provided at `support/etherpad-proxy.service`.

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin etherpad-proxy
sudo mkdir -p /opt/etherpad-proxy/db
sudo cp etherpad-proxy /opt/etherpad-proxy/
sudo cp settings.json /opt/etherpad-proxy/
sudo cp support/etherpad-proxy.service /etc/systemd/system/
sudo chown -R etherpad-proxy:etherpad-proxy /opt/etherpad-proxy
sudo systemctl daemon-reload
sudo systemctl enable --now etherpad-proxy
```

Check status and logs:
```bash
systemctl status etherpad-proxy
journalctl -u etherpad-proxy -f
```
````

- [ ] **Step 3: Verify the README renders sensibly**

Run: `git diff --stat README.md`
Expected: README.md modified. Visually confirm the new section sits between
"Getting started for production" and "Settings".

- [ ] **Step 4: Commit**

```bash
git add README.md support/etherpad-proxy.service
git commit -m "docs: add installation without Docker, Postgres setup, systemd unit (#52)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: GoReleaser config + release workflow

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the GoReleaser config**

Create `.goreleaser.yaml`:
```yaml
version: 2
project_name: etherpad-proxy
builds:
  - id: etherpad-proxy
    main: .
    binary: etherpad-proxy
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
archives:
  - id: default
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - README.md
      - LICENSE.md
      - settings.json.template
      - support/etherpad-proxy.service
checksum:
  name_template: 'checksums.txt'
release:
  github:
    owner: ether
    name: etherpad-proxy
```

- [ ] **Step 2: Validate the config**

Run: `go run github.com/goreleaser/goreleaser/v2@latest check`
Expected: `command finished successfully` / no config errors. (If network-restricted, skip; the release workflow validates on CI.)

- [ ] **Step 3: Create the release workflow**

Create `.github/workflows/release.yml`:
```yaml
name: Release
on:
  push:
    tags:
      - 'v*'
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml .github/workflows/release.yml
git commit -m "ci: add GoReleaser config and tag-triggered release workflow

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Step 1: Full build, vet, race tests**

Run:
```bash
go build ./...
go vet ./...
go test -race ./...
```
Expected: build clean; vet silent; all tests PASS (Postgres SKIP without DSN), no race warnings.

- [ ] **Step 2: Smoke-test the running binary**

Run (PowerShell):
```powershell
go build -o etherpad-proxy.exe .
# With a settings.json pointing at a SQLite file and at least one backend:
$env:SETTINGS_FILE="settings.json"; ./etherpad-proxy.exe
```
In another shell, confirm management endpoints:
```bash
curl -i http://localhost:8081/healthz   # 200
curl -i http://localhost:8081/readyz    # 503 until a backend is up, then 200
curl -s http://localhost:8081/metrics | grep etherpad_proxy   # metric lines present
```
Then send SIGINT (Ctrl+C) and confirm the logs show "Shutdown complete".

- [ ] **Step 3: Confirm `lib/pq` is gone**

Run: `grep -r "lib/pq" go.mod go.sum` 
Expected: no matches in `go.mod` (may remain transitively in `go.sum` only if another dep needs it; the direct require must be gone).
