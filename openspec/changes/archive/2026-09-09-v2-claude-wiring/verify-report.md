```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:fb81f8864b5c51e8f64ac5e3bdba568d7caf5b7862524509d526bd6f518566f6
verdict: pass
blockers: 0
critical_findings: 0
requirements: 2/2
scenarios: 9/9
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:17e807fe2c65b26e036d9f6ee2852064a6f6d8632b65725c849e55e6e2a87b13
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `v2-claude-wiring`
**Version**: v2-adapter delta (`F-01`, `F-05`)
**Mode**: Strict TDD
**Artifact store**: hybrid (OpenSpec + Engram)
**Evidence revision**: `sha256:fb81f8864b5c51e8f64ac5e3bdba568d7caf5b7862524509d526bd6f518566f6`
**Date**: 2026-09-09

### Completeness
| Metric | Value |
|---|---:|
| Requirements retrieved | 2 |
| Scenarios retrieved | 9 |
| Tasks total | 20 |
| Tasks complete | 20 |
| Tasks incomplete | 0 |

All required context files were read: proposal, delta spec, design, tasks, and apply-progress. Native status also reports `allComplete: true`, `verify: ready`, and no blocked reasons.

### Build & Tests Execution
| Command | Exit | Output hash | Result |
|---|---:|---|---|
| `go test ./... -race -count=1` | 0 | `sha256:17e807fe2c65b26e036d9f6ee2852064a6f6d8632b65725c849e55e6e2a87b13` | PASS; 15 test-bearing packages green, 2 packages have no test files |
| `go build ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS; empty output |
| `go vet ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS; empty output |
| `scripts/verify-adapter-boundary.sh` | 0 | `sha256:c7a98bc5c5060dee465654e76c09b78002c2ac80f4b912c7cba46668346d654a` | PASS; boundary and import-graph checks passed |
| Focused named tests (`-race -count=1 -v`) | 0 | `sha256:5c945ecd0e2fd5d097250e270f019032b3334aa46f8a4b4b8aa6b6c869783026` | PASS; every requested named test passed |
| Targeted `golangci-lint run` on changed packages | 1 | `sha256:92f2d6b13a85ec592fe7836d37e881ae7159d50d6636bd93afe5223bd7af993a` | WARNING; one unused test helper |

Coverage analysis skipped: `openspec/config.yaml` declares coverage unavailable and no threshold.

### Named-Test Mapping
| Test | Result |
|---|---|
| `internal/cmd/agent_wiring_test.go > TestAgentManagerCLIInjection` | PASS |
| `internal/adapter/opencode/adapter_test.go > TestOpenCodeRealSessionLifecycle` | PASS |
| `internal/adapter/claude/adapter_test.go > TestClaudeRealSessionLifecycle` | PASS |
| `internal/adapter/claude/parser_test.go > TestClaudeParseStreamJSON` | PASS |
| `internal/adapter/claude/adapter_test.go > TestClaudePromptViaStdin` | PASS |
| `internal/adapter/claude/adapter_test.go > TestClaudePermissionDontAsk` | PASS |
| `internal/adapter/claude/adapter_test.go > TestClaudeZeroTextFailsCleanly` | PASS |
| `internal/execution/agent_claude_evidence_test.go > TestClaudeEvidenceBoundedAndRedacted` | PASS |
| `internal/adapter/contract/claude_suite_test.go > TestClaudeContractSuiteFixture` | PASS |
| `internal/execution/agent_fallback_test.go > TestProbeCalledOncePerHarness` | PASS |
| `internal/adapter/manager_probe_once_test.go > TestManager_ProbeCachedOnce` | PASS |
| `internal/execution/agent_fallback_test.go > TestReopenDoesNotReprobeHarness` | PASS |

The focused command also passed the additional `TestClaudeParseStreamJSON_FirstSessionIDWins` and the full Claude/OpenCode lifecycle subtests. Opt-in real-binary tests are correctly skip-gated when `HARO_TEST_CLAUDE_BINARY` is unset.

### Spec Compliance Matrix
| Requirement | Scenario | Covering test | Result |
|---|---|---|---|
| `v2-adapter/F-01` | CLI lifecycle | `TestAgentManagerCLIInjection` | COMPLIANT |
| `v2-adapter/F-01` | Real OpenCode and Claude sessions | `TestOpenCodeRealSessionLifecycle`; `TestClaudeRealSessionLifecycle` | COMPLIANT |
| `v2-adapter/F-05` | Real session lifecycle | `TestClaudeRealSessionLifecycle` | COMPLIANT |
| `v2-adapter/F-05` | Envelope mapping | `TestClaudeParseStreamJSON` | COMPLIANT |
| `v2-adapter/F-05` | Prompt via stdin | `TestClaudePromptViaStdin` | COMPLIANT |
| `v2-adapter/F-05` | Fail-closed permission | `TestClaudePermissionDontAsk` | COMPLIANT with N2 warning |
| `v2-adapter/F-05` | Zero-text clean failure | `TestClaudeZeroTextFailsCleanly` | COMPLIANT |
| `v2-adapter/F-05` | Bounded redacted evidence | `TestClaudeEvidenceBoundedAndRedacted` | COMPLIANT |
| `v2-adapter/F-05` | Public boundary fixture | `TestClaudeContractSuiteFixture` | COMPLIANT |

**Compliance summary**: 9/9 scenarios compliant; 2/2 requirements complete.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|---|---|---|
| `F-01` prior initialization and CLI injection | IMPLEMENTED | Factory constructs one manager, probes once, initializes available adapters once, and all four agent-capable CLI handlers inject a manager. Engine consumes cached `ProbeResults()` without re-probing. |
| `F-05` real Claude contract | IMPLEMENTED | Claude uses `exec.CommandContext` with fixed print-mode argv, stdin-only prompt, workspace `Dir`, inherited environment plus overlay, bounded timeout, first native `session_id`, stream mapping, fail-closed zero-text behavior, and transport/evidence persistence. The acceptance checkbox remains intentionally unchecked. |

### Design Coherence
| Decision | Followed? | Notes |
|---|---|---|
| CLI-direct manager lifecycle | Yes | Manager probe/init precedes sessions; OpenCode and Claude fixture sessions use the generic engine intersection/fallback/evidence path. |
| Fixed Claude print-mode invocation and stdin prompt | Yes | No shell and no positional prompt; argv and stdin are asserted at runtime. |
| Default `dontAsk` and bounded/idempotent settlement | Yes | Fixed flag, timeout, `doneOnce`, `cancelOnce`, and `settleDone` are present and exercised. Live `dontAsk` semantics remain unconfirmed (N2). |
| Exact Claude registration / ACP remains unregistered | Partial | Factory additionally maps `claudecode` to Claude and maps an enabled `acp` key to the generic OpenCode path. No dedicated ACP implementation is constructed; this preserves arbitrary OpenCode data keys but differs from the design's exact-key wording. |
| Generic engine and scope boundaries | Yes | `internal/execution/engine.go` is unchanged; no ACP/PTY/broker/IPC implementation was added. |
| Single redaction point and transport-neutral evidence | Yes | `AgentEvidence` flows through `VisibleEvidence`/`Redact`; Claude transport metadata and native identity are persisted in `attempt_transport`. |

### TDD Compliance
| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | `apply-progress.md` contains the TDD Cycle Evidence table and all 20 tasks are checked. |
| All tasks have test or gate evidence | PASS | 20/20 tasks are represented; test/fixture work and final gate/document tasks are evidenced. |
| RED confirmed (test files exist) | WARNING | Referenced files exist and the focused tests pass, but apply-progress uses descriptive variants (`build fail`, `ok`) rather than strict `✅ Written`/`✅ Passed` labels. |
| GREEN confirmed (tests pass) | PASS | Full race suite and the focused named-test command pass. |
| Triangulation adequate | PASS | Parser, lifecycle, cancellation, permission, zero-text, evidence, factory, and fallback behaviors have multiple distinct cases. |
| Safety net | PASS | Apply-progress records baseline runs for modified code and `N/A (new)` for new fixture artifacts. |

**TDD Compliance**: 5/6 checks fully satisfy strict artifact notation/runtime criteria; the remaining item is documentation notation.

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|---|---:|---:|---|
| Unit | 4 | 2 | Go `testing` |
| Integration | 19 | 5 | Go `testing`, executable fixtures, SQLite |
| E2E | 0 | 0 | No browser/E2E tool; real binary is opt-in |
| **Total** | **23** | **7** | |

### Changed File Coverage
Coverage analysis skipped — no coverage tool configured.

### Assertion Quality
**Assertion quality**: All changed test files call production code before asserting behavior. No tautologies, ghost loops, smoke-only tests, or mock-heavy assertion defects were found. The requested named tests all make behavioral assertions; opt-in variants are explicitly gated.

### Invariants
| Invariant | Result | Evidence |
|---|---|---|
| `deltas-acceptance.md` unchanged | PASS | Base/current Git blob IDs match (`358a729268a5a349a0e3be10d76b7577d70af9c4`); current SHA-256 is `eae9209250b9fdbb119ae2f3e2f21939622284796843728c95494dd45f8461ec`. |
| 92 criteria / 11 specs | PASS | Tracking summary reports 92 criteria and 11 specs. |
| `v2-adapter/F-05` remains unclaimed | PASS | `deltas-acceptance.md:274` remains `[ ]`. |
| Normative `docs/v2` unchanged | PASS | Base/current hashes match for both v2 normative documents; tracked diff is empty. |
| No simulation / no ACP-PTY-broker-IPC implementation | PASS | Claude production path is a real subprocess; changed-file diff contains only adapter/factory/parser/evidence/test/fixture files; boundary gate passes. |

### Issues Found
**CRITICAL**: None.

**WARNING**:
1. N2 residual: `dontAsk` deny-vs-auto-allow behavior and nested `stream_event.event.delta` semantics are pinned by synthetic.2 and not live-confirmed against a real Claude binary. Opt-in tests fail loudly on drift but were skipped because `HARO_TEST_CLAUDE_BINARY` is unset.
2. Strict TDD artifact notation is noncanonical, and apply-progress subtest counts are stale in places (for example, parser claims 9 while the runtime output shows 10).
3. `golangci-lint` found the unused helper `newSessionFactoryProbe` at `internal/adapter/factory/factory_test.go:377`; the required Go test/build/vet/boundary gates remain green.
4. Factory behavior differs from the exact-key wording: `claudecode` is also routed to Claude, while an enabled `acp` data key is routed to OpenCode rather than being absent. No ACP adapter implementation is present; reconcile this ambiguity before any normative update.
5. `ParseStreamJSON` checks only the first JSON value on a newline; trailing non-whitespace or a second JSON value on the same line is not explicitly rejected. The current malformed test covers truncated JSON, not this case.

**SUGGESTION**: None.

### Verdict
**PASS WITH WARNINGS** — All 2 requirements and 9 scenarios have passing named runtime coverage, all required gates pass, all 20 tasks are complete, and the requested invariants hold. Warnings are limited to documented live-binary residuals, strict-TDD artifact notation, one test-only linter issue, registration wording ambiguity, and an untested parser-hardening edge.

**next_recommended**: `archive`
