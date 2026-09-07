# Archive Report: v2-store (persistence behind a repository interface — D07)

**Change**: v2-store
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-v2-store/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-store/spec.md` (new domain — full spec: Purpose, Constraints, Requirements, Non-Requirements)

## Goal

Deliver D07 criteria (`v2-store/F-01`–`F-03`, `U-01`–`U-03`): complete the reference store to the full 11-table §2 schema, expose lease and interaction repositories behind the `Store` facade, prove backend interchangeability between file-backed SQLite and a mutex-backed fake, make migration transactional and idempotent, and gate SQL imports strictly to `internal/store`.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:d235821f591e813d27f49af68b031e1908aae9824831041cb2a103acbd80204e
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 9/9
```

- **Tasks**: 19/19 complete (`tasks.md` all `[x]`; file read and verified in this phase; Engram obs 2443). Task Completion Gate: PASS, no stale unchecked implementation boxes.
- **Verification**: PASS, 6/6 requirements, 9/9 scenarios. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:d235821f591e813d27f49af68b031e1908aae9824831041cb2a103acbd80204e, verdict `pass`, blockers 0, critical_findings 0 (Engram obs 2449; file `verify-report.md`, evidence revision = commit `085f94e`). D07 criteria trace: 6/6 (F-01..F-03, U-01..U-03) with passing covering tests.
- **Gates green at close**: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -race -count=1` exit 0 (14 packages, 2 root `[no test files]`), `golangci-lint run` 0 issues, `scripts/verify-store-boundary.sh` exit 0. CRITICAL: 0, blockers: 0.
- **Post-verify remediation**: the MAJOR coverage gap noted as `PASS_WITH_NOTES` before `085f94e` — `TestClaimAcquireForeignKey` absent for F-03 "Dedicated claim connection" — was **closed** in commit `085f94e` (`claim_fk_test.go`: file-backed Acquire of an absent project via dedicated `Conn` with `PRAGMA foreign_keys=1` before `BEGIN IMMEDIATE` → `errors.Is(ErrForeignKeyViolation)`, 0 orphan rows, pragma=1 verified before/after, store usable after rollback). Per the archive report below, this ships the F-03 scenario with real passing evidence; no CRITICAL or blocker remains.
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (review budget 200000, config review_budget_lines 20000). Delivered as direct pushes to `main` (no PR) per user preference, branchless; implementation commits are already in `main`.
- **Commits** (6, all in `main`; this `docs(sdd)` commit archives the change):
  - `56b0eff` feat(store): transactional migrations with leases and interactions DDL
  - `6ac00fd` feat(store): sentinels, normalized errors, leases and interactions repos
  - `eb9148f` feat(store): fake backend and interchangeable contract suite
  - `fe068f4` fix(execution): enforce store boundary and parity guardrails
  - `2baf5b9` chore(sdd): mark v2-store tasks complete and record apply progress
  - `085f94e` test(store): cover dedicated-connection FK enforcement in claim.Acquire
- **Strict TDD**: ACTIVE and followed — RED→GREEN per task (evidence in apply-progress obs 2444; 19/19 tasks with test files; TDD Compliance 6/6 checks; only additive test coverage after the apply correction, no behavior change).
- **Warnings**: none at close. `verify-report` reports zero WARNINGs; two SUGGESTIONs recorded for the future: preserve `TestClaimAcquireForeignKey` as the per-connection FK regression anchor (keep it file-backed, not `:memory:`), and keep `fake.go` + `contract/suite.go` in lockstep when adding future tables to avoid fidelity drift.

## Criteria Covered (criterion → test evidence)

| ID | Criterion | Spec requirement | Test evidence |
|----|-----------|------------------|---------------|
| F-01 | No direct access outside the store [INT] P0 | No direct SQLite access outside the store | `internal/store/store_boundary_test.go > TestOnlyStoreImportsSQL` (grep `database/sql`/`modernc.org/sqlite` outside store → 0) + `scripts/verify-store-boundary.sh` (exit 0); `report.go` uses `errors.Is(err, store.ErrNotFound)` not `database/sql` |
| F-02 | Complete v2 DDL [INT] P0 | Complete reference schema and repositories | `TestMigrationsExactReferenceSchema` (11 tables match §2; leases+interactions verbatim) + `TestMigrationsVerifySoleOwners` (path_claims/attempt_transport/dag_hash/base_commit not redefined) + `TestLeaseMonotonicFencing` (tokens 1,2,3; stale holder/token fail closed) |
| F-03 | WAL and timestamps [INT] P1 | Connection pragmas and UTC timestamps | `TestClaimAcquireForeignKey` (file-backed dedicated-Conn FK → `ErrForeignKeyViolation`, pragma=1) + `TestStoredTimestampsUTC` (RFC3339 `Z` across all generated times, both backends); DSN `_pragma=foreign_keys(1)` + `PRAGMA journal_mode=WAL` |
| U-01 | Interchangeable backend [UNIT] P0 | Interchangeable repository backend | `TestStoreInterchangeability` (fake + file SQLite both run `contract.Run` 5 suites, identical outcomes/errors) |
| U-02 | Idempotent schema [UNIT] P1 | Atomic idempotent migration | `TestMigrationIdempotentPreservesRows` (twice on populated DB, schema+rows unchanged) + `TestMigrationRollbackOnInjectedFailure` (mid-list bad DDL rolls back, retry commits 11 tables) |
| U-03 | Enum CHECKs [UNIT] P1 | Constraint rejection parity | `TestConstraintsParity` (invalid enums → `ErrCheckViolation`, duplicate key → `ErrUniqueViolation`, missing FK → `ErrForeignKeyViolation`, both backends) + `TestInteractionCASIdempotent` |

**Compliance summary**: 9/9 scenarios compliant, each with a passing covering test under `-race -count=1`. D07 deltas 6/6 mapped 1:1 from spec to the deltas table.

## Files Changed (implementation, per apply-progress)

| File | Action | What Was Done |
|---|---|---|
| `internal/store/migrations.go` | Modified | `migrationStatements()` verbatim `leases`+`interactions` DDL; `migrateWithStatements` transactional (BeginTx/COMMIT/ROLLBACK); `migrate` wrapper |
| `internal/store/store.go` | Modified | Sentinels `ErrNotFound`/`ErrCheckViolation`/`ErrUniqueViolation`/`ErrForeignKeyViolation`, `normalizeSQLiteError`, DSN `_pragma=foreign_keys(1)`, `Leases()`/`Interactions()` facade |
| `internal/store/repositories.go` | Modified | All `Get`/Scan/exec wrapped via `normalizeSQLiteError` |
| `internal/store/leases.go` | Created | Fencing `Acquire` via dedicated `Conn` + `BEGIN IMMEDIATE` `MAX+1` retained (init 1), `Renew`/`Release` `RowsAffected==1` else `ErrNotFound` |
| `internal/store/interactions.go` | Created | Create/Get/Resolve CAS pending+key, mismatched key → `ErrUniqueViolation` |
| `internal/store/claim.go` | Modified | `PRAGMA foreign_keys=1` on `Conn` before `BEGIN IMMEDIATE`; normalized errors |
| `internal/store/transport.go` | Modified | Normalized errors |
| `internal/store/fake.go` | Created | Mutex 11-table fake, CHECK/UNIQUE/FK → sentinels, RFC3339 UTC, monotonic cursors, copy-on-write `WithTx`, no driver import |
| `internal/store/contract/suite.go` | Created | `Run(t, Factory)` shared backend-neutral suite (state/rollback, leases, claims, events/cursors, interactions/constraints) |
| `internal/store/interchangeable_test.go` | Created | Dual-backend harness fake vs file SQLite |
| 6 test files + `scripts/verify-store-boundary.sh` | Created | Schema ownership, txn rollback, monotonic fencing, CAS idempotent, UTC/constraints parity, dedicated-Conn FK, boundary gate (`+x`) |
| `internal/execution/report.go` | Modified | Removed `database/sql` import; uses `errors.Is(err, store.ErrNotFound)` |
| `go.mod` / `go.sum` | Unchanged | No new dependencies |

## Key Decisions

- **Typed constraint sentinels + `normalizeSQLiteError`**: four sentinels (`ErrNotFound`, `ErrCheckViolation`, `ErrUniqueViolation`, `ErrForeignKeyViolation`); normalize maps `sql.ErrNoRows→ErrNotFound`, extended codes 275/787→FK, 1555/2067→UNIQUE, CHECK-string priority before code; applied across `store.go`/`repositories.go`/`claim.go`/`transport.go`; fake returns the same sentinels without a driver.
- **Fencing `MAX+1` retained, never reused**: `Acquire` serializes via dedicated `Conn` + `BEGIN IMMEDIATE`, inspects retained rows, issues `MAX(fencing_token)+1` (initially 1); `Release` expires (sets `expires_at=now`, never deletes); `Renew`/`Release` check holder+token and fail closed with `ErrNotFound` when `RowsAffected` ≠ 1. Verified tokens 1,2,3.
- **DSN pragma + explicit WAL**: `file:%s?cache=shared&_pragma=foreign_keys(1)` (modernc.org/sqlite v1.57.0) applied on every opened connection, plus `PRAGMA foreign_keys=ON` and `PRAGMA journal_mode=WAL` (file-backed); dedicated `Conn` re-asserts `PRAGMA foreign_keys=1` before `BEGIN IMMEDIATE` in `claim.go`/`leases.go`. In-memory DBs are exempt from WAL (return `memory`).
- **Transactional migration seam**: `migrationStatements()` + `migrateWithStatements(ctx, db, statements)` works on one `BeginTx` with commit/rollback and no mutable globals, enabling deterministic bad-statement injection for U-02.
- **FakeStore + shared contract suite**: mutex-backed 11-table fake implements the full `Store` + all repositories with CHECK/UNIQUE/FK validation, RFC3339 UTC, monotonic cursors, and copy-on-write `WithTx`; one `contract.Run` suite runs unchanged against both backends.
- **Sole-owner discipline**: this change creates only `leases` and `interactions` verbatim §2; `path_claims`/`attempt_transport`/`dag_hash`/`base_commit` are verified but never redefined.

## Limits Respected

- Sole-owners intact: `attempt_transport` (v2-adapter), `path_claims` (v2-path-claims), `dag_hash` (v2-composicion), `base_commit` (v2-reporte) verified not redefined.
- No broker runtime, UDS/JSON-RPC, PTY, lease expiry/renewal loops, reporting persistence, distribution, multi-user behavior, or new dependencies; `go.mod` unchanged.
- No edits to `docs/v2/` or `openspec/config.yaml`. **Post-archive recommendation (not applied here)**: `docs/v2/haro-especificacion-tecnica.md` §7 sole-owner table could be extended to note that `leases` and `interactions` are now owned by `v2-store` — a normative docs update, deferred out of this change per the "do not edit docs without an SDD change" rule.

## Deferrals and Exclusions (intentional, recorded)

- `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture; lease fencing enforcement at runtime and renewal loops belong to the broker, not this DDL/repository change.
- `v2-no-regresion`, `v2-adapter`, `v2-path-claims` remain `pending` as recorded — their tracking-row updates in `deltas-acceptance.md` were begun later in the series and their checkboxes were left untouched; this change updates only the `v2-store` row and its checkboxes.
- `v2-distribucion` and `v2-flujo-gentle-ai` remain later work.

## Sole Documented Amendment

- `deltas-acceptance.md` — D07 spec (`v2-store`, lines 414–436): all 6 criterion checkboxes (F-01..F-03, U-01..U-03) marked verified `[x]`; tracking row updated from `pending` to **complete** (6 criteria, 3 P0; verdict pass); "Last updated" summary line updated to 2026-09-07. No implementation-time doc amendments: no broker/UDS/JSON-RPC/PTY/distribution/`agents_command` code and no changes to existing normative docs (guardrail greps + unchanged `go.mod`).

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| explore | 2436 | `openspec/changes/archive/2026-09-07-v2-store/exploration.md` |
| proposal | 2437 | `openspec/changes/archive/2026-09-07-v2-store/proposal.md` |
| spec | 2439 | `openspec/changes/archive/2026-09-07-v2-store/spec.md` |
| design | 2441 | `openspec/changes/archive/2026-09-07-v2-store/design.md` |
| tasks | 2443 | `openspec/changes/archive/2026-09-07-v2-store/tasks.md` |
| apply-progress | 2444 | `openspec/changes/archive/2026-09-07-v2-store/apply-progress.md` |
| verify-report | 2449 | `openspec/changes/archive/2026-09-07-v2-store/verify-report.md` |
| archive-report | (this save) | `openspec/changes/archive/2026-09-07-v2-store/archive-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run for it. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `spec.md` ✅ (delta spec, verbatim)
- `design.md` ✅
- `tasks.md` ✅ (19/19 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-store/spec.md` — new domain spec created. The change's delta spec is ADDED-only (6 requirements, no MODIFIED/REMOVED/RENAMED), so the main spec was composed as a full spec (Purpose, Constraints, Requirements, Non-Requirements) per the archive convention, modeled on `openspec/specs/v2-reporte/spec.md`: Purpose and Constraints composed from the delta's normative anchors; the 6 ADDED requirements carried over verbatim with scenarios; Non-Requirements section carried over verbatim. Carried-over bodies verified byte-identical against the delta by `diff` (requirement blocks and Non-Requirements body identical; heading `## ADDED Requirements` renamed to `## Requirements`; delta section markers dropped as delta bookkeeping). The `rules.archive` "warn before merging destructive deltas" did not trigger: no `REMOVED`/`RENAMED` sections exist and the sync is purely additive.
- `deltas-acceptance.md` — D07 criteria checkboxes, tracking row, and summary line updated (see Sole Documented Amendment).

## Mechanical Verification

- Folder move: `git add` of the initially-untracked artifacts (design, exploration, proposal, spec, verify-report) then `git mv openspec/changes/v2-store openspec/changes/archive/2026-09-07-v2-store`. Pre-move recursive snapshot vs archived tree, `diff -r` → **empty (exit 0)**: byte-identical, no truncation or alteration. `archive-report.md` excluded because it did not exist in the source change folder (additive-only).
- Source directory confirmed gone after the move; active changes directory contains only `archive/`.
- Spec sync: header (Purpose/Constraints) composed by the model; requirement and Non-Requirement bodies verified byte-identical to the delta with `diff` (verbatim diff output in phase result).
- `deltas-acceptance.md`: exactly 6 checkboxes marked `[x]`, tracking row `complete`, "Last updated" updated; diff confirmed no other spec rows/checkboxes touched.

## Next

- Post-archive: the next logical candidate per the tracking table and the established deferrals is **`v2-distribucion`** (distribution), followed by **`v2-flujo-gentle-ai`** (development flow with gentle-ai). `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture; `v2-no-regresion`, `v2-adapter`, and `v2-path-claims` rows remain `pending` as recorded, with their archive cycles having left `deltas-acceptance.md` untouched (tracking updates began with `v2-composicion`).
- Delivery: the orchestrator performs the direct `main` push of the accumulated local commits after this archive phase (per the pre-approved `size:exception` and the user's no-PR delivery preference).
