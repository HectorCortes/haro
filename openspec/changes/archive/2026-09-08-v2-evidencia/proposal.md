# Proposal: Inline Attempt Evidence and Reopen Recovery

## Intent

Evidence files create noise, can be empty, and omit what ran. Feedback depends on them, while reopened workflows remain blocked after producer reruns because stale history is checked.

## Scope

### In Scope
- Add nullable `attempt_events.payload TEXT`; write sanitized evidence inline while retaining legacy `payload_ref` references.
- Compose command evidence as `$ <argv>` plus output. Compose agent evidence with `harness: <name> (<index>/<total>)`, mode, bounded instructions, then output. Redact and bound each complete composition to 16 KiB.
- Read feedback evidence from the DB, falling back to legacy referenced files.
- Stop creating evidence directories/files; retain snapshots.
- Check `requires` staleness against only the producer's latest generation.
- Amend the payload doctrine and add strict-TDD tests, including reopen → producer rerun → downstream success.

### Out of Scope
- Real adapter-manager wiring; synthetic agent output remains a documented follow-up.
- Snapshot pruning, legacy cleanup, `v2-broker`, and `v2-ipc` implementation.

## Capabilities

### New Capabilities
- `v2-evidencia`: Inline bounded evidence, execution headers, legacy reads, and recovery semantics.

### Modified Capabilities
- `v2-store`: Additive payload persistence with SQLite/fake parity.
- `v2-no-regresion`: Evidence and reopen requirements gain positive recovery coverage.

## Approach

Mirror `ensureDagHashColumn`: probe `PRAGMA table_info`, run `ADD COLUMN payload TEXT`, guard races with `isDuplicateColumnError`, and update fresh DDL. Persist composition → redaction → budget through store interfaces. Prefer DB payload with legacy-file fallback. Check latest-generation invalidation only. Preserve 16 KiB visible, 1 MiB snapshot, and 2 MiB fallback limits.

## Delta-Spec Implications

Specs MUST define `v2-evidencia`, modify `v2-store` and `v2-no-regresion`, and narrowly amend `v2-ipc/U-03` plus Technical Specification §2/§2.1: `payload` may hold sanitized bounded deltas, never raw/full output; `payload_ref` stays external/legacy. Add no acceptance ID; map existing criteria, including `v2-flujo-sdd/U-01` traceability.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/store/`, `internal/execution/` | Schema, evidence, feedback, staleness, tests |
| `docs/v2/haro-especificacion-tecnica.md` | Doctrine amendment |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Doctrine/history break | High | Additive column, explicit amendment, legacy fallback |
| Budget/redaction bypass | Medium | Compose first; boundary tests |
| Migration/backend drift | Medium | Guarded migration; parity/reopen tests |

## Rollback Plan

Restore legacy `payload_ref` file use and revert the doctrine amendment; retain the nullable column. No destructive migration is needed.

## Delivery

Maintainer-approved `size:exception`: single direct push to `main`, no PR. Reviewable units: U1 schema/migration/store; U2 headers/inline/feedback/no files; U3 reopen fix/tests. Later phases MUST follow RED-GREEN-REFACTOR using `go test ./...`.

## Success Criteria

- [ ] New runs create no evidence files and persist useful bounded payloads.
- [ ] Legacy feedback works; negative and positive reopen paths pass.
- [ ] `go test ./...` passes with criterion-to-test traceability.
