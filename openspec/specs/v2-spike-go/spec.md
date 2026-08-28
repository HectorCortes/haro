# v2-spike-go Specification

## Purpose

Prove the minimal pure-Go bootstrap and integrations required by later v2 changes.

## Requirements

### Requirement: Buildable bootstrap [v2-spike-go/F-01] — P0 [E2E]

The project MUST declare module `github.com/HectorCortes/haro` with Go floor 1.23, build without cgo, and use only the standard package manager without third-party install scripts.

#### Scenario: Build binary
- GIVEN the bootstrap and a clean workspace
- WHEN `go build ./...` runs
- THEN it exits 0 and produces `haro`

### Requirement: Vet-clean bootstrap [v2-spike-go/F-02] — P0 [E2E]

The bootstrap MUST pass Go static analysis.

#### Scenario: Vet packages
- GIVEN all spike packages
- WHEN `go vet ./...` runs
- THEN it exits 0 without findings

### Requirement: Race-clean tests [v2-spike-go/F-03] — P0 [E2E]

All spike proofs MUST pass under the race detector.

#### Scenario: Test with race detector
- GIVEN the complete spike test suite
- WHEN `go test ./... -race` runs
- THEN it exits 0 without skipped required proofs or races

### Requirement: Lint-clean bootstrap [v2-spike-go/F-04] — P0 [E2E]

All Go code MUST satisfy the repository `.golangci.yml` policy.

#### Scenario: Run configured linter
- GIVEN the repository linter configuration
- WHEN `golangci-lint run` runs
- THEN it exits 0 without findings

### Requirement: Vulnerability-clean dependencies [v2-spike-go/F-05] — P0 [E2E]

The selected Go dependency graph MUST have no reported reachable vulnerabilities.

#### Scenario: Scan packages
- GIVEN the resolved module graph
- WHEN `govulncheck ./...` runs
- THEN it exits 0 without vulnerability findings

### Requirement: Go-aligned project configuration [v2-spike-go/U-01] — P0 [UNIT]

`openspec/config.yaml` MUST retain `auto`/`both`/`single-pr`, set runner `go test ./...`, `strict_tdd: true`, and budget 20000, and identify Go, `go vet`, and `golangci-lint`. Build/coverage ignores MUST be appended without removing existing `.gitignore` entries.

#### Scenario: Inspect configuration update
- GIVEN the preflight configuration and existing ignore entries
- WHEN the rewritten files are parsed and compared
- THEN all required values and prior ignore entries are present

### Requirement: Sample workflow parsing [v2-spike-go/U-02] — P0 [UNIT]

The YAML proof MUST parse the sample workflow and preserve its required bootstrap fields.

#### Scenario: Parse sample YAML
- GIVEN `testdata/sample-workflow.yaml`
- WHEN it is parsed
- THEN `version == 2` and `steps[0].id == "build"`

### Requirement: Pure-Go SQLite roundtrip [v2-spike-go/F-06] — P0 [INT]

The SQLite proof MUST use `modernc.org/sqlite`, enable foreign keys, execute `PRAGMA journal_mode=WAL` successfully in memory without requiring mode `wal`, and complete create/insert/select.

#### Scenario: Exercise in-memory database
- GIVEN a newly opened in-memory database
- WHEN pragmas and the roundtrip execute
- THEN no operation fails and the selected row matches the inserted row

### Requirement: Unix-socket health roundtrip [v2-spike-go/F-07] — P0 [INT]

The IPC proof MUST exchange a JSON-RPC 2.0 `health` request and response over a Unix socket using `t.TempDir()` plus a short name such as `h.sock`.

#### Scenario: Exchange health message
- GIVEN a listener on the short temporary socket path
- WHEN a valid `health` request is sent
- THEN the response preserves JSON-RPC 2.0 identity and reports success

### Requirement: Capabilities JSON preservation [v2-spike-go/U-03] — P0 [UNIT]

The adapter proof MUST roundtrip Capabilities JSON without losing protocol version, permission, or namespaced extra keys.

#### Scenario: Roundtrip capabilities
- GIVEN capabilities containing protocol, permission, and `_`-prefixed extra data
- WHEN JSON marshal and unmarshal complete
- THEN those values are equal to the input values

## Non-Requirements and Ownership

- Broker lifecycle belongs to `v2-broker`.
- Full JSON-RPC methods and notifications belong to `v2-ipc`.
- Adapter/session/host methods belong to `v2-adapter`.
- Path claims and workspace isolation belong to `v2-path-claims`.
- Workflow composition and cycle handling belong to `v2-composicion`.
- Change reporting belongs to `v2-reporte`.
- DDL, migrations, repositories, and persistence belong to `v2-store`.
- Distribution belongs to `v2-distribucion`.
- PTY/terminal remains deferred by `AGENTS.md`; no owner is assigned here.
- CLI frameworks remain deferred to an unassigned future change.
- `docs/v2/*` and `deltas-acceptance.md` MUST remain untouched by this change.
