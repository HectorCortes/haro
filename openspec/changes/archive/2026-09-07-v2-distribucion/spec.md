# Delta for v2-distribucion

## Purpose

Define Go-native Haro distribution with reproducible installation proof, no post-install execution hooks, and offline initialization in a clean project.

## Constraints

- Distribution MUST remain pure Go/no cgo and MUST add no dependencies.
- Under Constitution §III.1, `go install` plus a CI-published release binary SHALL be the Go reinterpretation of F-01's historical “current distribution mechanism.” The literal npm wording in `deltas-acceptance.md` MUST remain unchanged except for archive checkbox/tracking updates; this interpretation MUST remain in the spec and archive trail.
- F-02 MUST prohibit package post-install scripts or equivalent hooks that execute code. Their current absence is a permanent regression gate, not permission to introduce them.
- F-03 MUST prove `haro init` in an empty project without prior dependencies or network access. Networked `go install` verification MUST be separate from offline initialization verification using `GOPROXY=off` or an equivalent network-denial control.

## ADDED Requirements

### Requirement: Go-native end-to-end installation [v2-distribucion/F-01]

Haro MUST be installable from the published module with `go install github.com/HectorCortes/haro@latest` and SHALL be published by tagged CI releases as one self-contained static `haro` binary. The README MUST document the actual install and release paths. A clean-environment verification MUST prove the installed command resolves and executes without additional manual steps.

#### Scenario: Clean module installation
- GIVEN a clean environment with an empty `GOBIN` and network access
- WHEN the published module is installed with the documented command
- THEN `haro` is resolvable from that installation and executes successfully

#### Scenario: Tagged release binary
- GIVEN a tagged release containing the published `haro` asset
- WHEN the asset is obtained in a clean environment and executed
- THEN it runs as a self-contained static binary without project dependencies

### Requirement: No executable installation hooks [v2-distribucion/F-02]

The distribution MUST NOT contain `package.json`, `postinstall`, install hooks, or equivalent package lifecycle mechanisms that execute code. CI SHALL fail closed when the regression gate or distribution-artifact review detects one.

#### Scenario: Hook-free distribution review
- GIVEN the repository and candidate distribution artifact
- WHEN the distribution gate inspects their package and lifecycle surfaces
- THEN no executable installation hook or npm package artifact is present

#### Scenario: Package regression rejection
- GIVEN a change introduces `package.json` or an executable install hook
- WHEN the distribution gate runs
- THEN verification fails and the artifact is rejected

### Requirement: Dependency-free offline initialization [v2-distribucion/F-03]

An already-built `haro` binary MUST initialize an empty project without prior project dependencies or network calls. Initialization MUST create the complete `.haro/` structure and SHALL be idempotent.

#### Scenario: Offline clean-project initialization
- GIVEN an empty project, an already-built `haro`, and disabled network access
- WHEN `haro init` runs with `GOPROXY=off` or equivalent isolation
- THEN `.haro/config.yaml`, `workflows/`, `skills/`, `artifacts/`, and `docs/` exist without a network call

#### Scenario: Offline initialization is idempotent
- GIVEN a successfully initialized project containing preserved user content
- WHEN `haro init` runs again with network disabled
- THEN it succeeds without replacing existing configuration or user content

### Requirement: Constitutional interpretation and traceability [v2-distribucion/TRACE-D08]

The specification and archive record MUST document `go install` as the Constitution §III.1 equivalent of F-01's historical `pnpm add -g shardeo`, with `haro` as the v2 command. Traceability SHALL map D08 exactly 3/3 to F-01, F-02, and F-03 without rewriting their contract text.

#### Scenario: D08 traceability audit
- GIVEN the contract, this specification, and the archived change record
- WHEN criterion-to-requirement mappings are audited
- THEN all three D08 criteria map one-to-one and the Go reinterpretation is explicit

## Non-Requirements

`shardeo doctor`, `shardeo setup`, CLI registration in `config.yaml`, GoReleaser multi-architecture releases, Homebrew, npm wrappers, broker/IPC, PTY, new dependencies, and edits to F-01's literal contract text are outside this change.
