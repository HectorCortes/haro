# Design: M0 Go spike

## Technical approach

Bootstrap one pure-Go module and four disposable proof packages. `main.go` prints `haro v0.0.0-spike` and exits 0; no CLI framework is introduced. Each integration uses its smallest public boundary; F-01–F-05 remain repository gates. Strict TDD is not active because no Go runner exists yet; config enables it after bootstrap.

## Architecture decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| Remote casing or lowercase module | Casing drift breaks imports | Use `github.com/HectorCortes/haro`, exactly matching `origin`. |
| Go 1.23 or host 1.27 | Newer floor reduces compatibility | Declare `go 1.23`; the host may compile with 1.27. |
| Root `main.go` or `cmd/haro` | Root is less extensible | Use root `main.go` for spike minimalism; relocation is deferred. |
| Assert WAL result or execution | In-memory SQLite returns `memory` | Assert `PRAGMA journal_mode=WAL` has no error, never that its value is `wal`. |
| Long descriptive socket or short socket | Unix `sun_path` is commonly 108 bytes | Use `filepath.Join(t.TempDir(), "h.sock")` and always close/remove it. |

## Proof boundaries and flow

    sample YAML → workflow.Parse → validated subset
    SQLite DSN → pragmas → temp table → insert/select
    Unix client → health handler → JSON-RPC result
    Capabilities → JSON → Capabilities

`workflow.Parse(io.Reader)` decodes structs tagged for `version`, `name`, and `steps`; each step exposes `id`, `type`, and `run`. It errors unless version is 2 and at least one step exists. `store` opens `file::memory:?cache=shared` through the blank-imported `modernc.org/sqlite` driver and uses one `sql.Conn` so the temporary table cannot cross pooled connections. It verifies foreign keys equal 1, executes and scans the WAL pragma without comparing its value, then compares one selected value. `ipc` handles only `{"jsonrpc":"2.0","method":"health","id":1}` and emits `{"jsonrpc":"2.0","result":{"ok":true},"id":1}`. Transport/decode/write failures return errors. `adapter.Capabilities` has `ProtocolVersion int`, `Permission`, `Terminal`, `LoadSession bool`, and `Extra map[string]any`; JSON names are `protocolVersion`, `permission`, `terminal`, `loadSession`, and `extra`. The `_` prefix convention is documented, not enforced.

## File changes

| File | Action | Description |
|---|---|---|
| `go.mod`, `go.sum`, `main.go` | Create | Module, tidy-pinned dependencies, minimal executable. |
| `internal/workflow/parse.go`, `parse_test.go` | Create | Subset parser and table cases: valid, wrong version, empty steps. |
| `testdata/sample-workflow.yaml` | Create | `version: 2`, name, and `build` command step. |
| `internal/store/sqlite_test.go` | Create | Pure-Go in-memory SQLite proof in package `store`. |
| `internal/ipc/health.go`, `health_test.go` | Create | One-request Unix JSON-RPC health boundary. |
| `internal/adapter/capabilities.go`, `capabilities_test.go` | Create | Capabilities JSON preservation proof. |
| `openspec/config.yaml` | Modify | Replace stale TypeScript values with Go truth. |
| `.gitignore` | Modify | Append `*.out`, `coverage.*`, `*.cover`; preserve every existing line. |

Each proof package stays near 100 lines and introduces no later-spec abstractions. Dependencies are `modernc.org/sqlite` and `gopkg.in/yaml.v3`; apply selects current compatible versions through `go mod tidy`.

## Configuration mapping

Keep `schema`; set context to Go 1.23+, pure Go/no cgo, proof packages, standard `testing`, modernc SQLite, yaml.v3, Unix sockets, and `both`/`auto`/`single-pr`. Preserve `execution.mode: auto`, `artifact_store: both`, and `delivery_strategy: single-pr`; change budget `100000 → 20000`. Keep `strict_tdd: true`. Change runner to `{command: "go test ./...", framework: "testing"}`; retain all layers and unavailable coverage. Set linter to `{available: true, command: "golangci-lint run"}` and type checker to `{available: true, command: "go vet ./..."}`; remove TypeScript notes/guidelines. Set apply test command to `go test ./...`; verify commands to `go test ./... -race`, `go build ./...`, and `go vet ./...`.

## Testing and verification

Use table-driven workflow cases with `t.Run`; copy the fixture into `t.TempDir()`, open it, and pass the reader. SQLite is F-06, workflow U-02, IPC F-07, and capabilities U-03, one proof each. Socket tests use `net.Dial("unix", ...)` and cleanup; no golden files or external test dependencies. U-01 parses config and checks preserved ignores. F-01–F-05 run `go build ./...`, `go vet ./...`, `go test ./... -race`, `golangci-lint run`, and `govulncheck ./...`; additionally `go build -o haro .` proves the named artifact because a multi-package `./...` build may discard outputs.

## Threat matrix

The Unix socket process boundary is covered by F-07 and explicit transport errors. The required matrix has no applicable row:

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | No executable classification. |
| Git repository selection | N/A | No Git invocation. |
| Commit state | N/A | No commit automation. |
| Push state | N/A | No push automation. |
| PR commands | N/A | No PR automation. |

## Risks, rollout, and exclusions

Mitigations are exact remote casing, Go 1.23 floor, WAL success-only assertion, short socket path with cleanup, config rewrite, dependency scan, and a strict cut list. Rollout requires no migration; revert the spike files/config diff to roll back. Not designed: broker, full DDL or repository interfaces, adapter sessions, path claims, composition, reporting, distribution, PTY/terminal, or a CLI framework. Open questions: none.
