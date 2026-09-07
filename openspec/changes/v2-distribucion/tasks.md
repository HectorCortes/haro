# Tasks: v2 distribution (D08)

## Review Workload Forecast

Estimated changed lines: 150–300.
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

size:exception pre-approved (200000); single-pr, direct push.

### Work Units

- U1 Offline init E2E: test `go test ./internal/cmd -run TestDistribution -count=1`; harness built binary, init×2, GOPROXY=off; rollback delete `internal/cmd/distribution_e2e_test.go`.
- U2 F-02 gate: test `go test ./internal/cmd -run TestDistributionGate -count=1`; harness `bash scripts/verify-distribution.sh`; rollback delete gate script + fixtures + ci.yml step.
- U3 release + README: test `go build ./... && go vet ./...`; harness manual `v*` tag push; rollback delete release.yml, revert README.md.

## Phase 1: Offline init E2E (D08 F-03)

- [x] 1.1 Create `internal/cmd/distribution_e2e_test.go` (verified: no binary-build/network-denial test exists): build `.` with `CGO_ENABLED=0` into `t.TempDir()/haro`; `internal/project/init.go` untouched.
- [x] 1.2 Run `haro init` in fresh `t.TempDir()` cwd with `GOPROXY=off`; assert exit 0 and `.haro/config.yaml` + `.haro/{workflows,skills,artifacts,docs}`. Spec: Offline clean-project initialization.
- [x] 1.3 Custom `.haro/config.yaml` bytes + `.haro/docs/user.md`; rerun with `GOPROXY=off`; assert exit 0 and preserved bytes. Spec: Offline initialization is idempotent.

## Phase 2: F-02 gate (RED → GREEN)

- [x] 2.1 RED fixtures in `distribution_e2e_test.go` calling `scripts/verify-distribution.sh <root>`: clean fixture → exit 0; fixture gaining `package.json` or `postinstall.sh` → nonzero (validator MINOR 1); `preinstall.sh`, `install.sh`, `npm-shrinkwrap.json` → nonzero; allow-list `requirements.txt`, `CMakeLists.txt`, executable `*.md`/MDX, `README.sh` pass. Fails pre-GREEN.
- [x] 2.2 GREEN `scripts/verify-distribution.sh` (+x), style of `verify-store-boundary.sh`: `set -euo pipefail`; optional root, default repo root; `git grep` + non-git fallback; reject exactly those five names; print hits, exit 1; else 0. D08 F-02.
- [x] 2.3 GREEN `.github/workflows/ci.yml` `build-and-test`: add `bash scripts/verify-distribution.sh`; keep triggers + `contents: read`. F-02 fail-closed.

## Phase 3: release.yml (D08 F-01)

- [x] 3.1 Create `.github/workflows/release.yml`: `on.push.tags: ['v*']`; workflow `permissions: contents: read`; only `actions/checkout@v7` + `actions/setup-go@v7` (as ci.yml).
- [x] 3.2 Job `verify`: gate + `go test ./... -race -count=1`; clean-`GOBIN` install `GOBIN=$(mktemp -d) GOPROXY=direct go install github.com/HectorCortes/haro@$GITHUB_REF_NAME`; executes in empty dir. F-01 clean-env proof (networked install stays out of ordinary CI).
- [x] 3.3 Job `release` (`needs: verify`; `permissions: contents: write`, sole grant): `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/haro .` from root `main.go`; `sha256sum dist/haro > dist/haro.sha256`; `gh release create "$GITHUB_REF_NAME" --verify-tag --generate-notes` + `gh release upload ... --clobber` (`GH_TOKEN`); local `file dist/haro` statically linked (no cgo). F-01.

## Phase 4: README installation (F-01/F-02 docs)

- [x] 4.1 `README.md`: remove stale "go.mod will be created in the spike phase" + "no publishable binary"; add Installation: `go install github.com/HectorCortes/haro@latest` and release asset → `chmod +x haro && ./haro init`. F-01 docs.
- [x] 4.2 Document: networked install NOT tested in ordinary CI (tag/proxy timing); release verifies exact tag via `GOPROXY=direct`; post-release `@latest` manual check.
- [x] 4.3 Guard: no `deltas-acceptance.md` F-01 edits; D08 maps 3/3 (TRACE-D08 at archive).

## Phase 5: Final gates

- [x] 5.1 All green: `go build ./...`; `go vet ./...`; `go test ./... -race -count=1`; `golangci-lint run`; `bash scripts/verify-distribution.sh`; `bash scripts/verify-store-boundary.sh` (no regression); `git status --porcelain` shows only intended files (no npm artifacts, no new deps).

## Non-Goals

No goreleaser, Homebrew, npm wrappers, doctor/setup/CLI registration, F-01 text edits, broker/PTY, new deps, multi-arch.
