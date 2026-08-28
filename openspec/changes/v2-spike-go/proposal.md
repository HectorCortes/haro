# Proposal: M0 Go spike

## Intent

Bootstrap pure-Go, no-cgo Haro and prove foundational integrations for later scored v2 specs. This spike is not scored by `deltas-acceptance.md`.

## Scope

### In Scope
- Add `go.mod`/`go.sum` (`github.com/HectorCortes/haro`, Go 1.23; `modernc.org/sqlite`, `gopkg.in/yaml.v3`) and minimal `main.go`.
- Add proof packages `internal/{workflow,store,ipc,adapter}`, first tests, and `testdata/sample-workflow.yaml`.
- Rewrite `openspec/config.yaml` to Go truth: `go test ./...`, `strict_tdd: true`, review budget 20000, Go context and quality tools.
- Append Go build/coverage entries to `.gitignore`; preserve existing entries.

### Out of Scope
- Broker (`v2-broker`); full JSON-RPC (`v2-ipc`); adapter/session methods (`v2-adapter`); path isolation (`v2-path-claims`); composition (`v2-composicion`); reporting (`v2-reporte`); DDL/migrations/persistence (`v2-store`); distribution (`v2-distribucion`).
- PTY/terminal (deferred by `AGENTS.md`); CLI frameworks (`flag`/Cobra deferred).
- Edits to `docs/v2/*` or `deltas-acceptance.md`.

## Capabilities

### New Capabilities
- `v2-spike-go`: buildable Go bootstrap and YAML, SQLite, Unix-socket JSON-RPC, and Capabilities-shape proofs.

### Modified Capabilities
- None.

## Approach

Implement the ultra-minimal approach as one bounded work unit with small conventional commits in one PR. Avoid later-spec abstractions.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `go.mod`, `go.sum`, `main.go` | New | Bootstrap. |
| `internal/{workflow,store,ipc,adapter}/`, `testdata/` | New | Proofs. |
| `openspec/config.yaml`, `.gitignore` | Modified | Go configuration. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Module-path casing | Medium | Match remote `HectorCortes`. |
| WAL on `:memory:` | Medium | Assert success, not `wal`. |
| Unix path limit | Low | Short name under `t.TempDir()`. |
| Stale SDD config | High | P0 rewrite in this spike. |
| Scope creep | Medium | Enforce mapped cut list. |
| govulncheck Go drift | Low | Keep stable Go pin. |
| Stale “TypeScript kept” header in `deltas-acceptance.md` | Medium | Record governance debt; do not edit here. |

## Rollback Plan

Revert spike commits. Removing `go.mod` restores pre-bootstrap state; no runtime state exists.

## Dependencies

- Go 1.23+, `modernc.org/sqlite`, `gopkg.in/yaml.v3`; `docs/v2/haro-constitucion.md` and technical specification.

## Success Criteria

- [ ] **[E2E]** `go build ./...` exits 0 and produces `haro`.
- [ ] **[E2E]** `go vet ./...` is clean.
- [ ] **[E2E]** `go test ./... -race` is green.
- [ ] **[E2E]** `golangci-lint` is green with `.golangci.yml`.
- [ ] **[E2E]** `govulncheck ./...` is clean.
- [ ] **[UNIT]** Config has `go test ./...`, strict TDD true, budget 20000, Go context, `go vet`, and `golangci-lint`.
- [ ] **[UNIT]** Sample YAML parses with `version == 2` and `steps[0].id == "build"`.
- [ ] **[INT]** In-memory SQLite enables foreign keys and completes create/insert/select roundtrip.
- [ ] **[INT]** A short-path Unix socket completes JSON-RPC `health` roundtrip.
- [ ] **[UNIT]** Capabilities JSON roundtrip preserves protocol, permission, and extra keys.
