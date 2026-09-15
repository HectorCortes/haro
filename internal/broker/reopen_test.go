package broker

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestStepReopenReturnsStableCascadeAndPersistsFeedback(t *testing.T) {
	d, s, executionID := newReopenDaemon(t)
	result, rpcErr := d.handleStepReopen(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "s1",
		"feedback":     "fix the first artifact",
	}))
	if rpcErr != nil {
		t.Fatalf("reopen error=%+v", rpcErr)
	}
	got := result.(stepReopenResult)
	if want := []string{"s1", "s2", "s3"}; !reflect.DeepEqual(got.Invalidated, want) {
		t.Fatalf("invalidated=%v, want %v", got.Invalidated, want)
	}
	for _, stepID := range got.Invalidated {
		step, err := s.Steps().Get(context.Background(), executionID, stepID)
		if err != nil || step.Status != "pending" {
			t.Fatalf("step %s=%+v err=%v, want pending", stepID, step, err)
		}
		generations, err := s.Generations().ListByStep(context.Background(), executionID, stepID)
		if err != nil || len(generations) != 1 || generations[0].InvalidatedAt == nil || generations[0].InvalidatedByStep == nil || *generations[0].InvalidatedByStep != "s1" {
			t.Fatalf("generations %s=%+v err=%v, want invalidated by s1", stepID, generations, err)
		}
	}
	root, err := s.Steps().Get(context.Background(), executionID, "s1")
	if err != nil || root.PendingFeedback == nil || *root.PendingFeedback != "fix the first artifact" {
		t.Fatalf("root feedback=%+v err=%v", root, err)
	}
	transitions, err := s.Events().ListTransitions(context.Background(), executionID, "s1")
	if err != nil || len(transitions) < 1 {
		t.Fatalf("transitions=%+v err=%v, want reopen audit", transitions, err)
	}
}

func TestStepReopenRejectsUnboundedFeedbackBeforeMutation(t *testing.T) {
	d, s, executionID := newReopenDaemon(t)
	before, err := s.Steps().Get(context.Background(), executionID, "s1")
	if err != nil {
		t.Fatal(err)
	}
	_, rpcErr := d.handleStepReopen(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "s1",
		"feedback":     string(make([]byte, execution.FallbackLimit+1)),
	}))
	if rpcErr == nil || rpcErr.Code != -32602 {
		t.Fatalf("reopen error=%+v, want bounded invalid params", rpcErr)
	}
	after, err := s.Steps().Get(context.Background(), executionID, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != after.Status || before.PendingFeedback != after.PendingFeedback {
		t.Fatalf("rejected feedback mutated step: before=%+v after=%+v", before, after)
	}
}

func TestStepReopenFeedbackIsDeliveredToNextAttempt(t *testing.T) {
	d, s, executionID := newReopenDaemon(t)
	if _, rpcErr := d.handleStepReopen(context.Background(), mustJSON(t, map[string]any{
		"execution_id": executionID,
		"step_id":      "s1",
		"feedback":     "preserve this complete context",
	})); rpcErr != nil {
		t.Fatalf("reopen error=%+v", rpcErr)
	}
	attempt, err := d.engine.StartStep(context.Background(), executionID, "s1", "headless")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.engine.ExecuteAttempt(context.Background(), attempt.ID); err != nil {
		t.Fatal(err)
	}
	events, err := s.Events().ListAttemptEvents(context.Background(), attempt.ID, 0, 128)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, event := range events {
		if event.EventType == "feedback" && event.Payload != nil && *event.Payload == "---FEEDBACK---\npreserve this complete context\n---END---" {
			found = true
		}
	}
	if !found {
		t.Fatalf("feedback event missing from %+v", events)
	}
	step, err := s.Steps().Get(context.Background(), executionID, "s1")
	if err != nil || step.PendingFeedback != nil {
		t.Fatalf("pending feedback=%+v err=%v, want cleared", step.PendingFeedback, err)
	}
}

func newReopenDaemon(t *testing.T) (*Daemon, *store.SQLiteStore, string) {
	t.Helper()
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Projects().Create(context.Background(), root, root); err != nil {
		t.Fatal(err)
	}
	executionID := "reopen-execution"
	workflowPath := writeTestWorkflow(t, root, "reopen", `version: 2
name: reopen
steps:
  - id: s1
    type: command
    run: echo s1
`)
	if err := s.Executions().Create(context.Background(), &store.Execution{ID: executionID, ProjectID: root, WorkflowSource: workflowPath, Status: "running", WorkspaceMode: "shared", WorkspaceRoot: root, StartedAt: "now"}); err != nil {
		t.Fatal(err)
	}
	steps := []*store.ExecutionStep{
		{ExecutionID: executionID, StepID: "s1", Type: "command", Status: "completed", DependsOn: "[]", Requires: "[]", Produces: `["a.txt"]`, WorkspaceMode: "shared", CurrentGeneration: 1},
		{ExecutionID: executionID, StepID: "s2", Type: "command", Status: "completed", DependsOn: `["s1"]`, Requires: `["a.txt"]`, Produces: `["b.txt"]`, WorkspaceMode: "shared", CurrentGeneration: 1},
		{ExecutionID: executionID, StepID: "s3", Type: "command", Status: "completed", DependsOn: `["s2"]`, Requires: `["b.txt"]`, Produces: "[]", WorkspaceMode: "shared", CurrentGeneration: 1},
	}
	for _, step := range steps {
		if err := s.Steps().Create(context.Background(), step); err != nil {
			t.Fatal(err)
		}
		generationID := step.StepID + "-generation-1"
		if err := s.Generations().Create(context.Background(), &store.Generation{ID: generationID, ExecutionID: executionID, StepID: step.StepID, Number: 1, CreatedAt: "now"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Attempts().Create(context.Background(), &store.Attempt{ID: step.StepID + "-attempt-1", ExecutionID: executionID, StepID: step.StepID, GenerationID: generationID, Status: "completed", StartedAt: "now"}); err != nil {
			t.Fatal(err)
		}
	}
	d := testDaemon(t, root, s)
	d.registerStepHandlers()
	return d, s, executionID
}
