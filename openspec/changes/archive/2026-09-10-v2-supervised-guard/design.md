# Design: v2 Supervised Guard

## Context

`runAgentStep` re-parses the workflow, reads the declared mode, rejects `terminal`, and otherwise continues into harness configuration, intersection, attempt creation, and adapter sessions. Because `supervised` is valid at declaration time but has no broker, IPC, or supervisor implementation, it currently falls through the headless path. This design implements the fail-closed runtime behavior required by `v2-no-regresion/F-02` without changing validation.

## Technical Approach

Extend the existing mode guard in `internal/execution/engine.go` immediately after workflow mode extraction and the non-empty harness check, before configuration loading and harness intersection. Use one branch for both unsupported runtime modes:

```go
if mode == "terminal" || mode == "supervised" {
	reason := fmt.Sprintf("%s mode not supported", mode)
	_ = e.failAttempt(ctx, "no-attempt", executionID, stepID, reason)
	return fmt.Errorf("%s", reason)
}
```

The existing `failAttempt` path transitions the already-running step to `failed` and resynchronizes the execution. The sentinel attempt ID preserves the current no-attempt behavior; its ignored attempt update has no persisted row.

## Architecture Decisions

| Decision | Alternatives | Rationale |
|---|---|---|
| Keep one parameterized unsupported-mode branch | Add a separate `supervised` branch | This is the smallest diff, preserves terminal control flow exactly, and still emits distinct mode-specific reasons without duplicating failure logic. |
| Enforce only in `runAgentStep` | Reject in workflow validation or adapters | `supervised` remains a valid future-facing declaration; the runtime engine is the authority that knows support is absent and can stop before side effects. |
| Reuse `failAttempt(..., "no-attempt", ...)` | Create an attempt or add store schema | Unsupported execution never starts, so creating an attempt or persistence contract would misrepresent execution and expand scope. |

## Data Flow

```text
RunStep → runAgentStep → dependency/require checks → running → re-parse mode
                                                        │
                         terminal/supervised ────────────┤
                                                        ↓
                              failAttempt("no-attempt", reason)
                                  → step failed → execution resync failed

                         headless → existing config/intersection/fallback path
```

## Interfaces / Contracts

No public interface or schema changes. Runtime errors remain `<mode> mode not supported`: specifically `supervised mode not supported` and the unchanged `terminal mode not supported`. Unsupported modes create no generation, attempt, transport, fallback evidence, or adapter session.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/execution/engine.go` | Modify | Extend and parameterize the existing terminal guard. |
| `internal/execution/engine_test.go` | Modify | Add supervised fail-closed and terminal regression tests. |

`internal/execution/fallback.go`, `internal/workflow/validate.go`, adapters, store, CLI, docs, `deltas-acceptance.md`, and positive headless behavior remain unchanged.

## Testing Strategy

Strict TDD starts with RED engine tests using `newAgentEngine`, `setupFakeManager`, `assertStepStatus`, and `assertAttemptEvidence`.

| Test | Assertions |
|---|---|
| Supervised rejection | A valid supervised agent returns an error containing `supervised mode not supported`; step and execution are `failed`; `assertAttemptEvidence(..., 0)`; an available `fakeHarnessAdapter` reports `NewSessionCount() == 0`; no transport row can exist without an attempt. |
| Terminal regression | Write a terminal-mode workflow, bypass `CreateExecution`, and seed the required project row; a `running` `store.Execution` with that `WorkflowSource`, shared/root workspace, and nil `DagHash`; and a `pending` agent `store.ExecutionStep` with `[]` dependency/require/produce fields. `RunStep` re-parses without validation; assert `terminal mode not supported`, failed step/execution, zero attempts, and zero sessions. |

This seed is required because `CreateExecution` rejects terminal mode during `ValidateFile`; nil `DagHash` skips hash re-flattening while `WorkflowSource` drives the runtime parse. The available fake is probed/initialized by test setup, but zero sessions specifically proves `RunStep` never invokes execution through the adapter. Zero attempts plus zero sessions proves the fallback loop was not entered. Existing headless engine tests cover the positive path; no new headless test is needed.

Verification: run `go test ./...` during RED/GREEN and then `go test ./...`, `go vet ./...`, and `go build ./...`. CI additionally gates race testing, golangci-lint, and govulncheck.

## Threat Matrix

The change controls a process-integration boundary by preventing adapter session/subprocess entry. The reference matrix rows are otherwise inapplicable.

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no executable classification changes | — | — |
| Git repository selection | N/A: no repository/cwd selection changes | — | — |
| Commit state | N/A: no VCS operation | — | — |
| Push state | N/A: no VCS operation | — | — |
| PR commands | N/A: no PR automation | — | — |

## Migration / Rollout and Risks

No migration or feature flag is required. Supervised workflows that previously ran headless will now fail intentionally; this prevents unapproved semantic downgrade. Guard placement risk is covered by zero-attempt and zero-session assertions, and terminal regression is pinned separately. Reverting only the guard would restore the unacceptable silent downgrade; rollback must revert this complete unit with its tests/spec, or ship real supervised support.

## Open Questions

None.
