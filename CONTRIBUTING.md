# Contributing to Haro

Thank you for wanting to contribute. This document describes the workflow and the criteria that every change must meet to be integrated.

## Workflow

1. **Fork** the repository and clone it locally.
2. Create a **branch** with a descriptive name (`feat/broker-ipc`, `fix/race-store`, `docs/constitution`).
3. Implement the change in **small work units** (small PRs and commits, focused on a single thing).
4. Open a **pull request** against `main` describing the change (use the included template).
5. Wait for **review**. **1 approval** is required to merge.
6. The merge is **squash only**: each PR is integrated as a single commit with the branch's message.

**No** commit signing or DCO is required: unsigned commits are welcome.

## Commit convention

Conventional commits (`type(scope): description`), in English following the repository language convention:

- `feat(broker): persist executions per project`
- `fix(ipc): fix event replay after reconnection`
- `docs(constitution): clarify capability rule`
- `refactor(store): extract repository interface`
- `test(adapter): cover capability negotiation`
- `chore(ci): add govulncheck job`

Types used: `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `chore`.

## Integration gates

A PR is only integrated if all of the following are met:

- **Green CI**: `go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint and govulncheck pass on the PR.
- **Contract respected**: `deltas-acceptance.md` is the contract for what "complete" means. The PR must **reference the IDs of the criteria** it touches (for example, `v2-broker/F-01`, `v2-adapter/U-03`) in its description.
- **Bounded scope**: small, reviewable changes. If the PR touches several specs, split it into several PRs.
- **Consistent documentation**: if the change alters normative behavior, it must be reconciled with `docs/v2/` and with the criteria of `deltas-acceptance.md`.

## How development is structured

Each spec of `deltas-acceptance.md` is developed as an **SDD change** under `openspec/`, with the cycle `proposal → spec → design → tasks → apply → verify → archive`. Completed changes are archived in `openspec/changes/archive/`; current specs live in `openspec/specs/`.

- The criteria of `deltas-acceptance.md` are the source of each change's acceptance criteria.
- A delta is complete only when all its P0 and P1 criteria pass and the change is archived.
- Review `docs/v2/haro-constitucion.md` and `docs/reference/` before touching behavior: the constitution is normative and the reference is the oracle of v1 behavior during the migration.

## Reporting bugs and requesting features

- Bugs and features: open an issue using the templates in `.github/ISSUE_TEMPLATE/`.
- Security vulnerabilities: **do not** open a public issue; follow [SECURITY.md](SECURITY.md).