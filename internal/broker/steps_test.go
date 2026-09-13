package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestStepRunRejectsUnsupportedModeBeforeAttempt(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	workflowPath := writeTestWorkflow(t, root, "guard", `version: 2
name: guard
steps:
  - id: one
    type: command
    run: echo one
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	d := testDaemonWithRunner(t, root, s, &execution.FakeRunner{})
	start, rpcErr := d.handleExecutionStart(context.Background(), mustJSON(t, map[string]string{"workflow_path": workflowPath}))
	if rpcErr != nil {
		t.Fatalf("start: %+v", rpcErr)
	}
	executionID := start.(map[string]string)["execution_id"]

	for _, mode := range []string{"supervised", "terminal"} {
		result, runErr := d.handleStepRun(context.Background(), mustJSON(t, map[string]string{
			"execution_id": executionID,
			"step_id":      "one",
			"mode":         mode,
		}))
		if result != nil || runErr == nil || runErr.Code != -32003 || runErr.Message != mode+" mode not supported" {
			t.Fatalf("mode %q result=%#v err=%+v", mode, result, runErr)
		}
	}
	assertStepHasNoRuntimeRows(t, s, executionID, "one")
}

func TestStartStepAndExecuteAttemptAreSeparated(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	workflowPath := writeTestWorkflow(t, root, "async", `version: 2
name: async
steps:
  - id: one
    type: command
    run: echo one
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	started := make(chan struct{})
	release := make(chan struct{})
	runner := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		close(started)
		select {
		case <-release:
			return 0, "done", "", nil
		case <-ctx.Done():
			return 0, "", "", ctx.Err()
		}
	}}
	d := testDaemonWithRunner(t, root, s, runner)
	start, rpcErr := d.handleExecutionStart(context.Background(), mustJSON(t, map[string]string{"workflow_path": workflowPath}))
	if rpcErr != nil {
		t.Fatalf("start: %+v", rpcErr)
	}
	executionID := start.(map[string]string)["execution_id"]
	result, runErr := d.handleStepRun(context.Background(), mustJSON(t, map[string]string{
		"execution_id": executionID,
		"step_id":      "one",
	}))
	if runErr != nil {
		t.Fatalf("step.run: %+v", runErr)
	}
	payload, ok := result.(stepRunResult)
	if !ok || payload.AttemptID == "" || payload.NextCursor != 0 {
		t.Fatalf("step.run result=%#v", result)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("ExecuteAttempt did not start")
	}
	if got, err := s.Attempts().CountByStep(context.Background(), executionID, "one"); err != nil || got != 1 {
		t.Fatalf("attempt count=%d err=%v, want one attempt", got, err)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		attempt, getErr := s.Attempts().Get(context.Background(), payload.AttemptID)
		if getErr == nil && attempt.Status == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	attempt, err := s.Attempts().Get(context.Background(), payload.AttemptID)
	t.Fatalf("attempt did not complete: %+v (%v)", attempt, err)
}

func TestStepEventsProjectsCurrentAttemptWithStablePages(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	workflowPath := writeTestWorkflow(t, root, "events", `version: 2
name: events
steps:
  - id: one
    type: command
    run: echo one
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	d := testDaemonWithRunner(t, root, s, &execution.FakeRunner{})
	start, rpcErr := d.handleExecutionStart(context.Background(), mustJSON(t, map[string]string{"workflow_path": workflowPath}))
	if rpcErr != nil {
		t.Fatalf("start: %+v", rpcErr)
	}
	executionID := start.(map[string]string)["execution_id"]
	if err := seedAttemptEvents(context.Background(), s, executionID, "one"); err != nil {
		t.Fatal(err)
	}

	first, rpcErr := d.handleStepEvents(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "one",
		"since_cursor": 0,
	}))
	if rpcErr != nil {
		t.Fatalf("first page: %+v", rpcErr)
	}
	page := first.(stepEventsResult)
	if len(page.Events) != 128 || page.Events[0].Cursor != 0 || page.Events[127].Cursor != 127 || page.NextCursor != 128 {
		t.Fatalf("first page=%+v", page)
	}
	retry, rpcErr := d.handleStepEvents(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "one",
		"since_cursor": 0,
	}))
	if rpcErr != nil || retry.(stepEventsResult).Events[0].Payload != page.Events[0].Payload {
		t.Fatalf("retry=%#v err=%+v", retry, rpcErr)
	}
	second, rpcErr := d.handleStepEvents(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "one",
		"since_cursor": page.NextCursor,
	}))
	if rpcErr != nil {
		t.Fatalf("second page: %+v", rpcErr)
	}
	if got := second.(stepEventsResult); len(got.Events) != 1 || got.Events[0].Cursor != 128 || got.NextCursor != 129 {
		t.Fatalf("second page=%+v", got)
	}
}

func seedAttemptEvents(ctx context.Context, s store.Store, executionID, stepID string) error {
	step, err := s.Steps().Get(ctx, executionID, stepID)
	if err != nil {
		return err
	}
	gen := &store.Generation{ID: "seed-generation", ExecutionID: executionID, StepID: stepID, Number: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := s.Generations().Create(ctx, gen); err != nil {
		return err
	}
	if err := s.Steps().UpdateGeneration(ctx, executionID, stepID, 1); err != nil {
		return err
	}
	attempt := &store.Attempt{ID: "seed-attempt", ExecutionID: executionID, StepID: stepID, GenerationID: gen.ID, Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := s.Attempts().Create(ctx, attempt); err != nil {
		return err
	}
	for cursor := 0; cursor < 129; cursor++ {
		payload := fmt.Sprintf("event-%d", cursor)
		value := payload
		if err := s.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attempt.ID, Cursor: cursor, EventType: "output_delta", Payload: &value}); err != nil {
			return err
		}
	}
	if step.Status != "pending" {
		return errors.New("seed step unexpectedly changed")
	}
	return nil
}

func assertStepHasNoRuntimeRows(t *testing.T, s *store.SQLiteStore, executionID, stepID string) {
	t.Helper()
	attempts, err := s.Attempts().CountByStep(context.Background(), executionID, stepID)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("attempts=%d, want zero", attempts)
	}
	steps, err := s.Steps().List(context.Background(), executionID)
	if err != nil || len(steps) != 1 || steps[0].Status != "pending" || steps[0].CurrentGeneration != 0 {
		t.Fatalf("steps=%+v err=%v", steps, err)
	}
}

func testDaemonWithRunner(t *testing.T, root string, s store.Store, runner execution.CommandRunner) *Daemon {
	t.Helper()
	d := testDaemon(t, root, s)
	d.engine = execution.NewEngine(s, runner, root)
	d.registerStepHandlers()
	return d
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var _ = os.ErrNotExist
