# Design: v2 repository-backed persistence

## Technical Approach

Append two idempotent DDL statements, retain the `Store` facade/repository pattern, add a mutex-backed fake, and run one backend-neutral contract suite against fake and file SQLite. Writes use RFC3339 UTC; strict RED-GREEN-REFACTOR runs `go test ./...`.

## Architecture Decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| `LeaseRepository` | Facade remains plural | Use normative §5 `Leases() LeaseRepository`, matching singular `PathClaimRepository`. |
| Retained token | Released rows remain | Release expires, never deletes; serialized Acquire uses `MAX(fencing_token)+1`, initially 1, preventing token reuse. |
| Typed sentinels | Normalize everywhere | `ErrNotFound`, `ErrCheckViolation`, `ErrUniqueViolation`, `ErrForeignKeyViolation`; `normalizeSQLiteError` in `store.go` wraps `sql.ErrNoRows` and extended codes 275, 787, 1555/2067. Fake returns these without a driver. |
| Functional migration seam | Slight indirection | `migrationStatements()` plus `migrateWithStatements(ctx, db, statements)` avoids mutable globals and permits deterministic bad-statement injection. |
| DSN pragma | Driver-specific URI | Use `file:%s?cache=shared&_pragma=foreign_keys(1)`, supported by modernc.org/sqlite v1.57.0 and applied on every opened connection; retain explicit WAL setup. |
| Full fake | Larger initial slice | Implement every existing repository plus leases/interactions so `FakeStore` truly satisfies `Store`; copy-on-write transactions preserve rollback. |

## Interfaces and Schema

```go
type Lease struct { ExecutionID, StepID, Holder string; FencingToken int64; AcquiredAt, ExpiresAt string }
type LeaseRepository interface {
 Acquire(context.Context, string, string, string) (int64, error)
 Renew(context.Context, string, string, string, int64) error
 Release(context.Context, string, string, string, int64) error
}
type Interaction struct { ID, AttemptID, Type, Status string; Decision *string; IdempotencyKey string; ResolvedAt *string }
type InteractionRepository interface {
 Create(context.Context, *Interaction) error
 Get(context.Context, string) (*Interaction, error)
 Resolve(context.Context, string, string, string) (*Interaction, error) // id,key,decision
}
```

`migrations.go` appends §2 verbatim except idempotency:

```sql
CREATE TABLE IF NOT EXISTS leases (execution_id TEXT NOT NULL, step_id TEXT NOT NULL, holder TEXT NOT NULL, fencing_token INTEGER NOT NULL, acquired_at TEXT NOT NULL, expires_at TEXT NOT NULL, PRIMARY KEY (execution_id, step_id));
CREATE TABLE IF NOT EXISTS interactions (id TEXT PRIMARY KEY, attempt_id TEXT NOT NULL REFERENCES attempts(id), type TEXT NOT NULL CHECK (type IN ('permission','question')), status TEXT NOT NULL CHECK (status IN ('pending','resolved')), decision TEXT, idempotency_key TEXT NOT NULL, resolved_at TEXT, UNIQUE (attempt_id, idempotency_key));
```

The loop and guarded `dag_hash`/`base_commit` ALTERs execute on one `BeginTx`; errors roll back. This change owns only these tables; tests inspect all nine existing tables and prove `attempt_transport`, `path_claims`, `dag_hash`, and `base_commit` unchanged.

## Data Flow

```mermaid
sequenceDiagram
 participant C as Caller; participant R as Repository; participant D as SQLite
 C->>R: Acquire(exec,step,holder); R->>D: Conn + BEGIN IMMEDIATE
 R->>D: inspect active row; MAX(token)+1; INSERT/UPDATE; COMMIT
 C->>R: Resolve(id,key,decision); R->>D: CAS pending+key
 R->>D: read row; Note over R,D: same key returns stored result; mismatched key => ErrUniqueViolation
 C->>D: migrateWithStatements; D->>D: BEGIN; D->>D: DDL...bad DDL
 D-->>C: ROLLBACK (no partial schema); C->>D: retry canonical list; D->>D: COMMIT
```

Acquire rejects an unexpired holder conflict and sets expiry to now+one minute; Release sets expiry to now. Renew recomputes expiry only when holder+token match. Renew/release use `RowsAffected==1`; otherwise `ErrNotFound` (fail closed).

## File Changes

| File | Action | Design |
|---|---|---|
| `internal/store/{migrations,store}.go` | Modify | Transaction, seam, DDL, types, facade, sentinels, DSN/normalizer. |
| `internal/store/{leases,interactions,fake}.go` | Create | Dedicated-Conn fencing; interaction CAS; mutex maps for Projects/Executions/Steps/Attempts/Generations/Events/Transport/PathClaims/Leases/Interactions, constraints, monotonic cursors, UTC, copy-on-write `WithTx`, no-op `Close`. |
| `internal/store/claim.go` | Modify | Assert `PRAGMA foreign_keys=1` on its dedicated connection before `BEGIN IMMEDIATE`; normalize errors. |
| `internal/store/contract/suite.go` | Create | `Run(t, Factory)` covers state/rollback, leases, claims, events/cursors, interactions/constraints. |
| `internal/store/interchangeable_test.go` | Create | `package store_test`; invoke unchanged suite with `NewFakeStore` and `Open(t.TempDir()/store.db)`. |
| `internal/store/*_test.go` | Modify/Create | Schema ownership, migration, repository, timestamp and error tests. |
| `internal/execution/report.go` | Modify | Replace `sql.ErrNoRows`/string matching with `errors.Is(err, store.ErrNotFound)`. |
| `scripts/verify-store-boundary.sh` | Create | `git grep` both imports in tracked `*.go` outside `internal/store`; print hits and exit nonzero. |

## Testing and Spec Validation

| Requirement/scenario | RED test |
|---|---|
| F-01 boundary | `TestStoreBoundaryScript` plus script execution |
| F-02 ownership | `TestMigrationsExactReferenceSchema`; `TestLeaseMonotonicFencing` |
| F-03 connection/time | `TestClaimAcquireForeignKey`; `TestGeneratedTimestampsUTC` |
| U-01 parity | `TestStoreInterchangeability/{fake,sqlite}` |
| U-02 repeat/rollback | `TestMigrationIdempotentPreservesRows`; `TestMigrationRollbackOnInjectedFailure` |
| U-03 constraints | shared `constraints` cases for every enum, duplicate key, and missing FK |

## Threat Matrix

| Boundary | Applicability |
|---|---|
| Documentation-like paths | N/A: no executable classification |
| Git repository selection | N/A: gate only searches the current checkout |
| Commit state | N/A: no commit/index mutation |
| Push state | N/A: no remote operation |
| PR commands | N/A: no PR automation |

## Migration, Risks, and Open Questions

No data migration or flag is required; additive schema remains on rollback. The shared suite limits fake drift, snapshots limit sole-owner drift, and DSN plus dedicated-Conn assertion limits FK drift. Lease concurrency uses file WAL and `-race`. No blocking questions.
