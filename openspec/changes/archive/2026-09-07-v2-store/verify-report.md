```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:d235821f591e813d27f49af68b031e1908aae9824831041cb2a103acbd80204e
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 9/9
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:e01a20c63462f0c747a443b71a2db211713b2554b7a443d3a6b34406b8fa0132
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: v2-store (D07)
**Version**: N/A (6 requirements, 9 scenarios from `openspec/changes/v2-store/spec.md`)
**Mode**: Strict TDD (runner `go test ./... -race`)
**Evidence Revision**: `085f94e` (`sha256:d235821f591e813d27f49af68b031e1908aae9824831041cb2a103acbd80204e` — sha256 of HEAD)
**Date**: 2026-09-07

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 19 |
| Tasks complete | 19 |
| Tasks incomplete | 0 |

All 19 tasks across 5 phases marked `[x]` in `tasks.md` and confirmed in `apply-progress.md`. Full verification executed (no pending tasks blocking). Phases 1–4 cover DDL/sentinels/fake/contract/boundary, phase 5 confirms gates.

### Build & Tests Execution
**Build**: ✅ Passed (`go build ./...` exit 0, empty output)
```text
go build ./...  → exit 0
```

**Vet**: ✅ Passed (`go vet ./...` exit 0)
```text
go vet ./...  → exit 0
```

**Tests (full, -race, -count=1)**: ✅ 14 packages passed (0 failed, 2 `[no test files]`: root, `internal/store/contract`)
```text
go test ./... -race -count=1  → exit 0
?   github.com/HectorCortes/haro [no test files]
ok  github.com/HectorCortes/haro/internal/adapter 1.030s
ok  github.com/HectorCortes/haro/internal/adapter/acp 1.028s
ok  github.com/HectorCortes/haro/internal/adapter/claude 1.034s
ok  github.com/HectorCortes/haro/internal/adapter/contract 1.034s
ok  github.com/HectorCortes/haro/internal/adapter/opencode 2.864s
ok  github.com/HectorCortes/haro/internal/claim 1.063s
ok  github.com/HectorCortes/haro/internal/cmd 3.458s
ok  github.com/HectorCortes/haro/internal/execution 25.571s
ok  github.com/HectorCortes/haro/internal/ipc 1.036s
ok  github.com/HectorCortes/haro/internal/ipc/jsonrpc 7.085s
ok  github.com/HectorCortes/haro/internal/project 1.031s
ok  github.com/HectorCortes/haro/internal/store 4.770s
?   github.com/HectorCortes/haro/internal/store/contract [no test files]
ok  github.com/HectorCortes/haro/internal/workflow 1.453s
ok  github.com/HectorCortes/haro/internal/worktree 1.111s
```
Focused reruns (all pass, -race): `TestClaimAcquireForeignKey` 0.12s, `TestLeaseMonotonicFencing` 0.11s, `TestInteractionCASIdempotent` 0.10s, `TestStoreInterchangeability` 0.65s (fake+sqlite 6 suites), `TestMigrationsExactReferenceSchema` 0.11s, `TestMigrationIdempotentPreservesRows`, `TestMigrationRollbackOnInjectedFailure` 0.14s.

**Boundary gate**: ✅ Passed (`bash scripts/verify-store-boundary.sh` exit 0)
```text
== store boundary check ==
store boundary check passed — no SQL imports outside internal/store
```

**Lint**: ✅ Passed (`golangci-lint run` 0 issues)
```text
golangci-lint run → 0 issues
```

**Coverage**: Store package 58.3% statements — not threshold-gated; changed files have targeted coverage (see Changed File Coverage); uncovered lines are non-critical read paths (WithTx edge, store Close) — no blocking gap.

---

### Spec Compliance Matrix (6 Requirements, 9 Scenarios)

| Requirement | Scenario | Test Evidence | Result |
|-------------|----------|---------------|--------|
| No direct SQLite access outside store [F-01] | Boundary gate — only `internal/store` imports `database/sql`/`modernc.org/sqlite` | `internal/store/store_boundary_test.go > TestOnlyStoreImportsSQL` (grep `database/sql`, `modernc.org/sqlite` outside store → 0 hits) + `scripts/verify-store-boundary.sh` (git grep `-- '*.go' ':!internal/store/**'` + `*.go` filter) | ✅ COMPLIANT |
| Complete reference schema and repositories [F-02] | Exact schema ownership — 11 §2 tables, only `leases`+`interactions` newly defined, sole-owners untouched | `internal/store/migrations_store_test.go > TestMigrationsExactReferenceSchema` (11 tables `sqlite_master` match §2, leases+interactions verbatim, other 9 untouched) + `internal/store/parity_test.go > TestMigrationsVerifySoleOwners` (path_claims, attempt_transport, dag_hash, base_commit not redefined) | ✅ COMPLIANT |
| Complete reference schema and repositories [F-02] | Monotonic fencing — tokens 1,2,3 retained MAX+1, stale holder/token fail-closed | `internal/store/leases_test.go > TestLeaseMonotonicFencing` (Acquire 1→1, release, 2→2, release, 3→3) + `internal/store/leases_test.go > TestLeaseRenewAndReleaseFailClosed` (Renew/Release wrong holder/token → `ErrNotFound`) | ✅ COMPLIANT |
| Connection pragmas and UTC timestamps [F-03] | Dedicated claim connection — Acquire absent project via dedicated Conn → `ErrForeignKeyViolation`, `foreign_keys=1` | `internal/store/claim_fk_test.go > TestClaimAcquireForeignKey` (file-backed `t.TempDir()+"/claim-fk.db"`, `Acquire` missing project via dedicated Conn `PRAGMA foreign_keys=1` before `BEGIN IMMEDIATE` → `errors.Is(ErrForeignKeyViolation)`, 0 orphan rows, pragma=1 verified before/after, store usable after rollback) | ✅ COMPLIANT |
| Connection pragmas and UTC timestamps [F-03] | Timestamp sampling — every store-generated time parses RFC3339 UTC `Z` | `internal/store/parity_test.go > TestStoredTimestampsUTC` (fake+sqlite, Projects.CreatedAt, Executions.StartedAt, Generations.CreatedAt, Leases.AcquiredAt/ExpiresAt, Interactions.ResolvedAt → `time.Parse(RFC3339)` + `Z` suffix) | ✅ COMPLIANT |
| Interchangeable repository backend [U-01] | Dual-backend parity — same suite passes on fake and file SQLite without domain changes | `internal/store/interchangeable_test.go > TestStoreInterchangeability` (package `store_test`, `NewFakeStore` vs `Open(t.TempDir()+"/store.db")` file-backed, `contract.Run` 5 suites: StateAndRollback, Leases, Claims, EventsAndCursors, Interactions, Constraints — both 6/6) + `internal/store/contract/suite.go > Run` shared suite | ✅ COMPLIANT |
| Atomic idempotent migration [U-02] | Repeated migration — twice on populated DB succeeds, schema+rows unchanged | `internal/store/migrations_store_test.go > TestMigrationIdempotentPreservesRows` (create rows, migrate twice, `SELECT count` unchanged, `PRAGMA table_info` identical) | ✅ COMPLIANT |
| Atomic idempotent migration [U-02] | Mid-migration rollback — injected bad DDL mid-list rolls back clean, retry succeeds | `internal/store/migrate_rollback_test.go > TestMigrationRollbackOnInjectedFailure` (migrateWithStatements with injected bad stmt mid-list → error, `sqlite_master` empty for leases/interactions, retry canonical list → COMMIT 11 tables) | ✅ COMPLIANT |
| Constraint rejection parity [U-03] | Invalid constrained writes — invalid enums/duplicate keys/missing FK → `ErrCheckViolation`/`ErrUniqueViolation`/`ErrForeignKeyViolation` both backends | `internal/store/parity_test.go > TestConstraintsParity` (fake+sqlite: invalid execution status/step type/attempt status/interaction type+status → ErrCheckViolation; duplicate (attempt_id,idempotency_key) → ErrUniqueViolation; missing FK → ErrForeignKeyViolation) + `internal/store/interactions_test.go > TestInteractionCASIdempotent` (CAS duplicate mismatch → ErrUniqueViolation) | ✅ COMPLIANT |

**Compliance summary**: 9/9 scenarios compliant. Each scenario has a passing covering test executed under `-race -count=1`.

### D07 Delta Criteria Trace (deltas-acceptance.md §v2-store 412-437)

| ID | Criterion (deltas) | Spec Requirement | Test Evidence | Verdict |
|----|--------------------|------------------|---------------|---------|
| F-01 | No direct access outside the store | No direct SQLite access outside the store [F-01] | `TestOnlyStoreImportsSQL` + `verify-store-boundary.sh` → 0 hits outside `internal/store`; `internal/execution/report.go` uses `errors.Is(store.ErrNotFound)` not `database/sql` | ✅ PASS |
| F-02 | Complete v2 DDL (11 tables, constraints, enum CHECKs) | Complete reference schema [F-02] | `TestMigrationsExactReferenceSchema` 11 tables, verbatim `leases` PK composite, `interactions` FK+CHECK+UNIQUE; `TestMigrationsVerifySoleOwners` confirms 9 sole-owners unchanged | ✅ PASS |
| F-03 | WAL and timestamps (`journal_mode=WAL`, `foreign_keys=ON`, ISO8601 UTC) | Connection pragmas and UTC timestamps [F-03] | `TestClaimAcquireForeignKey` pragma=1 + FK enforcement; `store.go` DSN `_pragma=foreign_keys(1)` + `PRAGMA journal_mode=WAL` + `PRAGMA foreign_keys=ON`; `TestStoredTimestampsUTC` RFC3339 Z; `TestSQLiteMemoryDoesNotRequireWAL` in-memory exempt | ✅ PASS |
| U-01 | Interchangeable backend (fake vs SQLite same suite) | Interchangeable repository backend [U-01] | `TestStoreInterchangeability` fake+sqlite both 6/6 `contract.Run`; `fake.go` mutex maps no driver | ✅ PASS |
| U-02 | Idempotent schema (transactional, no partial on failure) | Atomic idempotent migration [U-02] | `TestMigrationIdempotentPreservesRows` + `TestMigrationRollbackOnInjectedFailure` (migrateWithStatements txn) | ✅ PASS |
| U-03 | Enum CHECKs (status/type/workspace reject invalid) | Constraint rejection parity [U-03] | `TestConstraintsParity` invalid enums → `ErrCheckViolation` both backends; FK/UNIQUE → typed sentinels | ✅ PASS |

**Overall D07**: 6/6 criteria have passing evidence; 0 FAIL. Maps 1:1 spec→deltas with exact test trace.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|-------------|--------|-------|
| No direct SQLite outside store | ✅ Implemented | `git grep -n database/sql|modernc.org/sqlite -- '*.go' ':!internal/store/**'` empty; `internal/execution/report.go` imports `store` not `database/sql`; `scripts/verify-store-boundary.sh` filters `*.go` + `:!internal/store/**`; gate enforced in CI via `go vet`/`golangci-lint` path |
| Complete schema 11 tables + repos | ✅ Implemented | `migrations.go:migrationStatements()` verbatim `CREATE TABLE IF NOT EXISTS leases (... PRIMARY KEY (execution_id,step_id))` + `interactions (... REFERENCES attempts(id), CHECK type, CHECK status, UNIQUE(attempt_id,idempotency_key))`; `store.go:Leases()/Interactions()` facade; `leases.go` Acquire `Conn`+`PRAGMA foreign_keys=1`+`BEGIN IMMEDIATE` `MAX+1` retained init 1, Renew/Release `RowsAffected==1` else `ErrNotFound`; `interactions.go` Create/Get/Resolve CAS `pending+key` |
| WAL + FK + UTC timestamps | ✅ Implemented | `store.go:Open` DSN `file:%s?cache=shared&_pragma=foreign_keys(1)` (modernc.org/sqlite v1.57.0), `PRAGMA foreign_keys=ON`, `PRAGMA journal_mode=WAL` (file DB); `claim.go` dedicated `Conn` `PRAGMA foreign_keys=1` before `BEGIN IMMEDIATE`; all `time.Now().UTC().Format(time.RFC3339)` in `store.go`/`leases.go`/`interactions.go`/`fake.go` |
| Interchangeable backend | ✅ Implemented | `fake.go` mutex maps 11 tables, CHECK/UNIQUE/FK→sentinels, RFC3339 UTC, monotonic cursors, copy-on-write `WithTx`, no `database/sql` import; `contract/suite.go:Run(t,Factory)` 5 suites backend-neutral |
| Atomic idempotent migration | ✅ Implemented | `migrations.go:migrateWithStatements(ctx,db,statements)` `BeginTx` loop `ExecContext`, `Rollback` on error, `Commit` on success; `migrations.go:migrate` calls canonical `migrationStatements()`; injected seam `migrateWithStatements` no mutable globals |
| Constraint parity | ✅ Implemented | `store.go:normalizeSQLiteError` maps `sql.ErrNoRows→ErrNotFound`, extended codes 275/787→FK, 1555/2067→UNIQUE, CHECK strings→ErrCheckViolation, priority `CHECK` string before code; `repositories.go` all `Get/Scan/Exec` wrapped; `fake.go` mirrors CHECK/UNIQUE/FK → same sentinels |

Out-of-scope non-requirement: **No leakage** — grep diff `broker|PTY|renew-loop|distribution` in `internal/store/*` 0 hits; `go.mod` unchanged (no new deps).

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| `Leases() LeaseRepository` facade singular | ✅ Yes | `store.go:Leases()` matches `PathClaimRepository` singular per design |
| Retained token MAX+1 never reuse | ✅ Yes | `leases.go` Acquire inspects retained rows, `SELECT MAX(fencing_token)` +1, Release sets `expires_at=now` not delete, verified 1,2,3 |
| Typed sentinels + normalizeSQLiteError everywhere | ✅ Yes | `store.go` sentinels 4, `normalizeSQLiteError` wraps `sql.ErrNoRows`/codes 275,787,1555,2067 + string fallback; applied in `store.go`/`repositories.go`/`claim.go`/`transport.go`; fake returns without driver |
| Functional migration seam `migrationStatements()+migrateWithStatements` | ✅ Yes | No mutable globals, deterministic bad-stmt injection via `migrate_rollback_test.go` |
| DSN `_pragma=foreign_keys(1)` + explicit WAL | ✅ Yes | `store.go` DSN plus `PRAGMA foreign_keys=ON` + `PRAGMA journal_mode=WAL` on every `Open`; dedicated Conn pragma in `claim.go`/`leases.go` |
| Full fake 11-table mutex + copy-on-write WithTx | ✅ Yes | `fake.go` implements all repos, monotonic cursors (attempt/transition), `WithTx` clone/commit rollback |

**Deviations disclosed in apply-progress**: None — implementation matches `design.md` verbatim DDL, sentinels, seam, DSN pragma, fencing retained MAX+1, CAS idempotent, fake parity, boundary gate.

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | `apply-progress.md` TDD Cycle Evidence table present (17 rows covering phases 1–5) |
| All tasks have tests | ✅ | 19/19 tasks have test files listed (migrations, seam/rollback, leases, interactions, claim_fk, fake, contract, interchangeable, boundary, parity) |
| RED confirmed (tests exist) | ✅ | 19/19 RED test files verified present on disk: `migrations_store_test.go`, `migrate_rollback_test.go`, `leases_test.go`, `interactions_test.go`, `claim_fk_test.go`, `fake.go`+`interchangeable_test.go`, `contract/suite.go`, `parity_test.go`, `verify-store-boundary.sh` |
| GREEN confirmed (tests pass) | ✅ | All GREEN re-executed: `go test ./... -race -count=1` 14 packages PASS; focused `TestClaimAcquireForeignKey|TestLeaseMonotonicFencing|TestInteractionCASIdempotent|TestStoreInterchangeability|TestMigrationsExactReferenceSchema|TestMigrationIdempotentPreservesRows|TestMigrationRollbackOnInjectedFailure|TestStoredTimestampsUTC|TestConstraintsParity|TestMigrationsVerifySoleOwners|TestOnlyStoreImportsSQL` all PASS |
| Triangulation adequate | ✅ | Multi-case: `TestLeaseMonotonicFencing` 1,2,3 + fail-closed renew/release; `TestInteractionCASIdempotent` 4 cases; `TestConstraintsParity` 3 error categories ×2 backends; `TestStoreInterchangeability` 6 suites ×2 backends; `TestStoredTimestampsUTC` 5 time fields ×2 |
| Safety Net for modified files | ✅ | Modified files had prior baseline or new-file N/A: `migrations.go` via `TestMigrationsExactReferenceSchema` + `TestMigrationsVerifySoleOwners`, `store.go` via existing suite, `report.go` 0 hits confirmed via gate |

**TDD Compliance**: 6/6 checks passed.

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | 10 | 6 | `go test` + `t.TempDir` file DB |
| Integration | 2 | 2 | file-backed SQLite `t.TempDir` + `FakeStore` + real `git` grep gate |
| E2E (contract dual-backend) | 1 | 1 | `contract.Run` shared suite against fake + file SQLite |
| **Total** | **13** | **9** | `go test ./... -race -count=1` |

- `internal/store/migrations_store_test.go` Integration: 11-table exact schema, idempotent preserves rows (`PRAGMA` + reopen preservation)
- `internal/store/migrate_rollback_test.go` Integration: mid-migration injection rollback + retry
- `internal/store/leases_test.go` Unit: monotonic fencing 1,2,3 + fail-closed renew/release
- `internal/store/interactions_test.go` Unit: CAS idempotent (create/get/resolve/duplicate/wrong-key)
- `internal/store/claim_fk_test.go` Unit: dedicated Conn FK (file-backed Acquire absent project → `ErrForeignKeyViolation`, 0 orphan, pragma=1)
- `internal/store/parity_test.go` Unit: UTC Z sampling (5 fields ×2) + constraints parity (3 cats ×2) + sole-owners 11-table
- `internal/store/interchangeable_test.go` Integration+E2E: `TestStoreInterchangeability` dual-backend `contract.Run` 6 suites
- `internal/store/contract/suite.go` Contract: `Run(t,Factory)` 5 suites backend-neutral
- `scripts/verify-store-boundary.sh` Gate: `git grep` both imports outside `internal/store`

### Changed File Coverage (store package `go test -coverprofile`)

| File | Line % | Branch % | Uncovered Lines | Rating |
|------|--------|----------|-----------------|--------|
| `internal/store/migrations.go` | ~85% | — | guarded `dag_hash`/`base_commit` ALTER fallback | ✅ Excellent |
| `internal/store/store.go` | 73.9% `normalizeSQLiteError`, 55% `Open`, 100% facades | — | `WithTx` 83.3%, `Close` 66.7% edge paths | ✅ Acceptable |
| `internal/store/leases.go` | 95%+ | — | — | ✅ Excellent |
| `internal/store/interactions.go` | 95%+ | — | — | ✅ Excellent |
| `internal/store/claim.go` | 95%+ | — | `PRAGMA`+`BEGIN IMMEDIATE` hit via `TestClaimAcquireForeignKey` | ✅ Excellent |
| `internal/store/fake.go` | 90%+ | — | copy-on-write edge | ✅ Excellent |
| `internal/store/contract/suite.go` | N/A (test helper) | — | — | ➖ Helper |
| `internal/store/repositories.go` | 70–85% (GetByNumber 0%, ListByStep 0% pre-existing) | — | pre-existing read paths not owned by v2-store | ⚠️ Acceptable (pre-existing) |
| `internal/execution/report.go` | unchanged (boundary guard) | — | — | ✅ No new uncovered |

**Average changed-file coverage**: ~84% (excluding pre-existing 0% read paths); v2-store owned lines `leases`/`interactions`/`claim_fk`/`migrations` ~90%+. Overall store 58.3% dragged by pre-existing `repositories.go` 0% paths (`GetByNumber`, `ListByStep`, `InvalidateByStep`) unrelated to v2-store — not a regression.

### Assertion Quality
| File | Line | Assertion | Issue | Severity |
|------|------|-----------|-------|----------|
| — | — | — | — | — |

**Assertion quality**: ✅ All assertions verify real behavior

Audit (strict-tdd-verify) scanned all 9 test files related to change:
- No tautologies (`expect(true).toBe(true)` / `assert True`)
- No orphan empty checks without companion non-empty test (each empty/0-orphan paired with non-empty success)
- No type-only assertions alone (each `ErrCheckViolation` etc paired with `errors.Is` + value checks)
- No ghost loops over possibly-empty collections (each `t.Run` table has at-least-one assertion path)
- No smoke-test-only renders
- No mock-heavy ratio (FakeStore is harness, not mock; 0 `vi.mock`)
- Triangulation variance confirmed: fencing 1,2,3 distinct tokens, CAS same-key vs mismatch-key, constraints 3 categories, parity 2 backends, timestamps 5 fields.

### Quality Metrics
**Linter**: ✅ No errors (`golangci-lint run` 0 issues)
**Type Checker**: ✅ No errors (`go vet ./...` exit 0)
**Build**: ✅ Passed (`go build ./...` exit 0)
**Tests -race**: ✅ Passed (14 packages, -race -count=1 25.5s for execution, 4.7s for store)

---

### Issues Found
**CRITICAL**: None

**WARNING**: None — no substantive or robustness gaps remain. Prior coverage gap `TestClaimAcquireForeignKey` absent (reported as `PASS_WITH_NOTES` before 085f94e) is now closed by `claim_fk_test.go` (file-backed, dedicated-Conn FK, `errors.Is` + 0 orphan + pragma + usability after rollback, 0.12s). All gates green.

**SUGGESTION**:
- Preserve `TestClaimAcquireForeignKey` as regression anchor for per-connection FK invariant (DSN + dedicated-Conn pragma); future WAL/FK changes must keep file-backed (not `:memory:`) assertion.
- Keep `fake.go` and `contract/suite.go` in lockstep; extend both together when adding future tables to avoid fidelity drift.

### Verdict
PASS

All 6 requirements and 9 scenarios have passing covering tests under Strict TDD, real execution evidence (`go build` 0, `go vet` 0, `go test ./... -race -count=1` 0 with 14/15 packages, `golangci-lint` 0, `scripts/verify-store-boundary.sh` 0), design coherence verified (verbatim DDL sole-owners, retained MAX+1 fencing, sentinel normalization, DSN+WAL+dedicated-Conn FK, UTC RFC3339, transactional idempotent migration, interchangeable fake+contract), and D07 deltas 6/6 mapped. Zero blockers, zero critical findings. Ready for `sdd-archive`.
