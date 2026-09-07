```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:04893677e6336de1fc6fe77019664bbb83e36359dcb0f125a3e7bf4ed776e75b
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 4/4
scenarios: 7/7
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:2fc37e427ec8eda8f57b7cb2e6b54eb5de9021d6f1ce591fa42f72c1e61bc1e4
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: v2-distribucion (D08)
**Version**: N/A
**Mode**: Strict TDD
**Evidence revision**: `0767c6bdd6d4ef6f3c464170e2bff80239a32dd6`
**Verification date**: 2026-09-07

### Executive Summary

All four requirements have implementation evidence and the local runtime gates are green. The real-binary offline initialization and adversarial hook gate pass independently; the published `@latest` module also installs into a clean `GOBIN` and executes. The only warnings are the intentionally external post-tag release-asset check and the archive-time traceability record, neither of which can be exercised on this untagged local HEAD.

### Completeness

| Metric | Value |
|--------|-------|
| Requirements | 4/4 implemented/evidenced |
| Scenarios | 7/7 evidenced; 5 runtime-complete and 2 partial external/audit gates |
| Tasks total | 13 |
| Tasks complete | 13 |
| Tasks incomplete | 0 |

### Build & Tests Execution

| Check | Command | Exit | Output hash / result |
|-------|---------|------|---------------------|
| Build | `go build ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty output) |
| Vet | `go vet ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty output) |
| Race tests | `go test ./... -race -count=1` | 0 | `sha256:2fc37e427ec8eda8f57b7cb2e6b54eb5de9021d6f1ce591fa42f72c1e61bc1e4` |
| Distribution gate | `bash scripts/verify-distribution.sh` | 0 | `sha256:cc677d2a2f78b224823bb02b94bfcac03cb09a36b30893a253683e500936e6a1` |
| Store boundary regression gate | `bash scripts/verify-store-boundary.sh` | 0 | `sha256:9bfa19fcd9e9b3bdd3fefb3b5f2c173a911d551bdfd6a78ef4dc31d6eeaceaf3` |
| Linter | `golangci-lint run` | 0 | `0 issues`; `sha256:e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47` |
| Workflow YAML parse | `python3` with `yaml.safe_load` on `release.yml` and `ci.yml` | 0 | `sha256:3054566641d46c826948f5b4847d279f065af87ed6f4022644521db3b981ee9b` |
| Workflow policy audit | PyYAML `BaseLoader` structural assertions | 0 | CI contents read, release workflow contents read, release job contents write, `v*` trigger, no forbidden artifacts |

The full race run produced 16 package entries: 14 packages passed and 2 packages reported `[no test files]` (`github.com/HectorCortes/haro` and `internal/store/contract`). This is the current runtime result; the apply artifact's older “12 packages” wording is an under-count, not a failing gate.

### Focused Runtime Verification

| Scope | Command | Exit | Result |
|-------|---------|------|--------|
| Distribution E2E | `go test ./internal/cmd -run TestDistribution -count=1` | 0 | Two real-binary init tests plus all gate tests passed; hash `sha256:d2ee5810dcbd07b48d0f0a37f04998435c2521d2213b8b352da993344707364b` |
| F-02 gate | `go test ./internal/cmd -run TestDistributionGate -count=1` | 0 | Hash `sha256:69a3822d3617b34e67e84e4341825c28a44db9c3c069378d05a22667891c2a67` |
| Verbose focused run | `go test ./internal/cmd -run TestDistribution -count=1 -v` | 0 | 6 top-level tests passed; 11 gate fixture cases passed; hash `sha256:61e9b1aefa64230ed078b6b22b6a08c3f8fd5f4e64b68c804239826949406fcb` |
| Published module smoke | `GOBIN=$(mktemp -d) go install github.com/HectorCortes/haro@latest` followed by `haro init` in an empty temp directory | 0 | Clean install and execution passed; output hash `sha256:170e649cfbf819b4b3f2a7f763b0bd8f8c4ed0cd40b9f2e1abe04bb37a1e2467` |
| Static release build | `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o <tmp>/haro .` plus `file`, checksum, and offline `init` | 0 | Statically linked, stripped ELF; init passed; output hash `sha256:7fe4974d403ab57559be3eb77d4d149f85ce5357aa465e248c3b5690fabade1a` |

### Spec Compliance Matrix

| Requirement | Scenario | Test or executed evidence | Result |
|-------------|----------|---------------------------|--------|
| F-01 Go-native end-to-end installation | Clean module installation | `module-install-latest`: clean `GOBIN`, networked `go install github.com/HectorCortes/haro@latest`, executable invoked from an empty directory | ✅ COMPLIANT for the currently published module |
| F-01 Go-native end-to-end installation | Tagged release binary | Local release-equivalent static build passed; `.github/workflows/release.yml` contains exact-tag verify, static build, checksum, and upload steps, but no `v*` tag/release exists locally | ⚠️ PARTIAL — manual post-tag runtime check remains |
| F-02 No executable installation hooks | Hook-free distribution review | `TestDistributionGateCleanFixture`, `bash scripts/verify-distribution.sh`, and repository gate pass | ✅ COMPLIANT |
| F-02 No executable installation hooks | Package regression rejection | `TestDistributionGateRejectsPackageArtifacts` and `TestDistributionGateRejectsInstallHooks`; `package.json`, `npm-shrinkwrap.json`, `preinstall.sh`, `install.sh`, and `postinstall.sh` fixtures all rejected | ✅ COMPLIANT |
| F-03 Dependency-free offline initialization | Offline clean-project initialization | `TestDistributionOfflineInitE2E`: real `CGO_ENABLED=0` binary, empty temp project, `GOPROXY=off`, exit 0, all five `.haro` paths present | ✅ COMPLIANT |
| F-03 Dependency-free offline initialization | Offline initialization is idempotent | `TestDistributionOfflineInitIdempotentE2E`: second offline run preserves custom config bytes and `docs/user.md` | ✅ COMPLIANT |
| TRACE-D08 Constitutional interpretation and traceability | D08 traceability audit | Spec explicitly maps Go distribution to Constitution §III.1; F-01 text is unchanged; exact 1:1 F-01/F-02/F-03 mapping is documented. The archive record is created only by the next SDD phase | ⚠️ PARTIAL — archive-time audit remains |

**Compliance summary**: 7/7 scenarios have evidence; 5 are runtime-complete and 2 are documented partial gates that require the tag/release and archive lifecycle respectively. No scenario failed.

### D08 Traceability: Criterion → Requirement → Real Test

| D08 criterion | Spec requirement | Real test/evidence today | Status |
|---------------|------------------|--------------------------|--------|
| `v2-distribucion/F-01` | `v2-distribucion/F-01` Go-native end-to-end installation | Published `@latest` clean-GOBIN install and local static build pass; exact tagged release workflow is structurally verified | Covered with post-tag gap |
| `v2-distribucion/F-02` | `v2-distribucion/F-02` No executable installation hooks | `go test ./internal/cmd -run TestDistributionGate -count=1` with adversarial fixtures plus repository gate | Covered |
| `v2-distribucion/F-03` | `v2-distribucion/F-03` Dependency-free offline initialization | Real-binary `TestDistributionOfflineInitE2E` and `TestDistributionOfflineInitIdempotentE2E`, both with `GOPROXY=off` | Covered |

The historical D08 contract remains byte-for-byte unchanged: F-01 still says `pnpm add -g shardeo`, `dist/index.js`, and `prepublishOnly`; F-02 and F-03 remain unchanged. Constitution §III.1 states: “The distribution model is not tied to any particular ecosystem; it may change with respect to v1.” The spec's Go reinterpretation is therefore normative and traceable rather than a silent contract rewrite.

### F-01 Installation and Release Limitation

The `@latest` smoke test required network access and passed, but resolved the currently published pseudo-version `v0.0.0-20260907164447-5cb547699fc1`, while this checkout is at `0767c6b` and is six commits ahead of `origin/main`. There are no local or remote `v*` tags. Therefore it proves the published module path and clean execution mechanism, not publication of this untagged change.

The remaining manual post-tag proof is: push a `v*` tag; let `release.yml` run its exact-tag `GOPROXY=direct` clean-`GOBIN` install and empty-project execution; download `haro` and `haro.sha256` from the created release in a clean environment; verify the checksum and `file` reports static linkage; run `./haro init`; then repeat the documented `@latest` smoke test after proxy propagation. No push or release creation was performed during this verification.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| F-01 Go-native installation/release | ✅ Implemented | `go.mod` module path, README install/release paths, tag-only `release.yml`, clean exact-tag job, static binary and checksum steps. Runtime publication remains post-tag. |
| F-02 Hook-free distribution | ✅ Implemented | Exact-name fail-closed gate, non-git fixture fallback, CI invocation, and adversarial fixture coverage. |
| F-03 Offline/idempotent init | ✅ Implemented | Production `internal/project/init.go` is unchanged; the real binary tests prove offline structure creation and preservation. |
| TRACE-D08 | ✅ Implemented | Spec records §III.1 interpretation and 3/3 mapping; archive record is the next phase's output. |

### No-Regression and Scope Checks

| Check | Result |
|-------|--------|
| `internal/project/init.go` unchanged from pre-change revision `5cb5476` | ✅ |
| `go.mod` unchanged; no dependencies added | ✅ |
| `deltas-acceptance.md` unchanged; literal npm-v1 criteria preserved | ✅ |
| `docs/v2/` unchanged by this change | ✅ |
| 12-package regression intent | ✅ Superseded by actual run: 16 package entries, 14 passes, 2 no-test packages |
| `ci.yml` permissions | ✅ `contents: read`; no escalated permission |
| Release permissions | ✅ `contents: write` only on dependent `release` job; verify workflow inherits read |
| No GoReleaser, Homebrew, or npm wrapper artifacts | ✅ No implemented artifacts; the terms appear only in scope/design documentation |
| Pre-existing untracked files | ⚠️ `.atl/`, `.engram/`, and `testdata/compose/symlink-escape/.../link.yaml` remain untouched and are not part of this change |

### Design Coherence

| Decision | Followed? | Notes |
|----------|-----------|-------|
| Separate tag-only release workflow | ✅ Yes | `release.yml` triggers only on `v*`; ordinary CI remains read-only. |
| Go module plus one static GitHub Release asset | ✅ Yes | Plain Go build, stripped binary, SHA-256 checksum, no GoReleaser/Homebrew. |
| Exact-tag network proof only in release workflow | ✅ Yes | Ordinary CI uses offline binary E2E; release workflow uses `GOPROXY=direct` and exact tag. |
| Exact-name fail-closed distribution gate | ✅ Yes | Gate and adversarial fixtures pass. |
| Preserve F-01 wording and record the constitutional interpretation | ✅ Yes | Contract file is unchanged; spec contains the §III.1 interpretation. |

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | `apply-progress.md` contains a TDD Cycle Evidence table for all 13 tasks. |
| All tasks have verification evidence | ✅ | 13/13 tasks have current source, command, structural, or runtime evidence. |
| RED confirmed (test files exist) | ✅ with note | `internal/cmd/distribution_e2e_test.go` exists and its six top-level tests pass; F-03 is explicitly documented as an approval test because production init was unchanged, while config/docs tasks are structural. |
| GREEN confirmed | ✅ | Focused and full runtime commands pass; YAML, lint, and boundary checks pass. |
| Triangulation adequate | ✅ | Two F-03 scenarios, adversarial package/hook cases, and five allow-list files provide behavioral variance; structural tasks are marked N/A. |
| Safety net | ✅ with note | Apply evidence records the baseline run for behavioral files; structural new files correctly use N/A. |

**TDD Compliance**: 6/6 verification dimensions passed, with the documented approval-test and structural-task exceptions rather than missing tests.

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | 0 | 0 | — |
| Integration | 4 top-level gate tests / 11 fixture cases | 1 | Go `testing`, `os/exec`, Bash gate |
| E2E | 2 real-binary tests | 1 | Go `testing`, `go build`, `GOPROXY=off` |
| **Total** | **6 top-level tests / 13 behavioral executions including init and fixtures** | **1** | |

### Changed File Coverage

Coverage was executed with `go test ./... -race -count=1 -coverprofile=<temporary path>` and passed with 64.0% repository statement coverage. No production Go file was changed by D08: the changed Go file is an E2E test, while the other changed files are shell, YAML, Markdown, and SDD artifacts. Consequently, changed-production-file coverage is not applicable; no changed production file is below a threshold.

### Assertion Quality

**Assertion quality**: ✅ All assertions verify real behavior. The audit found no tautologies, orphan empty assertions, ghost loops, type-only assertions used alone, smoke-only checks, or mock-heavy tests. The path loop iterates over five explicit required paths, and the allow-list loop iterates over five explicit fixture files.

### Quality Metrics

**Linter**: ✅ `golangci-lint run` returned `0 issues`.
**Type checker**: ✅ `go vet ./...` returned exit 0.
**Workflow parser**: ✅ Both workflow files parsed. Structural checks used PyYAML `BaseLoader` because YAML 1.1 `safe_load` resolves the GitHub Actions key `on` as a boolean; this is a parser compatibility detail, not a workflow error.

### Issues Found

**CRITICAL**: None.

**WARNING**:

1. The current checkout has no `v*` tag or release; the current HEAD's exact-tag install and downloaded release asset remain a manual post-tag verification. The existing published `@latest` smoke test passed but resolved the older published pseudo-version.
2. `apply-progress.md` understates the current full-suite result as 12 packages and describes the gate as 8 cases; the independent run observed 16 package entries (14 pass, 2 no-test) and 11 gate fixture cases. This is evidence-document drift only.
3. `ci.yml` retains an obsolete comment saying `go.mod` does not exist, although the module is now present. It has no runtime effect.
4. The archive record required by TRACE-D08 does not exist before the archive phase; the spec and literal contract mapping are ready for that phase.

**SUGGESTION**:

1. After tagging, record the release-asset checksum and the post-proxy `@latest` result in the archived change evidence.

### Verdict

**PASS WITH WARNINGS** — all local build, vet, race, lint, distribution, store-boundary, YAML, focused F-02, and real-binary offline gates passed; only the explicitly external post-tag publication proof and archive-time traceability check remain.
