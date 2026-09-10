# Proposal: v2 Supervised Guard

## Intent

Eliminate the silent runtime downgrade where an agent step declared with `mode: supervised` executes through the headless path. Until broker, IPC, and supervisor support exists, runtime execution must fail closed with a clear terminal error.

## Scope

### In Scope
- Reject `mode == "supervised"` in the execution engine before harness intersection, fallback, or adapter invocation.
- Fail the step with a distinct `supervised mode not supported` reason and create no attempt.
- Add engine-level tests for supervised rejection and terminal-mode regression.
- Extend existing criterion `v2-no-regresion/F-02` with the runtime fail-closed scenario for unimplemented modes.

### Out of Scope
- Changes to `internal/workflow/validate.go`; supervised remains a valid declared enum for later implementation.
- Broker, IPC, supervisor, or managed-session implementation (D01/D02 and F-09); PTY/terminal implementation.
- Normative `docs/v2/`, `docs/reference/SPECS.md`, or `deltas-acceptance.md` changes; new acceptance IDs or criterion-count changes.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `v2-no-regresion`: extend F-02 so runtime rejects declared but unimplemented execution modes without attempts, adapters, or fallback.

## Approach

Extend the existing runtime mode guard in `internal/execution/engine.go` with a dedicated supervised branch before harness intersection and `RunFallback`. Reuse the terminal failure shape while preserving a mode-specific reason. Drive the change through focused engine tests using an adapter/fallback spy to prove neither path runs.

## Compliance and Impact

This closes a fail-open gap under `docs/v2/haro-constitucion.md` while preserving the mode contract in `docs/v2/haro-especificacion-tecnica.md` and the v1 oracle in `docs/reference/SPECS.md`. The 92-criterion, 11-spec acceptance contract remains unchanged.

| Area | Impact |
|---|---|
| `internal/execution/engine.go` | Add supervised runtime rejection |
| `internal/execution/*_test.go` | Add supervised and terminal engine coverage |
| Change delta spec for `v2-no-regresion/F-02` | Add fail-closed runtime scenario |

## Risks

| Risk | Mitigation |
|---|---|
| Supervised workflows that silently ran headless now fail | Intentional fail-closed behavior with a clear reason |
| Guard placement permits side effects | Assert no attempt, adapter call, or fallback call |
| Terminal behavior regresses | Add an engine-level terminal regression test |

## Rollback Plan

Revert the engine guard, focused tests, and F-02 delta together. Do not retain a partial spec/runtime mismatch.

## Delivery and Success

- Delivery: `single-pr`, maintainer-pre-approved `size:exception` (200000-line review budget), direct push to `main`; no PR.
- Strict TDD runner: `go test ./...`.
- [ ] Supervised mode fails terminally with its distinct reason, no attempt, no adapter invocation, and no fallback.
- [ ] Terminal mode retains equivalent fail-closed behavior and the full test suite passes.
