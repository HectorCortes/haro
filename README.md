# Haro

> **Harnesses Orchestrator** — orchestrator of AI coding agents through harnesses.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Documentation is in English; issues/PRs may be in English or Spanish.

Haro structures and manages the execution of AI-assisted development workflows. It is not an agent nor an IDE: it does not replace OpenCode, Codex CLI, Claude Code or other coding agents. It treats them as **harnesses** (interchangeable agent runtimes) and gives them structure: what to run, with what context, in what order and with which runtime, separating methodology (workflows), knowledge (skills and artifacts) and execution (per-harness adapters).

Haro is the **v2 successor of Shardeo**: it preserves the model of step-based workflows, dependencies, artifacts and traceability, and rebuilds it on a persistent per-project broker, JSON-RPC IPC between CLI and broker, an ACP-based adapter contract, workspace isolation via `path_claims`, workflow composition, change reporting and a SQLite-backed store.

## Status

**Pre-alpha — in active migration from TypeScript to Go.** The "complete" contract is `deltas-acceptance.md` (92 acceptance criteria organized in specs, each with an ID like `v2-no-regresion/F-01`). The v2 technical specification defines the YAML schema, the SQLite DDL, the interfaces and the protocols; the Go code does not exist yet (`go.mod` will be created in the spike phase). There is no publishable binary or stable user-facing commands yet.

**Target stack:** pure Go, no cgo (SQLite via `modernc.org/sqlite`), CLI, JSON-RPC over Unix socket, YAML.

## Documentation

- [deltas-acceptance.md](deltas-acceptance.md) — verifiable acceptance criteria; the contract for what "complete" means.
- [docs/v2/haro-constitucion.md](docs/v2/haro-constitucion.md) — normative project constitution.
- [docs/v2/haro-especificacion-tecnica.md](docs/v2/haro-especificacion-tecnica.md) — v2 technical specification (schema, DDL, interfaces, protocols).
- [docs/reference/](docs/reference/) — v1 behavior reference (oracle of the migration).

## Development

Requirements: Go (the stable version; `go.mod` will be created in the spike phase).

```sh
go test ./...
```

CI runs `go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint and govulncheck on every push to `main` and on every pull request.

- [CONTRIBUTING.md](CONTRIBUTING.md) — how to contribute.
- [SECURITY.md](SECURITY.md) — how to report vulnerabilities.

## License

MIT. See [LICENSE](LICENSE).