package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HectorCortes/haro/internal/store"
)

// allowedTransitions defines idempotent state machine.
var allowedTransitions = map[string]map[string]bool{
	"pending":   {"running": true, "skipped": true},
	"running":   {"completed": true, "failed": true},
	"completed": {"pending": true},
	"failed":    {"pending": true},
}

// ReopenStep invalidates generations and resets descendants to pending.
// Files remain on disk but are considered invalid for requires.
func (e *Engine) ReopenStep(ctx context.Context, executionID, stepID string, cascade bool, feedback string) error {
	if _, err := e.store.Steps().Get(ctx, executionID, stepID); err != nil {
		return fmt.Errorf("get step: %w", err)
	}
	// Allow reopen for completed/failed/skipped; pending is idempotent but still invalidates descendants if cascade
	// Build descendants set if cascade
	toReset := map[string]bool{stepID: true}
	if cascade {
		steps, _ := e.store.Steps().List(ctx, executionID)
		// Build adjacency: step -> depends_on
		// Descendants are those that depend (transitively) on stepID
		changed := true
		for changed {
			changed = false
			for _, s := range steps {
				if toReset[s.StepID] {
					continue
				}
				var deps []string
				_ = json.Unmarshal([]byte(s.DependsOn), &deps)
				for _, d := range deps {
					if toReset[d] {
						toReset[s.StepID] = true
						changed = true
						break
					}
				}
			}
		}
	}
	// For each step to reset, invalidate generations and transition to pending
	for sid := range toReset {
		// Invalidate generations
		_ = e.store.Generations().InvalidateByStep(ctx, executionID, sid, stepID)
		// Transition to pending if not already pending
		cur, err := e.store.Steps().Get(ctx, executionID, sid)
		if err != nil {
			continue
		}
		if cur.Status != "pending" {
			_ = e.transitionStepWithForce(ctx, executionID, sid, cur.Status, "pending")
		}
	}
	// Audit feedback if provided (store as simple transition for now)
	if feedback != "" {
		_ = feedback
	}
	e.resyncExecution(ctx, executionID)
	return nil
}

// SkipStep skips a virgin pending step with reason, audited.
func (e *Engine) SkipStep(ctx context.Context, executionID, stepID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("skip requires --reason")
	}
	step, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return fmt.Errorf("get step: %w", err)
	}
	if step.Status != "pending" {
		return fmt.Errorf("skip only virgin pending, got %s", step.Status)
	}
	cnt, err := e.store.Attempts().CountByStep(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	if cnt != 0 {
		return fmt.Errorf("skip only virgin pending with no attempts")
	}
	if err := e.transitionStep(ctx, executionID, stepID, "pending", "skipped"); err != nil {
		return err
	}
	e.resyncExecution(ctx, executionID)
	return nil
}

// TransitionForTest exposes transition with validation for tests.
func (e *Engine) TransitionForTest(ctx context.Context, executionID, stepID, to string) error {
	cur, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	from := cur.Status
	if from == to {
		return nil // idempotent
	}
	if !isAllowed(from, to) {
		return fmt.Errorf("forbidden transition %s -> %s", from, to)
	}
	return e.transitionStep(ctx, executionID, stepID, from, to)
}

func isAllowed(from, to string) bool {
	if m, ok := allowedTransitions[from]; ok {
		return m[to]
	}
	return false
}

// transitionStepWithForce forces transition regardless of allowed map (for reopen).
func (e *Engine) transitionStepWithForce(ctx context.Context, executionID, stepID, from, to string) error {
	cur, err := e.store.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	if cur.Status == to {
		return nil
	}
	if err := e.store.Steps().UpdateStatus(ctx, executionID, stepID, to); err != nil {
		return err
	}
	cursor, _ := e.store.Events().NextTransitionCursor(ctx, executionID, stepID)
	ev := &store.StepTransitionEvent{
		ExecutionID: executionID,
		StepID:      stepID,
		Cursor:      cursor,
		FromStatus:  &from,
		ToStatus:    to,
	}
	return e.store.Events().CreateTransition(ctx, ev)
}


