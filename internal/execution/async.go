package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
	"github.com/google/uuid"
)

// StartStep performs the pre-attempt checks and persists the running attempt.
// ExecuteAttempt is intentionally separate so RPC callers can acknowledge the
// attempt before a command starts or blocks on its runner.
func (e *Engine) StartStep(ctx context.Context, executionID, stepID, requestedMode string) (*store.Attempt, error) {
	if err := e.verifyDAGHash(ctx, executionID); err != nil {
		return nil, err
	}
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return nil, fmt.Errorf("get step: %w", err)
	}
	if step.Type == "workflow" {
		return nil, fmt.Errorf("workflow node %q cannot run", stepID)
	}
	mode, err := e.stepMode(ctx, executionID, stepID, requestedMode)
	if err != nil {
		return nil, err
	}
	if mode == "supervised" || mode == "terminal" {
		return nil, fmt.Errorf("%s mode not supported", mode)
	}
	if step.Status != "pending" {
		return nil, fmt.Errorf("step %q not pending (status=%s)", stepID, step.Status)
	}
	if err := e.checkStepInputs(ctx, executionID, step); err != nil {
		return nil, err
	}
	claims, err := e.acquireClaims(ctx, executionID, stepID)
	if err != nil {
		return nil, err
	}
	leaseToken, err := e.store.Leases().Acquire(ctx, executionID, stepID, e.leaseHolder)
	if err != nil {
		e.releaseClaims(ctx, executionID, stepID, claims)
		return nil, err
	}
	startedAt := time.Now().UTC().Format(time.RFC3339)
	gen := &store.Generation{
		ID:          uuid.NewString(),
		ExecutionID: executionID,
		StepID:      stepID,
		Number:      step.CurrentGeneration + 1,
		CreatedAt:   startedAt,
	}
	attempt := &store.Attempt{
		ID:           uuid.NewString(),
		ExecutionID:  executionID,
		StepID:       stepID,
		GenerationID: gen.ID,
		Status:       "running",
		StartedAt:    startedAt,
	}
	setupStore := NewFencedStore(e.store, executionID, stepID, e.leaseHolder, leaseToken)
	err = setupStore.WithTx(ctx, func(tx store.Store) error {
		if err := tx.Steps().UpdateStatus(ctx, executionID, stepID, "running"); err != nil {
			return err
		}
		cursor, err := tx.Events().NextTransitionCursor(ctx, executionID, stepID)
		if err != nil {
			return err
		}
		from := "pending"
		if err := tx.Events().CreateTransition(ctx, &store.StepTransitionEvent{
			ExecutionID: executionID,
			StepID:      stepID,
			Cursor:      cursor,
			FromStatus:  &from,
			ToStatus:    "running",
		}); err != nil {
			return err
		}
		if err := tx.Generations().Create(ctx, gen); err != nil {
			return err
		}
		if err := tx.Steps().UpdateGeneration(ctx, executionID, stepID, gen.Number); err != nil {
			return err
		}
		return tx.Attempts().Create(ctx, attempt)
	})
	if err != nil {
		_ = e.store.Leases().Release(ctx, executionID, stepID, e.leaseHolder, leaseToken)
		e.releaseClaims(ctx, executionID, stepID, claims)
		return nil, err
	}
	e.trackAttemptClaims(attempt.ID, executionID, stepID, claims)
	e.trackAttemptLease(attempt.ID, executionID, stepID, leaseToken)
	return attempt, nil
}

// ExecuteAttempt runs a previously persisted command attempt. It is safe to
// call from a goroutine owned by the broker; all terminal state is persisted.
func (e *Engine) ExecuteAttempt(ctx context.Context, attemptID string) error {
	attempt, err := e.store.Attempts().Get(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("get attempt: %w", err)
	}
	if attempt.Status != "running" {
		return fmt.Errorf("attempt %q not running", attemptID)
	}
	writeStore, err := e.fencedStoreForAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	defer e.releaseAttemptClaims(ctx, attempt)
	defer e.releaseAttemptLease(ctx, attempt)
	step, err := e.store.Steps().Get(ctx, attempt.ExecutionID, attempt.StepID)
	if err != nil {
		return fmt.Errorf("get step: %w", err)
	}
	if step.Type != "command" {
		reason := fmt.Sprintf("unsupported async step type: %s", step.Type)
		_ = e.finishAttempt(ctx, attempt, "failed", &reason, nil)
		return errors.New(reason)
	}
	argv, env, err := e.commandForStep(ctx, attempt.ExecutionID, attempt.StepID)
	if err != nil {
		_ = e.finishAttempt(ctx, attempt, "failed", stringPtr(err.Error()), nil)
		return err
	}
	if step.PendingFeedback != nil {
		feedback := VisibleEvidence(fmt.Sprintf("---FEEDBACK---\n%s\n---END---", *step.PendingFeedback))
		cursor, cursorErr := writeStore.Events().NextAttemptCursor(ctx, attempt.ID)
		if cursorErr == nil {
			copy := feedback
			_ = writeStore.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attempt.ID, Cursor: cursor, EventType: "feedback", Payload: &copy})
		}
		if err := writeStore.Steps().SetPendingFeedback(ctx, attempt.ExecutionID, attempt.StepID, nil); err != nil {
			return fmt.Errorf("clear pending feedback: %w", err)
		}
	}
	exitCode, stdout, stderr, runErr := e.runner.Run(ctx, e.effectiveWorktreeRoot(ctx, attempt.ExecutionID), argv, env)
	if runErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = e.finishAttempt(ctx, attempt, "failed", stringPtr(runErr.Error()), nil)
		return fmt.Errorf("runner: %w", runErr)
	}
	if err := e.assertAttemptRunning(ctx, attempt.ID); err != nil {
		return err
	}
	visible := CommandEvidence(argv, stdout, stderr)
	cursor, cursorErr := writeStore.Events().NextAttemptCursor(ctx, attempt.ID)
	if cursorErr != nil {
		return cursorErr
	}
	if err := writeStore.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attempt.ID, Cursor: cursor, EventType: "output_delta", Payload: &visible}); err != nil {
		return err
	}
	if exitCode != 0 {
		reason := fmt.Sprintf("exit %d", exitCode)
		_ = e.finishAttempt(ctx, attempt, "failed", &reason, &visible)
		return nil
	}
	if err := e.finishAttempt(ctx, attempt, "completed", nil, &visible); err != nil {
		return err
	}
	return nil
}

// CancelAttempt records cancellation and releases the attempt's claims and
// lease. A terminal attempt is already cancelled from the broker's perspective
// and is therefore idempotent.
func (e *Engine) CancelAttempt(ctx context.Context, attemptID string) error {
	attempt, err := e.store.Attempts().Get(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt.Status != "running" {
		if attempt.Status == "cancelled" {
			return nil
		}
		return fmt.Errorf("attempt %q is not running (status=%s)", attemptID, attempt.Status)
	}
	reason := "cancelled"
	if err := e.store.Attempts().UpdateStatus(ctx, attempt.ID, "cancelled", nil, &reason, nil); err != nil {
		return err
	}
	if err := e.transitionStep(ctx, attempt.ExecutionID, attempt.StepID, "running", "failed"); err != nil {
		return err
	}
	e.releaseAttemptClaims(ctx, attempt)
	e.releaseAttemptLease(ctx, attempt)
	e.resyncExecution(ctx, attempt.ExecutionID)
	return nil
}

func (e *Engine) checkStepInputs(ctx context.Context, executionID string, step *store.ExecutionStep) error {
	var deps []string
	if err := json.Unmarshal([]byte(step.DependsOn), &deps); err != nil {
		return fmt.Errorf("invalid dependencies: %w", err)
	}
	for _, dep := range deps {
		depStep, err := e.store.Steps().Get(ctx, executionID, dep)
		if err != nil {
			return fmt.Errorf("missing dependency %q: %w", dep, err)
		}
		if depStep.Status != "completed" && depStep.Status != "skipped" {
			return fmt.Errorf("unsatisfied dependency %q: status %s", dep, depStep.Status)
		}
	}
	var requires, produces []string
	if err := json.Unmarshal([]byte(step.Requires), &requires); err != nil {
		return fmt.Errorf("invalid requires: %w", err)
	}
	if err := json.Unmarshal([]byte(step.Produces), &produces); err != nil {
		return fmt.Errorf("invalid produces: %w", err)
	}
	artifactsRoot := filepath.Join(e.root, ".haro", "artifacts")
	for _, required := range requires {
		if err := workflow.ValidateContainedPath(artifactsRoot, required); err != nil {
			return fmt.Errorf("requires containment: %w", err)
		}
		if _, err := os.Stat(filepath.Join(artifactsRoot, required)); err != nil {
			return fmt.Errorf("requires %q missing: %w", required, err)
		}
		if generation, stale := e.requiresStale(ctx, executionID, required); stale {
			return fmt.Errorf("requires %q invalidated (generation %d)", required, generation)
		}
	}
	for _, produced := range produces {
		if err := workflow.ValidateContainedPath(artifactsRoot, produced); err != nil {
			return fmt.Errorf("produces containment: %w", err)
		}
	}
	return nil
}

func (e *Engine) stepMode(ctx context.Context, executionID, stepID, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	exec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil {
		return "", err
	}
	file, err := os.Open(exec.WorkflowSource)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	wf, err := workflow.Parse(file)
	if err != nil {
		return "", err
	}
	for _, step := range wf.Steps {
		if step.ID == stepID {
			return step.Mode, nil
		}
	}
	return "", fmt.Errorf("step %q not found in workflow", stepID)
}

func (e *Engine) commandForStep(ctx context.Context, executionID, stepID string) ([]string, map[string]string, error) {
	exec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(exec.WorkflowSource)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = file.Close() }()
	wf, err := workflow.Parse(file)
	if err != nil {
		return nil, nil, err
	}
	for _, step := range wf.Steps {
		if step.ID != stepID {
			continue
		}
		argv, err := ParseArgv(step.Run)
		if err != nil {
			return nil, nil, err
		}
		return argv, step.Env, nil
	}
	return nil, nil, fmt.Errorf("run command empty for step %q", stepID)
}

func (e *Engine) finishAttempt(ctx context.Context, attempt *store.Attempt, status string, reason, digest *string) error {
	if err := e.assertAttemptRunning(ctx, attempt.ID); err != nil {
		return err
	}
	writeStore, err := e.fencedStoreForAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	if err := writeStore.Attempts().UpdateStatus(ctx, attempt.ID, status, nil, reason, digest); err != nil {
		return err
	}
	if err := e.transitionStepWithStore(ctx, writeStore, attempt.ExecutionID, attempt.StepID, "running", statusToStepStatus(status)); err != nil {
		return err
	}
	e.resyncExecutionWithStore(ctx, writeStore, attempt.ExecutionID)
	return nil
}

func (e *Engine) assertAttemptRunning(ctx context.Context, attemptID string) error {
	checkCtx := context.WithoutCancel(ctx)
	attempt, err := e.store.Attempts().Get(checkCtx, attemptID)
	if err != nil {
		return err
	}
	if attempt.Status != "running" {
		return fmt.Errorf("attempt %q is stale (status=%s)", attemptID, attempt.Status)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func statusToStepStatus(status string) string {
	if status == "completed" {
		return "completed"
	}
	return "failed"
}

func stringPtr(value string) *string { return &value }

func (e *Engine) trackAttemptClaims(attemptID, executionID, stepID string, claims []string) {
	e.activeClaimsMu.Lock()
	if e.activeClaims == nil {
		e.activeClaims = make(map[string]attemptClaims)
	}
	e.activeClaims[attemptID] = attemptClaims{executionID: executionID, stepID: stepID, paths: claims}
	e.activeClaimsMu.Unlock()
}

func (e *Engine) releaseAttemptClaims(ctx context.Context, attempt *store.Attempt) {
	e.activeClaimsMu.Lock()
	claims, ok := e.activeClaims[attempt.ID]
	if ok {
		delete(e.activeClaims, attempt.ID)
	}
	e.activeClaimsMu.Unlock()
	if ok {
		e.releaseClaims(context.WithoutCancel(ctx), claims.executionID, claims.stepID, claims.paths)
	}
}

type attemptClaims struct {
	executionID string
	stepID      string
	paths       []string
}

type attemptLease struct {
	executionID string
	stepID      string
	token       int64
}

func (e *Engine) trackAttemptLease(attemptID, executionID, stepID string, token int64) {
	e.activeLeasesMu.Lock()
	e.activeLeases[attemptID] = attemptLease{executionID: executionID, stepID: stepID, token: token}
	e.activeLeasesMu.Unlock()
}

func (e *Engine) fencedStoreForAttempt(ctx context.Context, attempt *store.Attempt) (store.Store, error) {
	e.activeLeasesMu.Lock()
	lease, ok := e.activeLeases[attempt.ID]
	e.activeLeasesMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w: attempt lease is not active; reopen and rerun", ErrStaleLease)
	}
	return NewFencedStore(e.store, lease.executionID, lease.stepID, e.leaseHolder, lease.token), nil
}

func (e *Engine) releaseAttemptLease(ctx context.Context, attempt *store.Attempt) {
	e.activeLeasesMu.Lock()
	lease, ok := e.activeLeases[attempt.ID]
	if ok {
		delete(e.activeLeases, attempt.ID)
	}
	e.activeLeasesMu.Unlock()
	if ok {
		_ = e.store.Leases().Release(context.WithoutCancel(ctx), lease.executionID, lease.stepID, e.leaseHolder, lease.token)
	}
}
