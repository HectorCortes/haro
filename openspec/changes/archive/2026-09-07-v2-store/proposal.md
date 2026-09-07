# Proposal: v2-store (repository-backed persistence)

## Intent

Deliver all D07 criteria (`v2-store/F-01`–`F-03`, `U-01`–`U-03`): complete the reference store while enforcing Constitution III.3, fail-closed constraints, backend interchangeability, and atomic migration.

## Scope

### In Scope
- Add only exact §2 `leases` and `interactions` DDL: leases deliberately have no FK/CHECK; interactions retain FK, CHECKs, and UNIQUE. Verify all 11 tables without redefining sole-owned schema.
- Add `Leases() LeaseRepository` and `Interactions() InteractionRepository`; the latter exposes minimal `Create`, `Get`, and idempotent `Resolve` operations.
- Implement atomic per-step lease tokens as retained previous maximum + 1 (initial 1), with holder/token-checked renew/release; timers and renewal loops remain deferred.
- Make all of `migrate` transactional and idempotent; an unexported statement-list seam permits deterministic test-only mid-migration failure injection.
- Add a mutex-backed `FakeStore` with transaction rollback, constraint/cursor parity, and typed `ErrNotFound`, `ErrCheckViolation`, `ErrUniqueViolation`, and `ErrForeignKeyViolation` sentinels normalized by both backends.
- Run one shared `internal/store/contract` suite for state machine, leases, claims, events, and interactions against fake and file-backed SQLite.
- Remove `database/sql` from `internal/execution/report.go`; gate both SQL imports outside `internal/store`. Apply `&_pragma=foreign_keys(1)` in the DSN and test the dedicated `claim.Acquire` connection; retain WAL setup.

### Out of Scope
- Broker/UDS/JSON-RPC; lease expiry timers/renew loop; PTY; reporting; distribution; `agents_command`; multi-user behavior; new dependencies.
- Redefining `path_claims`, `attempt_transport`, `dag_hash`, or `base_commit`; verification only.

## Capabilities

### New Capabilities
- `v2-store`: complete, interchangeable repository persistence.

### Modified Capabilities
- None.

## Approach

Use exploration Approach 1 as one coherent slice. Splitting DDL/fake delays 6/6 proof; mocks cannot prove SQLite; full schema takeover violates sole ownership.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/store/{migrations,store,leases,interactions,fake}.go` | DDL, facade, backends |
| `internal/store/contract/suite.go`, `internal/store/*_test.go` | shared/schema/atomicity tests |
| `internal/execution/report.go` | store sentinel |
| `scripts/verify-store-boundary.sh` | F-01 gate |

## Risks

| Risk | Mitigation |
|---|---|
| Sole-owner divergence | Create only two owned tables; inspect the rest. |
| Fake fidelity drift | Same contract suite and typed categories. |
| Per-connection FK disabled | DSN pragma plus dedicated-connection test. |

## Rollback Plan

Revert facade, repositories, fake, gate, and migration transaction. Retain additive schema safely under idempotent-series convention; do not remove persisted rows.

## Dependencies

- Existing pure-Go SQLite stack only; normative sources are `deltas-acceptance.md`, `docs/reference/SPECS.md`, and `docs/v2/*`.

## Traceability and Success Criteria

| Criterion | Proposed proof |
|---|---|
| F-01 | import gate; repository-only access |
| F-02 | exact 11-table schema inspection |
| F-03 | WAL/FK and RFC3339 UTC sampling |
| U-01 | identical dual-backend contract suite |
| U-02 | rerun plus injected atomic rollback |
| U-03 | invalid-enum inserts yield typed constraint errors |

- [ ] All 6/6 proofs and `go test ./...` pass reproducibly without sole-owner changes.
