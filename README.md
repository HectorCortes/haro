# Haro

> **Harnesses Orchestrator** — orchestrator of AI coding agents through harnesses.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Documentation is in English; issues/PRs may be in English or Spanish.

Haro structures and manages the execution of AI-assisted development workflows. It is not an agent nor an IDE: it does not replace OpenCode, Codex CLI, Claude Code or other coding agents. It treats them as **harnesses** (interchangeable agent runtimes) and gives them structure: what to run, with what context, in what order and with which runtime, separating methodology (workflows), knowledge (skills and artifacts) and execution (per-harness adapters).

Haro is the **v2 successor of Shardeo**: it preserves the model of step-based workflows, dependencies, artifacts and traceability, and rebuilds it on a persistent per-project broker, JSON-RPC IPC between CLI and broker, an ACP-based adapter contract, workspace isolation via `path_claims`, workflow composition, change reporting and a SQLite-backed store.

## Status

**Pre-alpha — in active migration from TypeScript to Go.** The "complete" contract is `deltas-acceptance.md` (92 acceptance criteria organized in specs, each with an ID like `v2-no-regresion/F-01`). The v2 technical specification defines the YAML schema, the SQLite DDL, the interfaces and the protocols. The Go module (`github.com/HectorCortes/haro`) is installable today (see [Installation](#installation)); command surface and workflows remain pre-alpha.

**Target stack:** pure Go, no cgo (SQLite via `modernc.org/sqlite`), CLI, JSON-RPC over Unix socket, YAML.

## Installation

Install with the Go toolchain (Go 1.25+):

```sh
go install github.com/HectorCortes/haro@latest
```

Or download the static `haro` binary from a tagged [release](https://github.com/HectorCortes/haro/releases) (a `haro.sha256` checksum is published alongside it), then run it from your project root:

```sh
chmod +x haro
./haro init
```

`haro init` is offline and dependency-free: it creates `.haro/{config.yaml,workflows/,skills/,artifacts/,docs/}` in the current project without any network access, and it is idempotent (re-running preserves your configuration and content).

### Release verification notes

- Networked installation is **not** exercised in ordinary CI: `go install ...@latest` is fragile against registry/proxy and tag timing, and coupling every pull request to that would make CI noisy. The repository gate instead proves that `haro init` works fully offline (`GOPROXY=off`) against the built binary.
- The release workflow (`.github/workflows/release.yml`, triggered only by `v*` tags) installs the **exact tag** with `GOPROXY=direct` into a clean `GOBIN` and executes it before publishing the binary asset.
- Resolution of `@latest` after a release is a manual post-release check (module proxies may lag a freshly pushed tag).

## Documentation

- [deltas-acceptance.md](deltas-acceptance.md) — verifiable acceptance criteria; the contract for what "complete" means.
- [docs/v2/haro-constitucion.md](docs/v2/haro-constitucion.md) — normative project constitution.
- [docs/v2/haro-especificacion-tecnica.md](docs/v2/haro-especificacion-tecnica.md) — v2 technical specification (schema, DDL, interfaces, protocols).
- [docs/reference/](docs/reference/) — v1 behavior reference (oracle of the migration).

## Development

Requirements: Go (stable version; see `go.mod`).

```sh
go test ./...
```

CI runs `go build ./...`, `go vet ./...`, the distribution gate (`scripts/verify-distribution.sh`), `go test ./... -race`, golangci-lint and govulncheck on every push to `main` and on every pull request. Tagged releases additionally publish a static binary; see [Installation](#installation).

- [CONTRIBUTING.md](CONTRIBUTING.md) — how to contribute.
- [SECURITY.md](SECURITY.md) — how to report vulnerabilities.

## License

MIT. See [LICENSE](LICENSE).