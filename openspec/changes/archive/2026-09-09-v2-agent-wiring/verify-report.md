```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:d7e77f6f615ae1694733d37bfbb1f1c9154914efb0f6fa1ad26c3b33fcb9f7ae
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 4/4
scenarios: 9/9
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:8d106e74f3a0f4043c7a33d34cc82e41bfd4196083fe49b553fd1ad01428491c
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `v2-agent-wiring`
**Round**: 3 (final verification)
**Version**: v2-adapter delta (`F-01`, `F-04`, `F-06`, `U-04`)
**Mode**: Strict TDD
**Artifact store**: hybrid (OpenSpec + Engram)
**Evidence revision**: SHA-256 of HEAD `3abd3c369d4eed250175060452fac20b44d10abe`
**Date**: 2026-09-09

### Completeness

| Metric | Value |
|---|---:|
| Requirements retrieved | 4 |
| Scenarios retrieved | 9 |
| Tasks total | 15 |
| Tasks complete | 15 |
| Tasks incomplete | 0 |

Proposal, delta spec, design, tasks, apply-progress, and the previous verification report were read. All task checkboxes are complete. Remediation round 2 evidence and the current test sources were also inspected before judging the result.

### Build & Tests Execution

| Command | Exit | Output hash | Result |
|---|---:|---|---|
| `go test ./... -race -count=1` | 0 | `sha256:8d106e74f3a0f4043c7a33d34cc82e41bfd4196083fe49b553fd1ad01428491c` | PASS; 15 test-bearing packages green, 2 packages have no test files |
| `go build ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS; empty output |
| `go vet ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS; empty output |
| `scripts/verify-adapter-boundary.sh` | 0 | `sha256:c7a98bc5c5060dee465654e76c09b78002c2ac80f4b912c7cba46668346d654a` | PASS; provider-boundary and import-graph checks passed |
| `golangci-lint run` | 0 | `sha256:e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47` | PASS; 0 issues |
| `go test ./internal/cmd -run TestFlowCriterionTraceability -count=1` | 0 | `sha256:0407472b4912342bb7402b69ddff6c27f580a35884c7ae63042b27c187140ab5` | PASS; 92-criterion/11-spec traceability |
| `go test ./internal/cmd -race -v -count=1 -run '^TestAgentManagerCLIInjection($|StepReopen$)'` | 0 | `sha256:8e4997b3bdca03c655eac7082f0f456637e6fcd36ee586fe38579ca0d565deed` | PASS; both CLI lifecycle tests |
| Focused named tests with `-race -count=1 -v` | 0 | `sha256:22a1b12c377c0fa17c0d1dd178384bdd6ea5ebf0d9a278bfde067e320379f1d1` | PASS |

Coverage is unavailable by project configuration and no threshold is configured. `govulncheck` is configured as CI-only and was not part of the local verification command set.

### Named-Test Mapping

| Named test or gate | Result |
|---|---|
| `TestHarnessConfigKnownFields` | PASS; 6 subcases |
| `TestOpenCodeRealSessionLifecycle` | PASS; 8 subcases |
| `TestAgentManagerCLIInjection` | PASS; run, step run, and step skip wiring |
| `TestAgentManagerCLIInjectionStepReopen` | PASS; real CLI run → step run → step reopen → post-reopen step run, 2 attempts, fresh transport identity, 2 fixture invocations |
| `TestManager_ProbeCachedOnce` | PASS |
| `TestProbeCalledOncePerHarness` | PASS |
| `TestReopenDoesNotReprobeHarness` | PASS; one probe per harness across run → reopen → rerun |
| `TestCancelReturnsAfterLaunchFailure` | PASS; launch-failure and settled-success subcases |
| `TestAgentHarnessIntersectionFallback` | PASS |
| `TestAgentHarnessCandidatesExhausted` | PASS |
| `TestAgentStepFailsWithoutConfiguredHarness` | PASS; 3 subcases |
| `TestAgentStepPersistsRealEvidenceAndTransport` | PASS |
| `TestTransport_OptionalRow`, migration tests | PASS |
| `TestParser_OversizeViaCodec` | PASS |
| `scripts/verify-adapter-boundary.sh` | PASS |

### Spec Compliance Matrix

| Requirement | Scenario | Covering test or gate | Result |
|---|---|---|---|
| `v2-adapter/F-01` | CLI lifecycle | `TestAgentManagerCLIInjection`, `TestAgentManagerCLIInjectionStepReopen` | COMPLIANT; all four declared sites are runtime-covered, including real `step reopen` and post-reopen execution |
| `v2-adapter/F-01` | Real OpenCode session | `TestOpenCodeRealSessionLifecycle` | COMPLIANT |
| `v2-adapter/F-04` | Boundary gate | `scripts/verify-adapter-boundary.sh`, `TestParser_OversizeViaCodec` | COMPLIANT |
| `v2-adapter/F-04` | Strict optional configuration | `TestHarnessConfigKnownFields` | COMPLIANT |
| `v2-adapter/F-06` | Ordered intersection fallback | `TestAgentHarnessIntersectionFallback` | COMPLIANT |
| `v2-adapter/F-06` | Candidates exhausted | `TestAgentHarnessCandidatesExhausted` | COMPLIANT |
| `v2-adapter/F-06` | No configured or usable harness | `TestAgentStepFailsWithoutConfiguredHarness` | COMPLIANT |
| `v2-adapter/U-04` | Optional transport row and repeated migration | `TestTransport_OptionalRow`, migration tests | COMPLIANT |
| `v2-adapter/U-04` | Real identity and bounded redacted evidence | `TestAgentStepPersistsRealEvidenceAndTransport`, `TestAgentManagerCLIInjectionStepReopen` | COMPLIANT |

**Compliance summary**: 9/9 scenarios compliant; 4/4 requirements complete.

### Prior-Blocker Resolution

| Prior blocker | Resolution evidence | Result |
|---|---|---|
| Probe-once violation | `TestManager_ProbeCachedOnce`, `TestProbeCalledOncePerHarness`, `TestReopenDoesNotReprobeHarness`; engine consumes cached `ProbeResults()` | RESOLVED |
| OpenCode cancellation deadlock | `TestCancelReturnsAfterLaunchFailure`; `settleDone`/`doneOnce` covers launch and successful-settlement paths | RESOLVED |
| Missing `step reopen` runtime coverage | `TestAgentManagerCLIInjectionStepReopen` drives real CLI `Execute` through reopen and post-reopen run; asserts pending state, invalidated generation, 2 attempts, fresh identities, and 2 fixture invocations | RESOLVED |

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|---|---|---|
| `F-01` prior initialization and CLI injection | IMPLEMENTED | Manager probe results are cached once; `runAgentStep` does not re-probe; all four injection sites are runtime-covered; OpenCode cancellation remains idempotent and settled on early failure paths. |
| `F-04` adapter boundary and strict config | IMPLEMENTED | Strict YAML decoding, arbitrary harness keys, timeout/default validation, OpenCode-only factory registration, and boundary/oversized-frame gates are present and green. |
| `F-06` ordered fallback and no engine simulation | IMPLEMENTED | Ordered configured/enabled/registered/probed intersection, clean fallback, fail-closed exhaustion, bounded sanitized context, and no synthetic production markers are verified. |
| `U-04` transport-neutral attempts and evidence | IMPLEMENTED | Attempts remain transport-neutral; real identity, resolved requirements, redacted bounded payloads, and migration behavior are verified. |

### Design Coherence

| Decision | Followed? | Notes |
|---|---|---|
| CLI-direct manager lifecycle | Yes | One configured manager is injected at run, step run, step reopen, and step skip; reopen rerun preserves the manager probe snapshot. |
| OpenCode-only registration | Yes | The factory constructs only OpenCode; Claude and ACP remain unregistered. |
| Delete production simulation and inject test fakes | Yes | The engine has no synthesized success, identity, or output markers; simulated adapters remain unreachable by production factory wiring. |
| Strict generic harness configuration | Yes | YAML strict decoding and arbitrary data-key handling match the design. |
| Real identity, requirements, and bounded evidence | Yes | Subprocess, transport accessor, requirements resolution, inline payload, and repository behavior match the design and passing tests. |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | `apply-progress.md` contains the 15-row TDD Cycle Evidence table plus remediation-round evidence. |
| All tasks have test or gate evidence | PASS | 15/15 task rows name a test, fixture, or gate; referenced files exist. |
| RED confirmed | WARNING | Test files and remediation RED observations exist, but the artifact uses `Written`/`Passed` rather than the strict `✅ Written`/`✅ Passed` labels. |
| GREEN confirmed | PASS | Full race suite, focused tests, build, vet, linter, boundary, and traceability checks passed. |
| Triangulation adequate | PASS | The added CLI and engine tests cover the previously missing reopen and reopen probe-once paths. |
| Safety net for changed files | WARNING | Existing apply evidence uses `ok` for several new-file safety-net entries instead of strict `N/A (new)` notation. |

**TDD Compliance**: 4/6 checks fully satisfy the strict artifact notation or runtime criteria; remaining warnings are documentation notation only.

### Test Layer Distribution

| Layer | Test files | Tools |
|---|---:|---|
| Unit | 6 | Go `testing` |
| Integration | 6 | Go `testing`, executable fixtures, fake adapters, SQLite |
| E2E | 1 | Go CLI `Execute` plus executable OpenCode fixture |
| **Total** | **13** | |

### Changed File Coverage

Coverage analysis skipped — project configuration declares coverage unavailable and no threshold.

### Assertion Quality

**Assertion quality**: PASS. Reviewed changed and remediation test files call production code before asserting behavior. The new reopen test asserts preconditions, state transition, invalidated generation, attempt count, transport identity, and subprocess invocation count; no tautologies, ghost loops, smoke-only checks, or mock-heavy assertion defects were found.

### Quality Metrics

**Linter**: PASS; `golangci-lint run` reported 0 issues.
**Type checker**: PASS; `go vet ./...` exited 0 with empty output.

### Invariants

| Invariant | Result | Evidence |
|---|---|---|
| `deltas-acceptance.md` unchanged | PASS | `git diff --quiet -- deltas-acceptance.md` exited 0; SHA-256 `eae9209250b9fdbb119ae2f3e2f21939622284796843728c95494dd45f8461ec`. |
| 92 criteria / 11 specs | PASS | `TestFlowCriterionTraceability` exited 0. |
| Old synthetic markers | PASS | No `success from`, `opencode-session`, or `claude-session` markers found under `internal/`. |

### Issues Found

**CRITICAL**: None.

**WARNING**:

1. Strict TDD artifact labels and several new-file safety-net entries do not use the exact notation required by `strict-tdd-verify.md`.
2. The deliberately unregistered Claude adapter retains a simulated prompt implementation; this remains unreachable through the OpenCode-only factory and is explicitly allowed by the design.

**SUGGESTION**:

1. Normalize remediation TDD evidence to the exact strict labels and `N/A (new)` safety-net notation in a later documentation pass.

### Verdict

**PASS WITH WARNINGS** — All 4 requirements and 9 scenarios are compliant, all runtime gates pass, and the probe-once, cancellation, and reopen-coverage blockers are resolved. Remaining warnings are limited to strict TDD artifact notation and the intentionally unreachable unregistered Claude simulator.

**next_recommended**: `archive`

## Key Learnings

1. Reopen coverage must prove both the handler's state transition and a subsequent real agent execution; the added CLI test now does both.
2. Probe-once behavior must be asserted across the full run → reopen → rerun lifecycle, not only within one engine run.
3. A passing final verification can retain documentation-only warnings when all requirements and scenarios have runtime evidence.
