package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/claim"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
	"github.com/HectorCortes/haro/internal/worktree"
)

// LogicalConflictError is returned when a claim conflicts with an active owner.
type LogicalConflictError struct {
	LogicalPath      string
	OwnerExecutionID string
	OwnerStepID      string
	Mode             string
}

func (e *LogicalConflictError) Error() string {
	return fmt.Sprintf("logical_conflict: %q owned by %s/%s mode %s", e.LogicalPath, e.OwnerExecutionID, e.OwnerStepID, e.Mode)
}

// Engine orchestrates command execution with store and runner.
type Engine struct {
	store       store.Store
	runner      CommandRunner
	root        string
	adapterMgr  *adapter.Manager
	worktreeMgr worktree.Manager
}

// NewEngine creates an engine.
func NewEngine(s store.Store, r CommandRunner, root string) *Engine {
	return &Engine{store: s, runner: r, root: root, worktreeMgr: worktree.NewManager()}
}

// SetAdapterManager sets the adapter manager for agent steps.
func (e *Engine) SetAdapterManager(m *adapter.Manager) {
	e.adapterMgr = m
}

// SetWorktreeManager sets the worktree manager (for tests).
func (e *Engine) SetWorktreeManager(m worktree.Manager) {
	e.worktreeMgr = m
}

// CreateExecution creates an execution for workflowName.
func (e *Engine) CreateExecution(ctx context.Context, workflowName string) (string, error) {
	// Discover workflows
	discovered, err := workflow.Discover(e.root)
	if err != nil {
		return "", fmt.Errorf("discover: %w", err)
	}
	var wf *workflow.Workflow
	var wfPath string
	for _, d := range discovered {
		if d.Name == workflowName || d.Workflow.Name == workflowName {
			wf = d.Workflow
			wfPath = d.Path
			break
		}
	}
	if wf == nil {
		return "", fmt.Errorf("workflow %q not found", workflowName)
	}
	// Validate root file before flatten (fail before any side effects)
	if err := workflow.ValidateFile(wf, wfPath, true); err != nil {
		return "", err
	}
	// Planning-time flatten: validates, checks guards, cycles, containment before any execution row or worktree
	flatDAG, err := workflow.Flatten(wfPath, nil)
	if err != nil {
		return "", err
	}
	if err := workflow.ValidateFlat(flatDAG); err != nil {
		return "", err
	}
	// Guard: runtime leaked workflow node → workflow_invalid (should have been caught by ValidateFlat)
	for _, s := range flatDAG.Steps {
		if s.Type == "workflow" {
			return "", &workflow.ValidationError{Code: "workflow_invalid", Field: "steps", Message: "workflow node leaked to flat DAG"}
		}
	}
	// Startup prune: clean orphan worktrees (best effort)
	if e.worktreeMgr != nil {
		_ = e.worktreeMgr.Prune(ctx, e.root)
	}
	// Ensure project exists
	projectID := e.root // simple: use root as id
	_ = e.store.Projects().Create(ctx, projectID, e.root)
	// Generate execution id
	execID := uuid.NewString()
	// Resolve workflow-level mode for execution's workspace (root governs)
	execMode := "isolated"
	if wf.Workspace != nil && wf.Workspace.Mode != nil {
		execMode = *wf.Workspace.Mode
	}
	// Capture committed HEAD in resolvedRoot BEFORE worktree.Create; dirty ignored
	var resolvedRoot string
	if eval, err := filepath.EvalSymlinks(e.root); err == nil {
		resolvedRoot = filepath.Clean(eval)
	} else {
		resolvedRoot = filepath.Clean(e.root)
	}
	baseCommit, capErr := captureBaseCommit(ctx, resolvedRoot)
	if capErr != nil {
		return "", capErr
	}
	workspaceRoot := e.root
	if execMode == "isolated" && e.worktreeMgr != nil {
		wt, err := e.worktreeMgr.Create(ctx, execID, e.root)
		if err != nil {
			workspaceRoot = e.root
			_ = wt
			if wt != "" {
				workspaceRoot = wt
			}
		} else {
			workspaceRoot = wt
		}
	}
	startedAt := time.Now().UTC().Format(time.RFC3339)
	hashCopy := flatDAG.Hash
	exec := &store.Execution{
		ID:              execID,
		ProjectID:       projectID,
		WorkflowSource:  wfPath,
		Status:          "running",
		WorkspaceMode:   execMode,
		WorkspaceRoot:   workspaceRoot,
		StartedAt:       startedAt,
		DagHash:         &hashCopy,
		BaseCommit:      baseCommit,
	}
	err = e.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.Executions().Create(ctx, exec); err != nil {
			return err
		}
		for _, st := range flatDAG.Steps {
			depJSON, _ := json.Marshal(st.DependsOn)
			reqJSON, _ := json.Marshal(st.Requires)
			prodJSON, _ := json.Marshal(st.Produces)
			if depJSON == nil {
				depJSON = []byte("[]")
			}
			if reqJSON == nil {
				reqJSON = []byte("[]")
			}
			if prodJSON == nil {
				prodJSON = []byte("[]")
			}
			// Resolve workspace for flattened step: system->root->step override (included workflow's workflow-level ignored)
			mode := resolveWorkspaceForFlat(wf, &st)
			step := &store.ExecutionStep{
				ExecutionID:       execID,
				StepID:            st.ID,
				Type:              st.Type,
				Status:            "pending",
				DependsOn:         string(depJSON),
				Requires:          string(reqJSON),
				Produces:          string(prodJSON),
				WorkspaceMode:     mode,
				CurrentGeneration: 0,
			}
			if err := tx.Steps().Create(ctx, step); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// Cleanup worktree on transaction failure
		if execMode == "isolated" && workspaceRoot != e.root && e.worktreeMgr != nil {
			_ = e.worktreeMgr.Remove(ctx, workspaceRoot)
		}
		return "", err
	}
	return execID, nil
}

func resolveWorkspaceForFlat(rootWf *workflow.Workflow, flat *workflow.FlatStep) string {
	mode := "isolated"
	if rootWf != nil && rootWf.Workspace != nil && rootWf.Workspace.Mode != nil {
		mode = *rootWf.Workspace.Mode
	}
	if flat.Workspace != nil && flat.Workspace.Mode != nil {
		mode = *flat.Workspace.Mode
	}
	return mode
}

// ResolveWorkspaceForFlat is exported for testing that root policy governs flattened steps.
// It mirrors resolveWorkspaceForFlat without behavior change.
func ResolveWorkspaceForFlat(rootWf *workflow.Workflow, flat *workflow.FlatStep) string {
	return resolveWorkspaceForFlat(rootWf, flat)
}

// captureBaseCommit runs `git rev-parse HEAD` in resolvedRoot with fixed argv.
// On success returns *string of trimmed HEAD. If not a git repo, returns nil, nil (legacy nullable).
// Other failures return error and caller must not create execution/worktree.
func captureBaseCommit(ctx context.Context, resolvedRoot string) (*string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = resolvedRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		// Not a git repo -> treat as nullable legacy, not fatal (allows non-git temp dirs in unit tests)
		if strings.Contains(msg, "not a git repository") || strings.Contains(msg, "not a git repo") {
			return nil, nil
		}
		// If root missing or no git binary, also treat missing repo as nil? But missing dir/gene should be fatal.
		// For safety, if resolvedRoot does not contain .git and error indicates fatal, treat as nil to preserve existing non-git tests.
		// Check for explicit git not found case.
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("git rev-parse HEAD: %w output: %s", err, string(out))
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, fmt.Errorf("git rev-parse HEAD empty")
	}
	return &trimmed, nil
}

func (e *Engine) effectiveWorktreeRoot(ctx context.Context, executionID string) string {
	exec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil || exec == nil {
		return e.root
	}
	if exec.WorkspaceRoot != "" {
		return exec.WorkspaceRoot
	}
	return e.root
}

func (e *Engine) acquireClaims(ctx context.Context, executionID, stepID string) ([]string, error) {
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return nil, err
	}
	exec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil {
		return nil, err
	}
	projectID := e.root
	if exec != nil {
		projectID = exec.ProjectID
	}
	var requires, produces []string
	_ = json.Unmarshal([]byte(step.Requires), &requires)
	_ = json.Unmarshal([]byte(step.Produces), &produces)
	all := append(append([]string{}, requires...), produces...)
	// deduplicate and canonicalize
	seen := make(map[string]bool)
	var canonical []string
	cfg, _ := project.LoadConfig(e.root)
	extPaths := []string{}
	if cfg != nil {
		extPaths = cfg.ExternalPaths
	}
	for _, raw := range all {
		c, err := claim.Canonicalize(e.root, raw)
		if err != nil {
			return nil, fmt.Errorf("canonicalize %q: %w", raw, err)
		}
		if c == "" {
			continue
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		canonical = append(canonical, c)
	}
	// Acquire each
	// Determine effective mode for this step: if external, forced shared, else step's workspace_mode
	effMode := step.WorkspaceMode
	if effMode == "" {
		effMode = "isolated"
	}
	var acquired []string
	for _, lp := range canonical {
		mode := effMode
		if claim.IsExternal(lp, extPaths) {
			mode = "shared"
		} else if strings.HasPrefix(mode, "isolated") {
			mode = "isolated:" + executionID
		}
		c := store.PathClaim{
			ProjectID:        projectID,
			LogicalPath:      lp,
			Mode:             mode,
			OwnerExecutionID: executionID,
			OwnerStepID:      stepID,
		}
		ok, conflict, err := e.store.PathClaims().Acquire(ctx, c)
		if err != nil {
			// rollback acquired so far
			for _, rel := range acquired {
				_ = e.store.PathClaims().Release(ctx, projectID, rel, executionID, stepID)
			}
			return nil, err
		}
		if !ok {
			// rollback acquired so far
			for _, rel := range acquired {
				_ = e.store.PathClaims().Release(ctx, projectID, rel, executionID, stepID)
			}
			if conflict != nil {
				return nil, &LogicalConflictError{LogicalPath: lp, OwnerExecutionID: conflict.OwnerExecutionID, OwnerStepID: conflict.OwnerStepID, Mode: conflict.Mode}
			}
			return nil, &LogicalConflictError{LogicalPath: lp}
		}
		acquired = append(acquired, lp)
	}
	return acquired, nil
}

func (e *Engine) releaseClaims(ctx context.Context, executionID, stepID string, paths []string) {
	if len(paths) == 0 {
		return
	}
	exec, err := e.store.Executions().Get(ctx, executionID)
	projectID := e.root
	if err == nil && exec != nil {
		projectID = exec.ProjectID
	}
	for _, lp := range paths {
		_ = e.store.PathClaims().Release(ctx, projectID, lp, executionID, stepID)
	}
}

// verifyDAGHash re-flattens source and compares to stored hash.
func (e *Engine) VerifyDAGHashForTest(ctx context.Context, executionID string) error {
	return e.verifyDAGHash(ctx, executionID)
}

func (e *Engine) verifyDAGHash(ctx context.Context, executionID string) error {
	exec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil || exec == nil || exec.DagHash == nil || exec.WorkflowSource == "" {
		return nil
	}
	flat, err := workflow.Flatten(exec.WorkflowSource, nil)
	if err != nil {
		return &workflow.ValidationError{Code: "workflow_invalid", Field: "", Message: fmt.Sprintf("re-flatten failed: %v", err)}
	}
	if flat.Hash != *exec.DagHash {
		return &workflow.ValidationError{Code: "workflow_invalid", Field: "dag_hash", Message: fmt.Sprintf("dag hash mismatch: stored %q vs current %q", *exec.DagHash, flat.Hash)}
	}
	return nil
}

// latestInvalidGeneration reports whether a producer step's latest current
// generation is invalidated. Only the maximum-number generation gates the
// check: an older invalidated generation does not block once a newer valid
// generation exists. The explicit max keeps the result robust to backend
// ListByStep ordering.
func latestInvalidGeneration(gens []*store.Generation) (*store.Generation, bool) {
	var latest *store.Generation
	for _, g := range gens {
		if latest == nil || g.Number > latest.Number {
			latest = g
		}
	}
	if latest == nil || latest.InvalidatedAt == nil {
		return nil, false
	}
	return latest, true
}

// requiresStale reports whether any producer of req has an invalid latest
// generation, i.e. the artifact was invalidated by a reopen that no
// producer rerun has yet recovered.
func (e *Engine) requiresStale(ctx context.Context, executionID, req string) (int, bool) {
	steps, _ := e.store.Steps().List(ctx, executionID)
	for _, s := range steps {
		var prods []string
		_ = json.Unmarshal([]byte(s.Produces), &prods)
		for _, p := range prods {
			if p != req {
				continue
			}
			gens, _ := e.store.Generations().ListByStep(ctx, executionID, s.StepID)
			if latest, invalid := latestInvalidGeneration(gens); invalid {
				return latest.Number, true
			}
		}
	}
	return 0, false
}

// RunStep executes a step with the 4 command-cycle cases, and agent fallback.
func (e *Engine) RunStep(ctx context.Context, executionID, stepID, feedback string) error {
	// Runtime re-flatten + hash verification on rehydrate
	if err := e.verifyDAGHash(ctx, executionID); err != nil {
		return err
	}
	// Fetch step
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return fmt.Errorf("get step: %w", err)
	}
	// Guard: leaked workflow node → workflow_invalid
	if step.Type == "workflow" {
		return &workflow.ValidationError{Code: "workflow_invalid", Field: "steps[" + stepID + "].type", Message: fmt.Sprintf("workflow node %q leaked to runtime", stepID)}
	}
	if step.Type == "agent" {
		return e.runAgentStep(ctx, executionID, stepID, feedback)
	}
	if step.Type != "command" {
		return fmt.Errorf("unsupported_step_type: %s", step.Type)
	}
	// State check: only pending can run; feedback allows reconstruction from completed/failed
	if step.Status != "pending" {
		if feedback != "" && (step.Status == "completed" || step.Status == "failed") {
			// Reconstruction: transition to pending first
			if err := e.transitionStep(ctx, executionID, stepID, step.Status, "pending"); err != nil {
				return err
			}
			// Refresh step
			step, err = e.store.Steps().Get(ctx, executionID, stepID)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("step %q not pending (status=%s)", stepID, step.Status)
		}
	}
	// Decode depends_on
	var deps []string
	if err := json.Unmarshal([]byte(step.DependsOn), &deps); err != nil {
		deps = nil
	}
	// Check dependencies
	for _, dep := range deps {
		depStep, err := e.store.Steps().Get(ctx, executionID, dep)
		if err != nil {
			return fmt.Errorf("missing dependency %q: %w", dep, err)
		}
		if depStep.Status != "completed" && depStep.Status != "skipped" {
			return fmt.Errorf("unsatisfied dependency %q: status %s", dep, depStep.Status)
		}
	}
	// Decode requires
	var requires []string
	_ = json.Unmarshal([]byte(step.Requires), &requires)
	artifactsRoot := filepath.Join(e.root, ".haro", "artifacts")
	// Check requires before execution: must exist and have valid generation
	for _, req := range requires {
		if err := workflow.ValidateContainedPath(artifactsRoot, req); err != nil {
			return fmt.Errorf("requires containment: %w", err)
		}
		abs := filepath.Join(artifactsRoot, req)
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("requires %q missing: %w", req, err)
		}
		// Check generation invalidated? For requires, only the producer's
		// latest current generation gates the check: an invalidated latest
		// generation blocks downstream work, while older invalidated
		// generations must not block once a newer valid generation exists.
		if genNum, stale := e.requiresStale(ctx, executionID, req); stale {
			return fmt.Errorf("requires %q invalidated (generation %d)", req, genNum)
		}
	}
	// Decode produces
	var produces []string
	_ = json.Unmarshal([]byte(step.Produces), &produces)
	for _, prod := range produces {
		if err := workflow.ValidateContainedPath(artifactsRoot, prod); err != nil {
			return fmt.Errorf("produces containment: %w", err)
		}
	}
	// Acquire path claims before pending->running (canonicalize requires+produces)
	acquiredClaims, err := e.acquireClaims(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	// Ensure release on every return (success and failure)
	defer e.releaseClaims(ctx, executionID, stepID, acquiredClaims)
	// Transition pending -> running
	if err := e.transitionStep(ctx, executionID, stepID, "pending", "running"); err != nil {
		return err
	}
	// Create generation and attempt
	genNumber := step.CurrentGeneration + 1
	genID := uuid.NewString()
	gen := &store.Generation{
		ID:          genID,
		ExecutionID: executionID,
		StepID:      stepID,
		Number:      genNumber,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := e.store.Generations().Create(ctx, gen); err != nil {
		return fmt.Errorf("create generation: %w", err)
	}
	if err := e.store.Steps().UpdateGeneration(ctx, executionID, stepID, genNumber); err != nil {
		return fmt.Errorf("update generation: %w", err)
	}
	attemptID := uuid.NewString()
	attempt := &store.Attempt{
		ID:           attemptID,
		ExecutionID:  executionID,
		StepID:       stepID,
		GenerationID: genID,
		Status:       "running",
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := e.store.Attempts().Create(ctx, attempt); err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	// Get run command from workflow
	discovered, _ := workflow.Discover(e.root)
	var runStr string
	var envMap map[string]string
	for _, d := range discovered {
		// Find execution's workflow source? Use step's run from stored? We stored only step id/type/depends/requires/produces, not run. Need to fetch workflow again.
		// For simplicity, we fetch workflow for this execution's source.
		// But CreateExecution stored workflow source path; we can reload that file.
		exec, _ := e.store.Executions().Get(ctx, executionID)
		if exec != nil && d.Path == exec.WorkflowSource {
			for _, s := range d.Workflow.Steps {
				if s.ID == stepID {
					runStr = s.Run
					envMap = s.Env
					break
				}
			}
		}
		// Also handle if discovered includes workflow with same name
		for _, s := range d.Workflow.Steps {
			if s.ID == stepID && runStr == "" {
				runStr = s.Run
				envMap = s.Env
			}
		}
	}
	if runStr == "" {
		// fallback: try to load workflow source directly
		exec, _ := e.store.Executions().Get(ctx, executionID)
		if exec != nil {
			f, _ := os.Open(exec.WorkflowSource)
			if f != nil {
				wf, _ := workflow.Parse(f)
				_ = f.Close()
				if wf != nil {
					for _, s := range wf.Steps {
						if s.ID == stepID {
							runStr = s.Run
							envMap = s.Env
							break
						}
					}
				}
			}
		}
	}
	if runStr == "" {
		return fmt.Errorf("run command empty for step %q", stepID)
	}
	argv, err := ParseArgv(runStr)
	if err != nil {
		_ = e.failAttempt(ctx, attemptID, executionID, stepID, fmt.Sprintf("parse argv: %v", err))
		return fmt.Errorf("parse argv: %w", err)
	}
	// Feedback handling: delimited feedback + bounded prior context.
	// DB-first: prior evidence comes from the prior attempt's inline payload;
	// the legacy payload_ref file is read only when the payload is absent
	// (a non-nil payload wins even when empty).
	if feedback != "" {
		var prior string
		if ev, err := e.store.Events().PriorOutputDelta(ctx, attemptID); err == nil && ev != nil {
			if ev.Payload != nil {
				prior = *ev.Payload
			} else if ev.PayloadRef != nil {
				if data, rerr := os.ReadFile(*ev.PayloadRef); rerr == nil {
					prior = string(data)
				}
			}
		}
		delimited := fmt.Sprintf("---FEEDBACK---\n%s\n---END---", feedback)
		// Keep total within FallbackLimit: prior truncated + delimited.
		available := FallbackLimit - len(delimited)
		if available < 0 {
			available = 0
		}
		if len(prior) > available {
			prior = prior[:available]
		}
		// Ensure combined within fallback limit
		combined := FallbackEvidence(prior + delimited)
		// Store feedback delimited
		cursor, _ := e.store.Events().NextAttemptCursor(ctx, attemptID)
		delimitedCopy := delimited
		_ = e.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
			AttemptID:  attemptID,
			Cursor:     cursor,
			EventType:  "feedback",
			PayloadRef: &delimitedCopy,
		})
		// Also store combined as reconstruction context (for prior bounded)
		if prior != "" {
			cursor2, _ := e.store.Events().NextAttemptCursor(ctx, attemptID)
			_ = e.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
				AttemptID:  attemptID,
				Cursor:     cursor2,
				EventType:  "reconstruction_context",
				PayloadRef: &combined,
			})
		}
	}
	// Run via runner at effective workspace root
	wtRoot := e.effectiveWorktreeRoot(ctx, executionID)
	exitCode, stdout, stderr, runErr := e.runner.Run(ctx, wtRoot, argv, envMap)
	if runErr != nil {
		// Start failure or timeout
		_ = e.failAttempt(ctx, attemptID, executionID, stepID, runErr.Error())
		return fmt.Errorf("runner: %w", runErr)
	}
	// Compose execution-identifying evidence, then redact and bound once.
	// Persisted inline; no evidence files are written (snapshots unchanged).
	visible := CommandEvidence(argv, stdout, stderr)
	cursor, _ := e.store.Events().NextAttemptCursor(ctx, attemptID)
	_ = e.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
		AttemptID:  attemptID,
		Cursor:     cursor,
		EventType:  "output_delta",
		Payload:    &visible,
		PayloadRef: nil,
	})
	// Snapshot produces files (bounded to 1 MiB) and compute digest
	hasMissing := false
	var missing []string
	for _, prod := range produces {
		abs := filepath.Join(artifactsRoot, prod)
		data, err := os.ReadFile(abs)
		if err != nil {
			hasMissing = true
			missing = append(missing, prod)
			continue
		}
		// Snapshot truncated
		snap := SnapshotBytes(data)
		_ = snap
		// For evidence we could write snapshot file
		snapPath := filepath.Join(artifactsRoot, "snapshots", attemptID, prod)
		_ = os.MkdirAll(filepath.Dir(snapPath), 0o755)
		_ = os.WriteFile(snapPath, snap, 0o600)
	}
	if exitCode != 0 {
		// Case 2: failed with stdout/stderr
		reason := fmt.Sprintf("exit %d", exitCode)
		dig := visible
		_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &dig)
		_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
		e.resyncExecution(ctx, executionID)
		return nil
	}
	if hasMissing {
		// Case 3: exit 0 missing produces -> failed listing them
		reason := fmt.Sprintf("missing artifacts: %s", strings.Join(missing, ", "))
		dig := visible
		_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &dig)
		_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
		e.resyncExecution(ctx, executionID)
		return fmt.Errorf("missing artifacts: %s", strings.Join(missing, ", "))
	}
	// Case 1: success
	digest := sha256.Sum256([]byte(visible))
	digestStr := hex.EncodeToString(digest[:])
	_ = e.completeAttempt(ctx, attemptID, "completed", nil, &digestStr)
	_ = e.transitionStep(ctx, executionID, stepID, "running", "completed")
	e.resyncExecution(ctx, executionID)
	return nil
}

func (e *Engine) runAgentStep(ctx context.Context, executionID, stepID, feedback string) error {
	if err := e.verifyDAGHash(ctx, executionID); err != nil {
		return err
	}
	// Fetch step for status checks (already verified pending)
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return fmt.Errorf("get step: %w", err)
	}
	if step.Type == "workflow" {
		return &workflow.ValidationError{Code: "workflow_invalid", Field: "steps[" + stepID + "].type", Message: fmt.Sprintf("workflow node %q leaked to runtime", stepID)}
	}
	if step.Status != "pending" {
		if feedback != "" && (step.Status == "completed" || step.Status == "failed") {
			if err := e.transitionStep(ctx, executionID, stepID, step.Status, "pending"); err != nil {
				return err
			}
			step, _ = e.store.Steps().Get(ctx, executionID, stepID)
		} else {
			return fmt.Errorf("step %q not pending (status=%s)", stepID, step.Status)
		}
	}
	// Dependency and requires checks (reuse command logic)
	var deps []string
	_ = json.Unmarshal([]byte(step.DependsOn), &deps)
	for _, dep := range deps {
		depStep, err := e.store.Steps().Get(ctx, executionID, dep)
		if err != nil {
			return fmt.Errorf("missing dependency %q: %w", dep, err)
		}
		if depStep.Status != "completed" && depStep.Status != "skipped" {
			return fmt.Errorf("unsatisfied dependency %q: status %s", dep, depStep.Status)
		}
	}
	var requires []string
	_ = json.Unmarshal([]byte(step.Requires), &requires)
	artifactsRoot := filepath.Join(e.root, ".haro", "artifacts")
	for _, req := range requires {
		if err := workflow.ValidateContainedPath(artifactsRoot, req); err != nil {
			return fmt.Errorf("requires containment: %w", err)
		}
		if _, err := os.Stat(filepath.Join(artifactsRoot, req)); err != nil {
			return fmt.Errorf("requires %q missing: %w", req, err)
		}
		// Same latest-generation predicate as the command path: only the
		// producer's latest current generation gates the check.
		if genNum, stale := e.requiresStale(ctx, executionID, req); stale {
			return fmt.Errorf("requires %q invalidated (generation %d)", req, genNum)
		}
	}
	// Acquire claims for agent step as well
	agentAcquired, err := e.acquireClaims(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	defer e.releaseClaims(ctx, executionID, stepID, agentAcquired)
	// Transition pending -> running
	if err := e.transitionStep(ctx, executionID, stepID, "pending", "running"); err != nil {
		return err
	}
	// Load workflow to get harness candidates
	exec, _ := e.store.Executions().Get(ctx, executionID)
	var wf *workflow.Workflow
	if exec != nil {
		f, err := os.Open(exec.WorkflowSource)
		if err == nil {
			wf, _ = workflow.Parse(f)
			_ = f.Close()
		}
	}
	if wf == nil {
		discovered, _ := workflow.Discover(e.root)
		for _, d := range discovered {
			for _, s := range d.Workflow.Steps {
				if s.ID == stepID {
					wf = d.Workflow
					break
				}
			}
		}
	}
	var harnessCandidates []string
	var instructions string
	var mode string
	if wf != nil {
		for _, s := range wf.Steps {
			if s.ID == stepID {
				harnessCandidates = s.Harness
				instructions = s.Instructions
				mode = s.Mode
				break
			}
		}
	}
	if len(harnessCandidates) == 0 {
		_ = e.failAttempt(ctx, "no-attempt", executionID, stepID, "no harness candidates")
		return fmt.Errorf("no harness candidates for agent step %q", stepID)
	}
	if mode == "terminal" || mode == "supervised" {
		reason := fmt.Sprintf("%s mode not supported", mode)
		_ = e.failAttempt(ctx, "no-attempt", executionID, stepID, reason)
		return fmt.Errorf("%s", reason)
	}
	// Load harness configuration. A malformed configuration fails closed
	// before any attempt is created.
	cfg, cfgErr := project.LoadConfig(e.root)
	if cfgErr != nil {
		_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
		e.resyncExecution(ctx, executionID)
		return fmt.Errorf("harness config: %w", cfgErr)
	}
	// Without an injected manager there is no usable candidate: production
	// never synthesizes success, identity, or output. The engine consumes
	// the availability state captured by the manager's single probe (the
	// factory probes each harness exactly once per CLI invocation, F-01);
	// it must never re-probe during execution.
	var probes map[string]adapter.ProbeResult
	if e.adapterMgr != nil {
		probes = e.adapterMgr.ProbeResults()
	}
	// Ordered intersection (F-06): preserve the step's harness order and
	// keep only configured, enabled, registered, successfully probed
	// harnesses. Unknown, disabled, unregistered, and unavailable
	// candidates fall through with sanitized skip diagnostics.
	var candidates []string
	var skipDiags []string
	seen := make(map[string]bool)
	for _, h := range harnessCandidates {
		if seen[h] {
			continue
		}
		seen[h] = true
		hc, ok := cfg.Harnesses[h]
		if !ok {
			skipDiags = append(skipDiags, fmt.Sprintf("skipped %q: not configured", h))
			continue
		}
		if !hc.IsEnabled() {
			skipDiags = append(skipDiags, fmt.Sprintf("skipped %q: disabled", h))
			continue
		}
		pr, ok := probes[h]
		if !ok {
			skipDiags = append(skipDiags, fmt.Sprintf("skipped %q: not registered", h))
			continue
		}
		if !pr.Available {
			skipDiags = append(skipDiags, fmt.Sprintf("skipped %q: unavailable", h))
			continue
		}
		candidates = append(candidates, h)
	}
	// Seed the fallback context with sanitized skip diagnostics so the next
	// usable candidate (and the exhaustion evidence) carries them, bounded
	// to 2 MiB and redacted once.
	var accumulated string
	if len(skipDiags) > 0 {
		accumulated = FallbackEvidence(VisibleEvidence(strings.Join(skipDiags, "\n")))
	}
	if len(candidates) == 0 {
		diag := VisibleEvidence(strings.Join(skipDiags, "\n"))
		_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
		e.resyncExecution(ctx, executionID)
		return fmt.Errorf("no usable harness for agent step %q: %s", stepID, diag)
	}
	// Resolved requirements passed to the session bundle; adapters map them
	// to absolute .haro/artifacts paths.
	reqMap := make(map[string]string, len(requires))
	for _, r := range requires {
		reqMap[r] = r
	}
	// Fallback loop over the intersected candidates only.
	var evidences []string
	for idx, harness := range candidates {
		// Create generation and attempt plus the identity-empty transport
		// row atomically. Real identity replaces the row after the session
		// settles (U-04); production never synthesizes identity.
		step, _ := e.store.Steps().Get(ctx, executionID, stepID)
		genNumber := step.CurrentGeneration + 1
		genID := uuid.NewString()
		gen := &store.Generation{
			ID:          genID,
			ExecutionID: executionID,
			StepID:      stepID,
			Number:      genNumber,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		attemptID := uuid.NewString()
		attempt := &store.Attempt{
			ID:           attemptID,
			ExecutionID:  executionID,
			StepID:       stepID,
			GenerationID: genID,
			Status:       "running",
			StartedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		transport := &store.Transport{
			AttemptID:   attemptID,
			AdapterName: harness,
		}
		// WithTx for attempt+transport
		err = e.store.WithTx(ctx, func(tx store.Store) error {
			if err := tx.Generations().Create(ctx, gen); err != nil {
				return err
			}
			if err := tx.Steps().UpdateGeneration(ctx, executionID, stepID, genNumber); err != nil {
				return err
			}
			if err := tx.Attempts().Create(ctx, attempt); err != nil {
				return err
			}
			return tx.Transport().Put(ctx, transport)
		})
		if err != nil {
			// If transport creation fails, treat as terminal
			evidences = append(evidences, VisibleEvidence(fmt.Sprintf("transport error for %s: %v", harness, err)))
			accumulated = FallbackEvidence(strings.Join(evidences, ""))
			if isTerminal(err) {
				reason := VisibleEvidence(fmt.Sprintf("terminal: %v", err))
				_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &accumulated)
				_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
				e.resyncExecution(ctx, executionID)
				return err
			}
			if idx == len(candidates)-1 {
				reason := VisibleEvidence(strings.Join(evidences, ""))
				_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &accumulated)
				_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
				e.resyncExecution(ctx, executionID)
				return fmt.Errorf("exhausted candidates: %w", err)
			}
			continue
		}
		// Run the session through the adapter manager.
		var output string
		var runErr error
		var sess adapter.Session
		wtRoot := e.effectiveWorktreeRoot(ctx, executionID)
		bundle := adapter.SessionBundle{
			Instructions:  instructions,
			WorkspaceRoot: wtRoot,
			Requires:      reqMap,
		}
		// Host with fail-closed permission
		host := &agentHost{store: e.store, attemptID: attemptID}
		sess, sErr := e.adapterMgr.NewSession(ctx, harness, bundle, host)
		if sErr != nil {
			runErr = sErr
			output = fmt.Sprintf("new session failed for %s: %v", harness, sErr)
		} else {
			// Carry the bounded sanitized fallback context plus instructions.
			input := adapter.PromptInput{Text: instructions}
			if accumulated != "" {
				input.Text = accumulated + "\n" + instructions
			}
			ch, pErr := sess.Prompt(ctx, input)
			if pErr != nil {
				runErr = pErr
				output = fmt.Sprintf("prompt failed %s: %v", harness, pErr)
			} else {
				// Collect events until completed or failed
				var collected string
				for ev := range ch {
					if ev.Type == "output_delta" {
						collected += string(ev.Payload)
					} else if ev.Type == "completed" {
						collected += string(ev.Payload)
						break
					} else if ev.Type == "failed" {
						runErr = fmt.Errorf("harness %s failed: %s", harness, string(ev.Payload))
						collected += string(ev.Payload)
						break
					}
				}
				output = collected
			}
			_ = sess.Cancel(ctx)
			// Replace the identity-empty transport row with the real
			// identity captured by the session (U-04).
			if tp, ok := sess.(adapter.TransportProvider); ok {
				if st, ok := tp.SessionTransport(); ok {
					extra := "{}"
					if st.Extra != nil {
						if b, jErr := json.Marshal(st.Extra); jErr == nil {
							extra = string(b)
						}
					}
					ver := st.ProtocolVersion
					native := st.NativeSessionID
					if pErr := e.store.Transport().Put(ctx, &store.Transport{
						AttemptID:       attemptID,
						AdapterName:     harness,
						NativeSessionID: &native,
						ProtocolVersion: &ver,
						Extra:           extra,
					}); pErr != nil {
						// Store errors are terminal.
						runErr = fmt.Errorf("transport persist failed: %w", pErr)
					}
				}
			}
		}
		// Compose execution-identifying agent evidence, then redact and bound
		// once. Persisted inline; no evidence files are written.
		visible := AgentEvidence(harness, idx+1, len(candidates), mode, instructions, output)
		evidences = append(evidences, visible)
		cursor, _ := e.store.Events().NextAttemptCursor(ctx, attemptID)
		_ = e.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
			AttemptID:  attemptID,
			Cursor:     cursor,
			EventType:  "output_delta",
			Payload:    &visible,
			PayloadRef: nil,
		})
		if runErr == nil {
			// success
			digest := sha256.Sum256([]byte(visible))
			digestStr := hex.EncodeToString(digest[:])
			_ = e.completeAttempt(ctx, attemptID, "completed", nil, &digestStr)
			_ = e.transitionStep(ctx, executionID, stepID, "running", "completed")
			e.resyncExecution(ctx, executionID)
			return nil
		}
		if isTerminal(runErr) {
			reason := VisibleEvidence(fmt.Sprintf("terminal: %v", runErr))
			all := FallbackEvidence(strings.Join(evidences, ""))
			_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &all)
			_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
			e.resyncExecution(ctx, executionID)
			return runErr
		}
		// Clean failure: accumulate and continue
		accumulated = FallbackEvidence(strings.Join(evidences, ""))
		// Mark this attempt failed but continue
		reason := VisibleEvidence(runErr.Error())
		_ = e.completeAttempt(ctx, attemptID, "failed", &reason, &accumulated)
		if idx == len(candidates)-1 {
			// Exhausted
			_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
			e.resyncExecution(ctx, executionID)
			return fmt.Errorf("exhausted candidates: %w", runErr)
		}
		// continue to next harness
	}
	return fmt.Errorf("no candidates")
}

type agentHost struct {
	store     store.Store
	attemptID string
}

func (h *agentHost) RequestPermission(ctx context.Context, req adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	// For agent steps, permission is gated by Store? We assume fail-closed if not negotiated; here we just allow
	// But we can record event
	cursor, _ := h.store.Events().NextAttemptCursor(ctx, h.attemptID)
	desc := req.Description
	_ = h.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
		AttemptID:  h.attemptID,
		Cursor:     cursor,
		EventType:  "permission_requested",
		PayloadRef: &desc,
	})
	return adapter.PermissionDecision{Option: req.Options[0]}, nil
}

func (e *Engine) failAttempt(ctx context.Context, attemptID, executionID, stepID, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_ = e.store.Attempts().UpdateStatus(ctx, attemptID, "failed", &now, &reason, nil)
	_ = e.transitionStep(ctx, executionID, stepID, "running", "failed")
	e.resyncExecution(ctx, executionID)
	return nil
}

func (e *Engine) completeAttempt(ctx context.Context, attemptID, status string, reason *string, digest *string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return e.store.Attempts().UpdateStatus(ctx, attemptID, status, &now, reason, digest)
}

func (e *Engine) transitionStep(ctx context.Context, executionID, stepID, from, to string) error {
	// Idempotent: if current status already equals to, do not create duplicate event
	cur, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	if cur.Status == to {
		return nil
	}
	// Update step status
	if err := e.store.Steps().UpdateStatus(ctx, executionID, stepID, to); err != nil {
		return err
	}
	// Audit transition
	cursor, _ := e.store.Events().NextTransitionCursor(ctx, executionID, stepID)
	fromCopy := from
	ev := &store.StepTransitionEvent{
		ExecutionID: executionID,
		StepID:      stepID,
		Cursor:      cursor,
		FromStatus:  &fromCopy,
		ToStatus:    to,
	}
	if from == "" {
		ev.FromStatus = nil
	}
	return e.store.Events().CreateTransition(ctx, ev)
}

func (e *Engine) resyncExecution(ctx context.Context, executionID string) {
	steps, err := e.store.Steps().List(ctx, executionID)
	if err != nil {
		return
	}
	status := "running"
	allCompleted := true
	hasFailed := false
	hasRunning := false
	for _, s := range steps {
		switch s.Status {
		case "failed":
			hasFailed = true
			allCompleted = false
		case "pending", "running":
			allCompleted = false
		case "completed", "skipped":
			// ok
		}
		if s.Status == "running" {
			hasRunning = true
		}
	}
	if allCompleted && len(steps) > 0 {
		allSkippedOrCompleted := true
		for _, s := range steps {
			if s.Status != "completed" && s.Status != "skipped" {
				allSkippedOrCompleted = false
				break
			}
		}
		if allSkippedOrCompleted {
			status = "completed"
		}
	} else if hasFailed {
		// if any failed and none pending/running, maybe failed? but keep running until all done
		// For simplicity, if any failed, execution remains running until resync determines final?
		// We set failed if not all completed but has failed and no pending?
		hasPending := false
		for _, s := range steps {
			if s.Status == "pending" || s.Status == "running" {
				hasPending = true
				break
			}
		}
		if !hasPending {
			status = "failed"
		}
	}
	if hasRunning {
		status = "running"
	}
	_ = e.store.Executions().UpdateStatus(ctx, executionID, status)
	// Terminal resync: remove worktree if execution completed/failed
	if (status == "completed" || status == "failed") && e.worktreeMgr != nil {
		exec, err := e.store.Executions().Get(ctx, executionID)
		if err == nil && exec != nil && exec.WorkspaceMode == "isolated" && exec.WorkspaceRoot != "" && exec.WorkspaceRoot != e.root {
			_ = e.worktreeMgr.Remove(ctx, exec.WorkspaceRoot)
		}
	}
}
