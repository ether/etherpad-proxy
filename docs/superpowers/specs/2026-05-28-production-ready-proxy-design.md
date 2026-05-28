# Production-Ready Etherpad Proxy — Design

Date: 2026-05-28
Issues: [#54 — does the etherpad-proxy itself scale?](https://github.com/ether/etherpad-proxy/issues/54), [#52 — Installation without docker](https://github.com/ether/etherpad-proxy/issues/52)

## Goal

Make etherpad-proxy a reverse proxy you can rely on in production: a fleet of
proxy instances can share routing state correctly through a shared database,
the process is observable and operable, the code is free of data races, it is
covered by tests, and it can be installed and run without Docker.

## Background: current state

The proxy shards pads across a pool of Etherpad backends. It maps `padId →
backend` in a database so the same pad always lands on the same backend. A
background loop health-checks backends (`/stats`) to know which are *up* and
which still have capacity (*available*), and another loop reconciles the DB
against each backend's real pad list (`listAllPads`), recording *clashes* when
the same pad exists on more than one backend.

PR #57 added a Postgres backend to the DB layer with the intent of letting
multiple proxy instances share state (issue #54). However, that support is
**non-functional** and the routing logic has correctness and concurrency
defects that block reliable multi-instance operation.

## Problems this design fixes

### Scaling correctness (#54)

1. **Postgres driver is never registered.** `databases/postgres` calls
   `sql.Open("postgres", …)` but nothing blank-imports a driver, so it fails at
   runtime with `sql: unknown driver "postgres"`.
2. **SQLite-only SQL is sent to Postgres.** `RecordClash` and `Set` use
   `INSERT OR REPLACE … VALUES (?, ?)`; the squirrel queries use `?`
   placeholders. Postgres requires `INSERT … ON CONFLICT …` and `$N`
   placeholders. Writes would fail even with a driver loaded.
3. **Pad assignment races across proxies (split-brain).** When a pad has no
   stored backend, each proxy independently picks a *random* backend and writes
   it (last-write-wins). Two proxies handling the same new pad route it to
   **different** backends — producing the exact clash the system is meant to
   avoid.

### Concurrency (data races)

4. `AvailableBackends.Available` / `.Up` are read **without holding the mutex**
   in several places in `proxyHandler.go` (e.g. the `len(...)` guard and the
   `slices.Index` lookup), while a background goroutine reassigns them under the
   lock. This is a data race.
5. `StaticResourceMap` (package-level `map`) is written by the `ScrapeJSFiles`
   goroutine and read in `createRoute` with no synchronization — a data race.
6. `cleanUpEtherpads` holds `AvailableBackends.Mutex` across slow outbound HTTP
   calls, serializing the health loop against request routing for the duration
   of network I/O.

### Operability

7. No graceful shutdown — the process can't drain in-flight requests or close
   the DB cleanly.
8. No health/readiness endpoint for load balancers and orchestrators.
9. No startup config validation — misconfiguration surfaces as obscure runtime
   failures.
10. Ad-hoc `log.Println` / debug prints instead of the structured zap logger.
11. Management port is a hardcoded constant (`8081`).
12. No metrics.

### Quality

13. Zero tests; CI only builds a Docker image.

### Installation (#52)

14. No documented way to install/run without Docker; no prebuilt binaries.

## Design

### 1. Database layer — drivers, dialects, atomic assignment

- **Driver:** replace `github.com/lib/pq` with `github.com/jackc/pgx/v5` used
  through its `database/sql` adapter. Blank-import
  `_ "github.com/jackc/pgx/v5/stdlib"` in the postgres package; open with driver
  name `"pgx"`. Remove `lib/pq` from `go.mod`. The existing `database/sql` +
  squirrel code is otherwise unchanged.
- **Dialect-correct SQL:** each implementation builds squirrel statements with
  the right placeholder format — `sq.Dollar` for Postgres, `sq.Question` for
  SQLite — and uses upserts instead of `INSERT OR REPLACE`:
  - pad: `INSERT INTO pad (id, backend) VALUES (…) ON CONFLICT (id) DO UPDATE SET backend = EXCLUDED.backend`
  - clashes: `INSERT INTO clashes (id, data) VALUES (…) ON CONFLICT (id, data) DO NOTHING`
  SQLite (modernc) supports `ON CONFLICT` / `EXCLUDED`, so both backends use the
  same statement shape, differing only in placeholder format.
- **Atomic assignment (approach A).** Add to `IDB`:

  ```go
  // Assign stores `candidate` as the backend for `padId` only if no backend is
  // already stored, and returns the backend that is now authoritative (the
  // existing one if there was a race, otherwise `candidate`).
  Assign(padId string, candidate string) (string, error)
  ```

  Implemented as `INSERT … ON CONFLICT (id) DO NOTHING` followed by a
  `SELECT backend FROM pad WHERE id = …`. The first proxy to win the insert
  decides the backend; concurrent proxies read the winner. This removes the
  split-brain window and works identically on SQLite and Postgres.

- `createRoute` uses `Assign` wherever it currently does "pick random backend +
  `Set`" for an unassigned pad. The "stored backend is no longer up → reassign"
  path keeps using `Set` (an intentional overwrite).

The `IDB` interface gains `Assign`; both implementations and the fake used in
tests implement it. `databases.CreateNewDatabase` selection logic is unchanged.

### 2. Concurrency-safe shared state

- Introduce a `BackendState` type (in `models`) wrapping `up`/`available`
  string slices behind a `sync.RWMutex`, exposing:
  - `SetState(available, up []string)`
  - `SnapshotAvailable() []string` / `SnapshotUp() []string` (return copies)
  - small helpers like `AvailableCount()` / `IsUp(backend string) bool`
  Replace the bare `AvailableBackends` global and all manual `Mutex.Lock()`
  call sites. Routing takes one consistent snapshot per request.
- Wrap `StaticResourceMap` in a `sync.RWMutex`-guarded type (or `sync.Map`) with
  `Get`/`Set`/`Snapshot`. Scraper writes, router reads — both synchronized.
- `cleanUpEtherpads` snapshots the up-list under the lock, releases it, then
  performs HTTP I/O.
- The whole suite runs under `go test -race` in CI.

### 3. Operability

- **Graceful shutdown:** build explicit `http.Server` values for the proxy and
  management listeners. `main` derives a context via
  `signal.NotifyContext(ctx, SIGINT, SIGTERM)`; on signal, call
  `Shutdown(timeoutCtx)` on both servers and `db.Close()`.
- **Health endpoints** on the management server (the proxy port is a `/`
  catch-all and can't host them):
  - `GET /healthz` → 200 always (process is alive).
  - `GET /readyz` → 200 if ≥1 backend is up, else 503.
- **Config validation** in a `Settings.Validate() error`, called in `main`
  before `StartServer`: port in range; ≥1 backend; each backend has host and
  valid port; exactly one of `filename` / `postgresConnstr` set; per-backend
  auth coherent (basic = username+password, oauth = clientId+clientSecret+
  tokenURL, or none). Fail fast with a clear message.
- **Structured logging:** route request-path and scraper logging through the
  existing zap `SugaredLogger`; remove `log.Println` and the `doc.Find` debug
  prints.
- **Configurable management port:** add `managementPort` to `Settings`
  (default `8081` when zero).

### 4. Metrics (Prometheus)

- Add `github.com/prometheus/client_golang`. Register a `promhttp` handler at
  `GET /metrics` on the management server.
- Metrics:
  - `etherpad_proxy_requests_total{outcome}` — counter (outcome ∈
    proxied / no_backend / clash / resource_redirect / error).
  - `etherpad_proxy_backends_up` / `etherpad_proxy_backends_available` — gauges,
    set by the availability loop.
  - `etherpad_proxy_pad_assignments_total` — counter, incremented on a new
    assignment.
  - `etherpad_proxy_clashes_total` — counter, incremented when a clash is
    recorded.
  - `etherpad_proxy_db_errors_total` — counter, incremented on DB errors.
- Metrics live in a small `metrics` package (or `metrics.go`) so handlers and
  loops can update them without import cycles.

### 5. Tests + CI

- **Unit tests:**
  - DB layer against in-memory SQLite (`file::memory:?cache=shared`):
    `Get`/`Set`/`Assign` (including the conflict path)/`CleanUpPads`/clashes.
  - `createRoute` routing decisions against a fake `IDB` and a seeded
    `BackendState`: unassigned pad, assigned-and-up, assigned-but-down
    (reassign), clash, no-backends-available, static-resource paths.
  - `checkAvailability` against `httptest` backends returning crafted `/stats`.
  - `Settings.Validate` table tests.
- **Postgres integration test** gated on `PG_TEST_DSN`; `t.Skip` when unset so
  local runs don't require Postgres. Exercises the same DB contract as SQLite.
- **CI:** new `.github/workflows/test.yml` running `go vet ./...` and
  `go test -race ./...` on a matrix; provides a `postgres` service container and
  sets `PG_TEST_DSN` for the integration test. Existing `build.yml` (Docker
  image) is unchanged.

### 6. Installation without Docker (#52) + releases

- **README "Installation without Docker" section** covering:
  - Prerequisites: Go 1.24+ (for building) or a downloaded release binary.
  - Build from source: `go build -o etherpad-proxy .`
  - Configuration: copy `settings.json.template` → `settings.json`; the
    `SETTINGS_FILE` env var overrides the path.
  - Running: `./etherpad-proxy`; proxy on `port`, management/metrics/health on
    `managementPort`.
  - Database choice: SQLite default (zero setup) vs Postgres for multi-instance
    deployments, including provisioning steps and the `postgresConnstr` format.
- **systemd:** add `support/etherpad-proxy.service` (a sample unit running a
  dedicated user, `WorkingDirectory`, `SETTINGS_FILE`, restart policy) with
  install instructions in the README.
- **Prebuilt binaries:** add `.goreleaser.yaml` building
  linux/darwin/windows × amd64/arm64 archives, and
  `.github/workflows/release.yml` triggered on `v*` tags that runs GoReleaser
  and publishes to GitHub Releases (`contents: write`). README documents
  downloading and running a release binary.

## Out of scope (YAGNI)

- Rate limiting, TLS termination (handled by an upstream LB / ingress), and
  authentication on the admin panel.
- Distributed coordination beyond the shared DB (no leader election, no gossip)
  — the shared DB + atomic assignment is sufficient for correctness.

## Risks / notes

- `ON CONFLICT` requires the `pad.id` and `clashes.(id,data)` primary keys that
  already exist in both schemas — verified in the current `CREATE TABLE`
  statements.
- Switching the Postgres DSN scheme: pgx's stdlib accepts the same
  `postgres://…` URLs as lib/pq, so existing `postgresConnstr` values keep
  working; documented regardless.
- The availability loop currently swaps whole slices; `BackendState` preserves
  that semantics (atomic replace), only adding safe reads.
