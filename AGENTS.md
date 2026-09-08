# AGENTS.md — Conventions for AI agents

This repository is about AI coding agents and is worked on with agents. These conventions apply to any agent (or human) editing the repo.

## Language

- The repository's written language is **English**: documentation, technical artifacts (including openspec/SDD artifacts), commit messages, PRs and issues are written in English.
- Conversation with the user may be in any language.
- Follow the existing document convention: professional tone, no emojis, no filler.

## Sources of truth

1. **`deltas-acceptance.md`** — the contract: 92 acceptance criteria organized in specs, with IDs like `v2-no-regresion/F-01` (`F-<n>` functional, `U-<n>` unit). A criterion is met only when its verification passes reproducibly.
2. **`docs/v2/haro-constitucion.md`** — project regulations (traceability, fail-closed, methodological neutrality, core without provider literals, stability rules).
3. **`docs/v2/haro-especificacion-tecnica.md`** — v2 technical specification (YAML schema, SQLite DDL, interfaces, JSON-RPC/ACP protocols).
4. **`docs/reference/`** — v1 behavior (migration oracle; v1 contracts are preserved unless v2 regulations say otherwise).

On conflict between sources, the constitution and the contract prevail. Do not modify existing documents without an SDD change that justifies it.

## Stack

- **Pure Go, no cgo** (SQLite via `modernc.org/sqlite`), CLI, JSON-RPC over Unix socket, YAML.
- **`go.mod` does not exist yet**: it will be created in the spike phase. Do not generate it in advance.
- **Security posture**: no third-party installation scripts or hooks that execute external code are incorporated into the repo (project policy; dependency installation goes through the standard package manager).

## Out of scope

- **Embedded terminal / PTY**: deferred. Do not design or implement anything on that surface.

## Development flow

- Each spec of `deltas-acceptance.md` is developed as an **SDD change** in `openspec/` (cycle `proposal → spec → design → tasks → apply → verify → archive`); completed changes are archived in `openspec/changes/archive/`.
- **Gate**: green CI (`go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint, govulncheck). Locally, at least `go test ./...`.
- **Conventional commits** (`feat(...)`, `fix(...)`, `docs(...)`, `refactor(...)`, `test(...)`, `chore(...)`) and small PRs as work units.
- When adding build or coverage artifacts, add the corresponding entries to `.gitignore` without deleting existing ones.