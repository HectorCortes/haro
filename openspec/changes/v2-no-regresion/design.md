# Design: Preservation of Specs 1–9 — Go re-expression

## Technical approach

Implement the command-only slice as CLI → execution service → repositories. Every mutation revalidates the workflow and atomically commits state, generation, aggregate status, and audit.

    main → internal/cmd → internal/execution → store.Store → SQLite
                              ├→ CommandRunner → child process
                              └→ contained .haro/{artifacts,evidence}

## Architecture decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| CLI-direct or broker | Direct is temporary | CLI-direct; broker/JSON-RPC belongs to v2-broker/v2-ipc. |
| Shell or argv | `run` needs tokenization | Use a quoted-argv lexer and `exec.CommandContext(argv[0], argv[1:]...)`; never shell expansion. |
| SQLite leakage or repositories | More interfaces | `Store` exposes `Projects`, `Executions`, `Steps`, `Attempts`, `Generations`, `Events`, and `WithTx`; only `internal/store` imports `database/sql`. |
| Memory or files | File tests are slower | Use `.haro/store.db`; tests use isolated files under `t.TempDir()`, proving WAL. |
| Subprocess E2E or callable CLI | Callable misses process startup | Thin `main()` calls `cmd.Execute(ctx,args,cwd,out,errOut) int`; tests inject dependencies and writers. |

## Contracts and behavior

Each leaf owns `flag.NewFlagSet(name, flag.ContinueOnError)` with output discarded. Routers consume `workflows|steps|step` and the leaf name. Leaves consume required operands first, then parse the option tail: `init`; `workflows list [--json]`; `workflows describe NAME [--json]`; `run WORKFLOW [--json]`; `steps next EXEC [--json]`; `step run EXEC STEP [--feedback TEXT] [--json]`; `step reopen EXEC STEP --cascade [--feedback TEXT]`; `step skip EXEC STEP --reason TEXT`; `status EXEC [--json]`. `--` terminates option parsing; any remaining positional is rejected as `unexpected_argument`, not reinterpreted as a flag. JSON success uses command-specific objects; every error is `{"error":string,"code":string}`/1. `syscall.EPIPE` while writing is success/0.

Workflow structs cover `version`, `name`, and steps `{id,type,run,env,timeout_seconds,depends_on,requires,produces}`; agent/workflow fields parse but execution fails `unsupported_step_type`. Discovery scans `.haro/workflows/*/workflow.yaml` deterministically. Validation reports invalid YAML, duplicate IDs, missing dependencies, cycle paths, and no entry point. Containment rejects absolute/`..` paths and symlink escapes after `EvalSymlinks`, permitting internal targets.

`CommandRunner.Run(ctx,cwd,argv,env)` returns exit code/stdout/stderr with a 300-second ceiling. The engine checks completed/skipped dependencies and valid-generation `requires`, transitions to running, snapshots changed `produces`, then completes only on exit 0 with all outputs. Visible evidence is redacted then bounded to 16 KiB; snapshots are ≤1 MiB; prior/fallback reconstruction is ≤2 MiB. Redaction covers Bearer, Basic, and token/key assignments. Delimited feedback creates a reconstruction attempt with bounded prior evidence.

Migrations execute once per connection with `foreign_keys=ON`, `journal_mode=WAL`, and idempotent `CREATE TABLE/INDEX IF NOT EXISTS`. The seven §2 tables are retained: `projects(id,root_path,created_at)`; `executions(...,status CHECK pending|running|completed|failed,workspace_mode CHECK isolated|shared,...)`; `execution_steps(...,type/status CHECKs,current_generation)`; `generations(...,number,invalidated_at,invalidated_by_step,UNIQUE step+number)`; `attempts(...,status CHECK running|completed|failed|cancelled,termination_reason,result_digest)`; `attempt_events(...,cursor,event_type,payload_ref,UNIQUE attempt+cursor)`; and `step_transition_events(...,cursor,from_status,to_status,occurred_at,UNIQUE step+cursor)`. Timestamps are UTC RFC3339 text. Evidence/feedback/manifests are contained files referenced by `payload_ref`.

Transitions are idempotent for pending→running→completed|failed, completed|failed→pending, and pending→skipped; repeats add no event. Reopen atomically invalidates generations, resets descendants, preserves files/attempts, and enforces cascade. Skip requires a reason and virgin pending state. Both audit and resynchronize execution status.

## File changes

| Files | Action | Purpose |
|---|---|---|
| `main.go`; `internal/cmd/{execute,output}.go` | Modify/Create | Routing, flags, JSON/EPIPE. |
| `internal/project/init.go` | Create | Non-destructive initialization. |
| `internal/workflow/{parse,discover,validate,path}.go` | Modify/Create | Schema, discovery, DAG, containment. |
| `internal/store/{store,sqlite,migrations,repositories}.go` | Create | Interfaces, DDL, transactions. |
| `internal/execution/{engine,runner,evidence,state}.go` | Create | Command lifecycle and iteration. |
| Matching `*_test.go`; `testdata/workflows/` | Create/Modify | RED-first fixtures and proofs. |
| `testdata/opencode/v1.17.18/`, `v1.18.16/` | Create | Byte-for-byte imports from the v1 fixture source plus a SHA-256 identity test; originals are absent locally, so U-02 cannot pass until acquired unchanged. |

## Testing and verification

Table-driven tests cover argv, DAG errors, containment, budgets, and transition repetition/forbidden states. Integrations use `t.TempDir()`, file SQLite, fake `CommandRunner`, four command outcomes, feedback, cascade, stale files, and skip audit. CLI tests call `Execute`; a closed `os.Pipe` proves EPIPE. Run `go test ./...` each RED/GREEN cycle, then `go test ./... -race`, `go vet ./...`, `golangci-lint run`, and `govulncheck ./...`.

## Threat matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | No executable classification; only explicit `run` argv executes. |
| Git repository selection | N/A | No Git invocation. |
| Commit state | N/A | No commit automation. |
| Push state | N/A | No push automation. |
| PR commands | N/A | No PR automation. |

Process RED tests nevertheless prove no shell metacharacter expansion, exact argv/cwd, timeout, start failure, and bounded capture.

## Rollout, risks, and exclusions

No migration or feature flag; rollback reverts code and removes newly created project-local state. `openspec/config.yaml` and `.gitignore` remain unchanged (coverage is already ignored). Main risks are fixture provenance, stale-generation correctness, and transaction breadth.

Excluded: agent steps/fallback/permissions, supervised, bundle/admission, resume, PTY, path claims beyond containment, composition, reporting, distribution, full DDL/backends, agent `DiagnosticRaw`, broker/socket/JSON-RPC. Open questions: none; unchanged v1 fixture acquisition is an apply gate.
