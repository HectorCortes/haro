# Proposal: v2-distribucion (distribution)

## Intent

Deliver all D08 criteria (`v2-distribucion/F-01`–`F-03`, 3/3): Go-native installation, no executable install hooks, and reproducible offline `haro init`.

## Scope

### In Scope
- Define and clean-`GOBIN` test `go install github.com/HectorCortes/haro@latest`, the Go equivalent of global npm installation; require `haro` to resolve and execute.
- Add tag-triggered `.github/workflows/release.yml` using plain Go to test, build, and publish one self-contained binary. Keep `contents: write` job-local; publish via `git tag vX.Y.Z && git push origin vX.Y.Z`.
- Correct stale `README.md` status and document canonical installation, local build, releases, and tag publishing.
- Add CI-wired `scripts/verify-distribution.sh`; fail on npm/package artifacts or executable hooks (`postinstall`, `prepare`, `prepublishOnly`, or equivalents).
- Prove an already-built `haro init` initializes an empty project with `GOPROXY=off`, without dependencies/network. No implementation change is planned.
- Reinterpret F-01's npm/`shardeo` vocabulary under Constitution III.1. Preserve criterion text for audit history; record the Go mapping in the delta spec/archive report. Archive changes only checkboxes and tracking.

### Out of Scope
- `shardeo doctor/setup`, CLI registration in `config.yaml`, GoReleaser, multi-architecture releases, Homebrew, npm wrappers, broker/IPC, PTY, and new dependencies.

## Capabilities

### New Capabilities
- `v2-distribucion`: Go-native installation, release, hook-free supply-chain gate, and offline initialization proof.

### Modified Capabilities
- None.

## Approach

Adopt Approach 1 (150–300 lines). A separate workflow isolates tag-only writes from CI. Approach 2 leaves no published binary; 3 overbuilds with GoReleaser/Homebrew; 4 contradicts pure Go and F-02.

## Affected Areas

| Area | Impact |
|---|---|
| `.github/workflows/release.yml` | tag-driven binary release |
| `.github/workflows/ci.yml` | distribution gate/E2E |
| `README.md` | accurate install/publish guidance |
| `scripts/verify-distribution.sh` | hook/package regression gate |
| `internal/cmd/*_test.go` | clean-install and offline-init E2E |
| `deltas-acceptance.md` | archive checkboxes/tracking only |

## Risks

| Risk | Mitigation |
|---|---|
| Literal npm audit | Cite III.1; preserve text and record mapping in spec/report. |
| Release write permission | Job-local `contents: write`; tags only. |
| Network-dependent tests | Separate networked install proof from `GOPROXY=off` init proof. |
| `haro`/`shardeo` drift | Specify `haro` as the v2 canonical binary; no alias. |

## Rollback Plan

Remove `release.yml` and revert README guidance. Additive gates/tests may remain; no data/dependency rollback is needed.

## Dependencies

- Existing Go toolchain and GitHub Releases; normative authority: Deltas, `SPECS.md`, Constitution III.1–III.2.

## Traceability and Success Criteria

| Criterion | Deliverable/proof |
|---|---|
| F-01 | `go install` docs, tag release, clean `GOBIN` executable E2E |
| F-02 | permanent no-package/no-hook gate |
| F-03 | clean-project `haro init` E2E with `GOPROXY=off` |

- [ ] All 3/3 proofs and `go test ./...` pass reproducibly with no new dependencies.
