# Apply Progress: v2-store

**Change**: v2-store (D07)
**Mode**: Strict TDD
**Delivery**: single-pr size:exception (pre-approved 200000)
**Artifact store**: hybrid (Engram + OpenSpec)

## Task Progress

### Phase 1: Migrations — ✅ Complete
- [x] 1.1 RED `migrations_store_test.go` — TestMigrationsExactReferenceSchema + TestMigrationIdempotentPreservesRows written first, failed pre-GREEN (undefined seam), passed after GREEN.
- [x] 1.2 RED `migrate_rollback_test.go` — Injected bad stmt mid-list, proved rollback cleans schema and retry succeeds; failed pre-txn (undefined migrateWithStatements), passed after txn wrapper.
- [x] 1.3 GREEN `migrations.go` — Added leases + interactions verbatim DDL with IF NOT EXISTS, migrationStatements() + migrateWithStatements txn (BeginTx loop commit/rollback). Verified sole-owners untouched.

### Phase 2: Sentinels & Repos — ✅ Complete
- [x] 2.1 GREEN `store.go` — Sentinels ErrNotFound/ErrCheckViolation/ErrUniqueViolation/ErrForeignKeyViolation + normalizeSQLiteError (sql.ErrNoRows→ErrNotFound, 275 CHECK, 787 FK, 1555/2067 UNIQUE, string fallback) + DSN file:%s?cache=shared&_pragma=foreign_keys(1) + WAL pragma retained. Store facade extended with Leases() and Interactions().
- [x] 2.2 GREEN `repositories.go` — All Get/Scan/exec paths wrapped via normalizeSQLiteError; applied same helper in claim.go/transport.go. Import database/sql confined to internal/store.
- [x] 2.3 RED→GREEN `leases.go` — RED TestLeaseMonotonicFencing 1,2,3 + stale holder/token → ErrNotFound failed with stub; GREEN Acquire via dedicated Conn PRAGMA foreign_keys=1 BEGIN IMMEDIATE MAX+1 retained init 1, Renew/Release RowsAffected==1 else ErrNotFound fail-closed.
- [x] 2.4 RED→GREEN `interactions.go` — RED TestInteractionCASIdempotent failed with stub; GREEN Create/Get/Resolve CAS pending+key, same key idempotent, mismatched key → ErrUniqueViolation, FK/CHECK→sentinels, RFC3339 UTC.
- [x] 2.5 GREEN `claim.go` — Added PRAGMA foreign_keys=1 on Conn before BEGIN IMMEDIATE, normalized all errors via normalizeSQLiteError.

### Phase 3: Fake & Contract — ✅ Complete
- [x] 3.1 GREEN `fake.go` — No database/sql import; mutex maps for 11 tables, CHECK/UNIQUE/FK validation → sentinels, RFC3339 UTC, monotonic cursors (attempt/transition), copy-on-write WithTx (clone/commit), Leases retained token, Interactions CAS. Passes golangci-lint.
- [x] 3.2 GREEN `contract/suite.go` — Run(t,Factory) covering state/rollback, leases monotonic, claims overlap, events/cursors monotonic + UNIQUE, interactions CAS + constraints. Shared, backend-neutral.
- [x] 3.3 GREEN `interchangeable_test.go` — package store_test, runs contract.Run with FakeStore and SQLite file-backed Open(t.TempDir()+"/store.db") (no :memory: for WAL). Both satisfy identical outcomes.

### Phase 4: Boundary & Integration — ✅ Complete
- [x] 4.1 RED→GREEN `report.go` — Dropped database/sql import, now imports store, uses errors.Is(err,store.ErrNotFound) for not_found mapping. Verified via git grep gate.
- [x] 4.2 GREEN `scripts/verify-store-boundary.sh` — git grep -n -E "database/sql|modernc.org/sqlite" -- '*.go' ':!internal/store/**' fails on hit; chmod +x; passes post-fix.
- [x] 4.3 GREEN parity tests — TestStoredTimestampsUTC (RFC3339 Z sampling across all generated times both backends), TestConstraintsParity (enums/FK/UNIQUE both backends → sentinels), TestMigrationsVerifySoleOwners (11 tables, path_claims/attempt_transport dag_hash/base_commit verified not redefined).
- [x] 4.4 Guardrail — No broker/PTY/renew-loop/distribution code added; no new dependencies; go.mod unchanged.

### Phase 5: Cleanup — ✅ Complete
- [x] 5.1 go build ./... — PASS
- [x] 5.2 go vet ./... — PASS
- [x] 5.3 go test ./... -race -count=1 — PASS (14 packages, store 5.9s, execution 29s)
- [x] 5.4 golangci-lint run — PASS (0 issues after fake fix)

## TDD Cycle Evidence (Strict TDD)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | migrations_store_test.go | Unit | ✅ 14 pkgs pass | ✅ Written (undefined seam) | ✅ Passed (0.008s) | ✅ 2 cases (exact schema + idempotent rows) | ✅ Clean |
| 1.2 | migrate_rollback_test.go | Unit | ✅ | ✅ Written (undefined migrateWithStatements) | ✅ Passed (0.045s rollback+retry) | ✅ bad mid-list + retry | ✅ Clean |
| 1.3 | migrations.go | Unit | ✅ | N/A (GREEN impl) | ✅ Passed | ✅ leases+interactions verbatim | ✅ txn wrapper |
| 2.1 | store.go | Unit | ✅ | N/A (GREEN sentinels/DSN) | ✅ Passed (existing tests) | ✅ ErrNotFound mapping, DSN pragma | ✅ Clean |
| 2.2 | repositories.go | Unit | ✅ | N/A | ✅ Passed | ✅ wrap all repos | ✅ Clean |
| 2.3 | leases_test.go / leases.go | Unit | ✅ | ✅ Written (stub returned ErrNotFound) → FAIL Acquire 1: not found | ✅ Passed (tokens 1,2,3 + stale ErrNotFound) | ✅ 2 cases (monotonic + fail-closed renew/release) | ✅ Conn+IMMEDIATE |
| 2.4 | interactions_test.go / interactions.go | Unit | ✅ | ✅ Written (stub not found) → FAIL Create: not found | ✅ Passed (CAS idempotent + ErrUniqueViolation) | ✅ 4 cases (create/get/resolve/duplicate/wrong-key) | ✅ CAS |
| 2.5 | claim.go | Unit | ✅ | N/A | ✅ Passed (concurrency 8 workers) | ✅ PRAGMA + normalize | ✅ Clean |
| 3.1 | fake.go | Unit | ✅ | N/A | ✅ Passed | ✅ mutex maps, cursor, UTC, WithTx copy-on-write | ✅ 0 lint |
| 3.2 | contract/suite.go | Integration | ✅ | N/A | ✅ Passed (fake+sqlite) | ✅ 5 suites | ✅ Clean |
| 3.3 | interchangeable_test.go | Integration | ✅ | N/A | ✅ Passed (0.049s) | ✅ dual backend | ✅ file-backed not :memory: |
| 4.1 | report.go | Unit | ✅ execution 2.1s | ✅ gate grep found import | ✅ Passed (no database/sql outside store) | ✅ errors.Is sentinel | ✅ Clean |
| 4.2 | verify-store-boundary.sh | Integration | ✅ | N/A | ✅ Passed (gate 0) | ✅ detects docs filtered | ✅ +x |
| 4.3 | parity_test.go | Unit | ✅ | N/A | ✅ Passed (UTC Z + constraints parity 0.017s) | ✅ both backends | ✅ Clean |
| 5.x | gates | N/A | N/A | N/A | ✅ build/vet/test -race/lint all PASS | N/A | N/A |

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command | `go test ./internal/store -run TestStoreInterchangeability -count=1` → ok 0.049s; `go test ./internal/store -run TestLeaseMonotonicFencing -count=1` → ok 0.012s; `go test ./internal/store -run TestInteractionCASIdempotent -count=1` → ok 0.012s |
| Runtime harness | `bash scripts/verify-store-boundary.sh` → store boundary check passed — no SQL imports outside internal/store (exit 0) |
| Rollback boundary | Revert `internal/store/*`, `internal/store/contract/*`, `internal/execution/report.go`, `scripts/verify-store-boundary.sh` — additive schema retained safely under IF NOT EXISTS; no data migration |

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/store/migrations.go` | Modified | migrationStatements verbatim leases+interactions, migrateWithStatements txn, migrate wrapper |
| `internal/store/store.go` | Modified | Sentinels, normalizeSQLiteError, DSN _pragma=foreign_keys(1), Leases()/Interactions() facade |
| `internal/store/repositories.go` | Modified | Wrapped all exec/Scan via normalizeSQLiteError |
| `internal/store/leases.go` | Created | Fencing Acquire Conn+BEGIN IMMEDIATE MAX+1 retained, Renew/Release RowsAffected fail-closed |
| `internal/store/interactions.go` | Created | Create/Get/Resolve CAS idempotent, ErrUniqueViolation on mismatch |
| `internal/store/claim.go` | Modified | PRAGMA foreign_keys=1 on Conn before BEGIN IMMEDIATE, normalized errors |
| `internal/store/transport.go` | Modified | Normalized errors |
| `internal/store/fake.go` | Created | Mutex 11-table fake, CHECK/UNIQUE/FK→sentinels, UTC RFC3339, monotonic cursors, WithTx copy-on-write, no driver import |
| `internal/store/contract/suite.go` | Created | Run(t,Factory) shared suite (state/rollback, leases, claims, events/cursors, interactions, constraints) |
| `internal/store/interchangeable_test.go` | Created | Dual-backend harness fake vs SQLite file-backed |
| `internal/store/migrations_store_test.go` | Created | RED 11-table exact schema + idempotent rows |
| `internal/store/migrate_rollback_test.go` | Created | RED txn rollback injection + retry |
| `internal/store/leases_test.go` | Created | RED→GREEN monotonic fencing |
| `internal/store/interactions_test.go` | Created | RED→GREEN CAS idempotent |
| `internal/store/parity_test.go` | Created | UTC Z, constraints parity, sole-owners 11-table |
| `internal/execution/report.go` | Modified | Removed database/sql, uses store.ErrNotFound |
| `scripts/verify-store-boundary.sh` | Created | +x gate git grep both SQL imports outside internal/store |
| `openspec/changes/v2-store/tasks.md` | Modified | All 19 tasks marked [x] |
| `go.mod` / `go.sum` | Unchanged | No new deps |

## Deviations from Design
None — implementation matches design.md verbatim DDL, sentinels, seam, DSN pragma, fencing token retained MAX+1, CAS idempotent, Fake parity, boundary gate.

## Issues Found
- normalizeSQLiteError initially mapped 275 (CHECK) to FK due to code grouping; fixed to prioritize CHECK string before code check; resolved interchangeable sqlite Constraints failure.
- fake.go staticcheck SA4006 for unused step existence check in CreateTransition; removed dead FK check.
- verify-store-boundary.sh initially flagged docs/go.mod; filtered to '*.go' only.

## Gate Outputs (summarized)
- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go test ./... -race -count=1` → ok 14 pkgs (store 5.947s, execution 29.029s, adapter/opencode 3.279s)
- `golangci-lint run` → 0 issues
- `bash scripts/verify-store-boundary.sh` → store boundary check passed — no SQL imports outside internal/store

## Next Recommended
sdd-verify

## Risks
- Fake fidelity drift mitigated by shared suite; future changes must extend both fake and contract together.
- Per-connection FK: DSN pragma plus dedicated-Conn PRAGMA ensures enforcement; tested via claim Acquire dedicated Conn.

## Skill Resolution
- sdd-apply Strict TDD followed
- work-unit-commits: single coherent slice size:exception
- go-testing: table-driven, t.TempDir file DB not :memory:, monotonic cursors, race -race

**Status**: 19/19 tasks complete. Ready for verify.
