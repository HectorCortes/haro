package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
)

// Engine orchestrates command execution with store and runner.
type Engine struct {
	store  store.Store
	runner CommandRunner
	root   string
}

// NewEngine creates an engine.
func NewEngine(s store.Store, r CommandRunner, root string) *Engine {
	return &Engine{store: s, runner: r, root: root}
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
	if err := workflow.Validate(wf); err != nil {
		return "", fmt.Errorf("validate: %w", err)
	}
	// Ensure project exists
	projectID := e.root // simple: use root as id
	_ = e.store.Projects().Create(ctx, projectID, e.root)
	// Generate execution id
	execID := uuid.NewString()
	workspaceRoot := e.root
	startedAt := time.Now().UTC().Format(time.RFC3339)
	exec := &store.Execution{
		ID:              execID,
		ProjectID:       projectID,
		WorkflowSource:  wfPath,
		Status:          "running",
		WorkspaceMode:   "isolated",
		WorkspaceRoot:   workspaceRoot,
		StartedAt:       startedAt,
	}
	err = e.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.Executions().Create(ctx, exec); err != nil {
			return err
		}
		for _, st := range wf.Steps {
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
			wsMode := "isolated"
			step := &store.ExecutionStep{
				ExecutionID:       execID,
				StepID:            st.ID,
				Type:              st.Type,
				Status:            "pending",
				DependsOn:         string(depJSON),
				Requires:          string(reqJSON),
				Produces:          string(prodJSON),
				WorkspaceMode:     wsMode,
				CurrentGeneration: 0,
			}
			if err := tx.Steps().Create(ctx, step); err != nil {
				return err
			}
			// Audit initial pending transition? For idempotency we create an event for pending? Not needed.
			// Create initial transition pending->pending? No, skip.
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return execID, nil
}

// RunStep executes a step with the 4 command-cycle cases.
func (e *Engine) RunStep(ctx context.Context, executionID, stepID, feedback string) error {
	// Fetch step
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return fmt.Errorf("get step: %w", err)
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
		// Check generation invalidated? For requires, we need to ensure that the step that produces this file has a valid generation.
		// Simplify: if any generation for any step is invalidated, and that step produces this file, then requires is invalid.
		// We check all steps that produce this file and ensure their current generation is not invalidated.
		steps, _ := e.store.Steps().List(ctx, executionID)
		for _, s := range steps {
			var prods []string
			_ = json.Unmarshal([]byte(s.Produces), &prods)
			for _, p := range prods {
				if p == req {
					// Check generations for that step: if any invalidated, then file is stale
					gens, _ := e.store.Generations().ListByStep(ctx, executionID, s.StepID)
					for _, g := range gens {
						if g.InvalidatedAt != nil {
							// If the file's generation is invalidated, then requires fails
							// For simplicity, if any generation invalidated, treat requires as missing
							// But we need to know which generation the file belongs to. For now, if step has any invalidated generation, requires fails.
							// This will make stale produces invalid after reopen.
							return fmt.Errorf("requires %q invalidated (generation %d)", req, g.Number)
						}
					}
				}
			}
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
	// Feedback handling: delimited feedback + bounded prior context
	if feedback != "" {
		// Fetch prior visible evidence for this step (last attempt's evidence, excluding current)
		var prior string
		if sqliteStore, ok := e.store.(*store.SQLiteStore); ok {
			rows, _ := sqliteStore.QueryForTest(ctx, "SELECT id FROM attempts WHERE execution_id = ? AND step_id = ? AND id != ? ORDER BY started_at DESC LIMIT 1", executionID, stepID, attemptID)
			if rows != nil {
				var priorID string
				if rows.Next() {
					_ = rows.Scan(&priorID)
					evPath := filepath.Join(artifactsRoot, "evidence", priorID+".txt")
					if data, err := os.ReadFile(evPath); err == nil {
						prior = string(data)
					}
				}
				_ = rows.Close()
			}
		}
		// Bound prior to FallbackLimit
		delimited := fmt.Sprintf("---FEEDBACK---\n%s\n---END---", feedback)
		// Keep total within FallbackLimit: prior truncated + delimited
		available := FallbackLimit - len(delimited)
		if available < 0 {
			available = 0
		}
		if len(prior) > available {
			prior = FallbackEvidence(prior) // ensures <=2MiB
			if len(prior) > available {
				prior = prior[:available]
			}
		}
		combined := prior + delimited
		// Ensure combined within fallback limit
		combined = FallbackEvidence(combined)
		_ = combined
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
	// Run via runner
	exitCode, stdout, stderr, runErr := e.runner.Run(ctx, e.root, argv, envMap)
	if runErr != nil {
		// Start failure or timeout
		_ = e.failAttempt(ctx, attemptID, executionID, stepID, runErr.Error())
		return fmt.Errorf("runner: %w", runErr)
	}
	visible := VisibleEvidence(stdout + stderr)
	// Store visible evidence as attempt event (payload_ref points to file)
	evidencePath := filepath.Join(artifactsRoot, "evidence", attemptID+".txt")
	_ = os.MkdirAll(filepath.Dir(evidencePath), 0o755)
	_ = os.WriteFile(evidencePath, []byte(visible), 0o600)
	cursor, _ := e.store.Events().NextAttemptCursor(ctx, attemptID)
	_ = e.store.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
		AttemptID:  attemptID,
		Cursor:     cursor,
		EventType:  "output_delta",
		PayloadRef: &evidencePath,
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
}
