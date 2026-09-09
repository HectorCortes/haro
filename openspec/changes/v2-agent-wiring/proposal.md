# Proposal: V2 Agent Wiring

## Intent

Invoke harnesses for `agent` steps through the adapter contract. Persist real session identity and sanitized output (≤16 KiB), replacing simulation.

## Scope

### In Scope
- Add `.haro/config.yaml.harnesses: map[string]HarnessConfig{binary, env, timeout_seconds, enabled}` with `yaml.v3` `KnownFields(true)`.
- Build/probe/initialize one manager per CLI invocation; call `SetAdapterManager` at `handleRun`, `handleStepRun`, `handleStepReopen`, and `handleStepSkip`.
- Intersect ordered step harnesses with enabled configuration; unavailable candidates fall through cleanly and no usable candidate fails closed.
- Remove simulation. Tests inject fakes; absent configuration never succeeds, intentionally breaking simulation-dependent demos.
- Resolve `SessionBundle.Requires`; persist real `native_session_id`, `adapter_name`, protocol metadata, and once-redacted/bounded `attempt_events.payload`.
- Add the missing OpenCode `Adapter/NewSession` process path. Claude simulates prompts and ACP only translates; register neither until real-contract compliant.

### Out of Scope
- `v2-broker` (D01), `v2-ipc` (D02), broker hosting/concurrency, and PTY.
- Changes to `deltas-acceptance.md`: preserve 92 criteria/11 specs. This enables but does not claim `v2-no-regresion/F-04`.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `v2-adapter`: require CLI-direct lifecycle, real sessions, candidate intersection, and transport identity.

## Approach

`internal/project/config.go` parses configuration; an adapter-local factory contains implementations and initializes once. `internal/cmd/execute.go` wires all four sites. `internal/execution/engine.go` runs probe→initialize→NewSession→Prompt→Cancel and preserves fallback/evidence semantics.

`SPECS.md` and Constitution I.3/I.5/II.3/V/VIII require fail-closed neutrality, provider isolation, negotiation, and sanitized deltas. Technical Specification §§4/7 defines adapters; §6 and Constitution VII require broker hosting. CLI-direct `SessionHost` is temporary `deferred:cli-direct` tension; project harness configuration lacks normative text.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/project/config.go` | Strict harness config |
| `internal/adapter/{factory.go,opencode/,contract/}` | Real sessions; test seam |
| `internal/cmd/execute.go` | Four-site injection |
| `internal/execution/engine.go` | Selection, identity, evidence |
| `scripts/verify-adapter-boundary.sh` | Mandatory unchanged gate |

Tests: `TestHarnessConfigKnownFields`, `TestAgentHarnessIntersectionFallback`, `TestAgentStepFailsWithoutConfiguredHarness`, `TestAgentStepPersistsRealEvidenceAndTransport`. Contract fixtures cover lifecycle, not CLI wiring/process identity; add fake-binary/adapter seams. TDD: `go test ./...`.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Binary unavailable/misclassified | High | Probe; clean fallback |
| Provider literal leakage | Medium | Boundary gate |
| Timeout/process leak | Medium | Context deadline; idempotent Cancel |
| Raw/oversized evidence | Medium | Single redact→16 KiB boundary |
| Session identity API gap | High | Tested accessor/result and transport update |

## Rollback Plan

Disable harnesses to fail closed, then revert wiring as one unit; retain compatible evidence/transport rows. Never restore silent simulation.

## Dependencies

- Adapter/store contracts and binaries.
- Delivery: `single-pr`, maintainer pre-approved `size:exception` (200000 lines), direct push to `main`, no PR.

## Success Criteria

- [ ] Configured fallback reaches a real harness and records identity plus bounded redacted evidence.
- [ ] Missing, disabled, unavailable, or unknown harnesses fail closed; all named tests, `go test ./...`, boundary, and 92/11 traceability gates pass.
