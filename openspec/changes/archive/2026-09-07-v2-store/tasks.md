# Tasks: v2-store

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1300 |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR (size:exception) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

`size:exception` pre-approved (200000).

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Full slice | Single PR | `go test ./... -race -count=1` | `bash scripts/verify-store-boundary.sh` | Revert `internal/store/*`, `contract/*`, `report.go`, `scripts/verify-store-boundary.sh` |

## Phase 1: Migrations

- [x] 1.1 RED `internal/store/migrations_store_test.go`: `TestMigrationsExactReferenceSchema` 11 tables + `TestMigrationIdempotentPreservesRows` — fails pre-GREEN. D07 F-02.
- [x] 1.2 RED `internal/store/migrate_rollback_test.go`: bad stmt via `migrateWithStatements` mid-list; rollback clean, retry ok — fails pre-txn. D07 U-02.
- [x] 1.3 GREEN `internal/store/migrations.go`: `migrationStatements()` verbatim `CREATE TABLE IF NOT EXISTS leases (execution_id TEXT NOT NULL, step_id TEXT NOT NULL, holder TEXT NOT NULL, fencing_token INTEGER NOT NULL, acquired_at TEXT NOT NULL, expires_at TEXT NOT NULL, PRIMARY KEY (execution_id, step_id));` + `CREATE TABLE IF NOT EXISTS interactions (id TEXT PRIMARY KEY, attempt_id TEXT NOT NULL REFERENCES attempts(id), type TEXT NOT NULL CHECK (type IN ('permission','question')), status TEXT NOT NULL CHECK (status IN ('pending','resolved')), decision TEXT, idempotency_key TEXT NOT NULL, resolved_at TEXT, UNIQUE (attempt_id, idempotency_key));` + `migrateWithStatements` txn. D07 F-02/U-02.

## Phase 2: Sentinels & Repos

- [x] 2.1 GREEN `internal/store/store.go`: sentinels `ErrNotFound`/`ErrCheckViolation`/`ErrUniqueViolation`/`ErrForeignKeyViolation` + `normalizeSQLiteError` (`sql.ErrNoRows`→`ErrNotFound`, 275/787→FK, 1555/2067→UNIQUE); DSN `file:%s?cache=shared&_pragma=foreign_keys(1)` + WAL. D07.
- [x] 2.2 GREEN `internal/store/repositories.go`: wrap `Get`/Scan/exec via `normalizeSQLiteError` (helper in `store.go` applied here+`claim.go`+`transport.go`). D07.
- [x] 2.3 RED→GREEN `internal/store/leases.go` + `store.go:Leases()`: RED `TestLeaseMonotonicFencing` 1,2,3 stale→`ErrNotFound`; GREEN `Acquire` `Conn` `BEGIN IMMEDIATE` `MAX+1` init 1 retained, `Renew`/`Release` `RowsAffected==1` else `ErrNotFound`. D07 F-02.
- [x] 2.4 RED→GREEN `internal/store/interactions.go` + `store.go:Interactions()`: RED `TestInteractionCASIdempotent`; GREEN `Create/Get/Resolve` CAS pending+key, same key idempotent else `ErrUniqueViolation`. D07 F-02/U-03.
- [x] 2.5 GREEN `internal/store/claim.go`: `PRAGMA foreign_keys=1` on `Conn` before `BEGIN IMMEDIATE`; normalize errors. D07 F-03.

## Phase 3: Fake & Contract

- [x] 3.1 GREEN `internal/store/fake.go` (no `database/sql`/`modernc.org/sqlite`): mutex maps 11 tables, CHECK/UNIQUE/FK→sentinels, RFC3339 UTC, monotonic cursors, copy-on-write `WithTx`. D07 U-01.
- [x] 3.2 GREEN `internal/store/contract/suite.go`: `Run(t,Factory)` state/rollback, leases, claims, events/cursors, interactions. D07 U-01.
- [x] 3.3 GREEN `internal/store/interchangeable_test.go` (`store_test`): `FakeStore` vs `Open(t.TempDir()+"/store.db")` invoke `contract.Run`. D07 U-01.

## Phase 4: Boundary & Integration

- [x] 4.1 RED→GREEN `internal/execution/report.go`: drop `database/sql`, use `errors.Is(err,store.ErrNotFound)`. D07 F-01.
- [x] 4.2 GREEN `scripts/verify-store-boundary.sh` (+x): `git grep -n "database/sql\|modernc.org/sqlite" -- ':!internal/store/**'` fails on hit. D07 F-01.
- [x] 4.3 GREEN `internal/store/*_test.go`: `TestStoredTimestampsUTC` RFC3339 Z; `TestConstraintsParity` enums/duplicate/FK both backends→sentinels; `TestMigrationsVerifySoleOwners` 11-table `path_claims`/`attempt_transport`/`dag_hash`/`base_commit` not redefined. D07 F-03/U-03.
- [x] 4.4 Guardrail: no broker/PTY/renew-loop/distribution; out-of-scope excluded.

## Phase 5: Cleanup

- [x] 5.1 `go build ./...`
- [x] 5.2 `go vet ./...`
- [x] 5.3 `go test ./... -race -count=1`
- [x] 5.4 `golangci-lint run` if available

## Non-Goals

No broker, expiry timers, PTY, reporting, distribution, sole-owner redefinition.
