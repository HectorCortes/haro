# Synthetic Fixtures v2.0.0-synthetic.1

Provenance: synthetic neutral fixtures derived from `docs/v2/haro-especificacion-tecnica.md` §7 and Agent Client Protocol shape. Original OpenCode 1.17.18 JSONL originals were unavailable in this repository (deferred via `v2-no-regresion`); these fixtures are not byte-identical to those originals and make no byte-identity claim.

Version: v2.0.0-synthetic.1

Contents:
- `acp-fixture.json` — minimal ACP initialize → session/new → prompt → update → cancel → request_permission cycle.
- `opencode-fixture.jsonl` — synthetic OpenCode JSONL events with `cursor`, `delta`, neutral payloads.

Usage:
- Contract suite `internal/adapter/contract` and `internal/adapter/acp` use these fixtures for fast-path verification without requiring a real binary. CI runs with `go test ./... -short` use fixtures only.
- Real Claude binary path is opt-in via `HARO_TEST_CLAUDE_BINARY`; fixtures do not attempt to replicate provider literals.

Notes:
- Fixtures are versioned and isolated; updates must bump the version directory.
- No provider credentials are present; samples are sanitized.

