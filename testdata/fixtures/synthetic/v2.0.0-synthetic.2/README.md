# Synthetic Fixtures v2.0.0-synthetic.2

Provenance: synthetic neutral fixture derived from the pinned Claude Code 2.1.245 stream-json envelope assumptions documented in `openspec/changes/v2-claude-wiring/design.md` (Interfaces / Contracts mapping table). Real Claude 2.1.245 stream originals were unavailable in this repository; this fixture is not byte-identical to real CLI output and makes no byte-identity claim.

Version: v2.0.0-synthetic.2

Contents:
- `claude-fixture.jsonl` — synthetic Claude stream-json frames: `system/init` with the native `session_id`, an `assistant` text frame, a nested `stream_event` content-block delta, and a terminal success `result`.

Usage:
- `internal/adapter/contract/claude_suite_test.go` (`TestClaudeContractSuiteFixture`) generates an executable fixture in `t.TempDir()` that emits exactly these pinned frames, so `go test ./... -short` crosses the subprocess, protocol, JSON, and adapter-settlement boundary without an installed or authenticated real binary.
- Real Claude binary path is opt-in via `HARO_TEST_CLAUDE_BINARY` (non-short mode, cycle-end gate only); the opt-in variant fails loudly on envelope or flag drift.

Notes:
- Fixtures are versioned and isolated; updates must bump the version directory.
- The nested `stream_event.event.delta` shape and the `dontAsk` deny-vs-auto-allow semantics are pinned envelope expectations (N2): they are not live-confirmed against the real binary.
- No provider credentials are present; samples are sanitized.
