package broker

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestStepApproveEnforcesInteractionCASAndAvailableDecisions(t *testing.T) {
	d, s, executionID, attemptID := newInteractionDaemon(t)
	interaction := &store.Interaction{ID: "interaction-1", AttemptID: attemptID, Type: "permission", Status: "pending", IdempotencyKey: "interaction-1"}
	if err := s.Interactions().Create(context.Background(), interaction); err != nil {
		t.Fatal(err)
	}
	payload := `{"interaction_id":"interaction-1","available_decisions":["allow","deny"],"description":"approve","options":["allow","deny"]}`
	if err := s.Events().CreateAttemptEvent(context.Background(), &store.AttemptEvent{AttemptID: attemptID, Cursor: 0, EventType: "interaction_required", Payload: &payload}); err != nil {
		t.Fatal(err)
	}
	for name, params := range map[string]map[string]string{
		"unknown decision": {
			"execution_id":    executionID,
			"step_id":         "one",
			"interaction_id":  interaction.ID,
			"decision":        "maybe",
			"idempotency_key": interaction.ID,
		},
		"different key": {
			"execution_id":    executionID,
			"step_id":         "one",
			"interaction_id":  interaction.ID,
			"decision":        "allow",
			"idempotency_key": "other-key",
		},
		"foreign execution": {
			"execution_id":    "other-execution",
			"step_id":         "one",
			"interaction_id":  interaction.ID,
			"decision":        "allow",
			"idempotency_key": interaction.ID,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, rpcErr := d.handleStepApprove(context.Background(), mustJSON(t, params))
			if rpcErr == nil || rpcErr.Code != -32002 {
				t.Fatalf("approval error=%+v, want interaction conflict", rpcErr)
			}
		})
	}
	got, err := s.Interactions().Get(context.Background(), interaction.ID)
	if err != nil || got.Status != "pending" || got.Decision != nil {
		t.Fatalf("rejected approval changed interaction=%+v err=%v", got, err)
	}

	result, rpcErr := d.handleStepApprove(context.Background(), mustJSON(t, map[string]string{
		"execution_id":    executionID,
		"step_id":         "one",
		"interaction_id":  interaction.ID,
		"decision":        "allow",
		"idempotency_key": interaction.ID,
	}))
	if rpcErr != nil || result.(stepApproveResult).Resolved != true {
		t.Fatalf("first approval result=%#v err=%+v", result, rpcErr)
	}
	result, rpcErr = d.handleStepApprove(context.Background(), mustJSON(t, map[string]string{
		"execution_id":    executionID,
		"step_id":         "one",
		"interaction_id":  interaction.ID,
		"decision":        "allow",
		"idempotency_key": interaction.ID,
	}))
	if rpcErr != nil || result.(stepApproveResult).Resolved != true {
		t.Fatalf("idempotent approval result=%#v err=%+v", result, rpcErr)
	}

	for name, decision := range map[string]string{"changed decision": "deny"} {
		t.Run(name, func(t *testing.T) {
			_, rpcErr := d.handleStepApprove(context.Background(), mustJSON(t, map[string]string{
				"execution_id":    executionID,
				"step_id":         "one",
				"interaction_id":  interaction.ID,
				"decision":        decision,
				"idempotency_key": interaction.ID,
			}))
			if rpcErr == nil || rpcErr.Code != -32002 {
				t.Fatalf("approval error=%+v, want interaction conflict", rpcErr)
			}
		})
	}
	got, err = s.Interactions().Get(context.Background(), interaction.ID)
	if err != nil || got.Decision == nil || *got.Decision != "allow" {
		t.Fatalf("interaction=%+v err=%v", got, err)
	}
}

func TestBrokerSessionHostPersistsPermissionAndWaitsForApproval(t *testing.T) {
	d, s, executionID, attemptID := newInteractionDaemon(t)
	host := NewBrokerSessionHost(s, d.sessions, attemptID)
	decisionCh := make(chan adapter.PermissionDecision, 1)
	go func() {
		decision, err := host.RequestPermission(context.Background(), adapter.PermissionRequest{
			Kind:        "permission",
			Description: "write artifact",
			Options:     []string{"allow", "deny"},
		})
		if err != nil {
			decisionCh <- adapter.PermissionDecision{Option: "error: " + err.Error()}
			return
		}
		decisionCh <- decision
	}()

	var interactionID string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		events, err := s.Events().ListAttemptEvents(context.Background(), attemptID, 0, 10)
		if err == nil && len(events) == 1 {
			var payload struct {
				InteractionID string `json:"interaction_id"`
			}
			if json.Unmarshal([]byte(*events[0].Payload), &payload) == nil {
				interactionID = payload.InteractionID
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if interactionID == "" {
		t.Fatal("permission interaction was not persisted")
	}
	result, rpcErr := d.handleStepApprove(context.Background(), mustJSON(t, map[string]string{
		"execution_id":    executionID,
		"step_id":         "one",
		"interaction_id":  interactionID,
		"decision":        "allow",
		"idempotency_key": interactionID,
	}))
	if rpcErr != nil || result.(stepApproveResult).Resolved != true {
		t.Fatalf("approval result=%#v err=%+v", result, rpcErr)
	}
	select {
	case decision := <-decisionCh:
		if decision.Option != "allow" {
			t.Fatalf("decision=%+v", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("permission request did not unblock")
	}
}

func TestStepCancelCancelsSessionAndPersistsCancelledAttempt(t *testing.T) {
	d, s, executionID, attemptID := newInteractionDaemon(t)
	session := &recordingSession{}
	d.sessions.Register(attemptID, session)
	result, rpcErr := d.handleStepCancel(context.Background(), mustJSON(t, map[string]string{
		"execution_id": executionID,
		"step_id":      "one",
	}))
	if rpcErr != nil || result.(stepCancelResult) != (stepCancelResult{}) {
		t.Fatalf("cancel result=%#v err=%+v", result, rpcErr)
	}
	attempt, err := s.Attempts().Get(context.Background(), attemptID)
	if err != nil || attempt.Status != "cancelled" {
		t.Fatalf("attempt=%+v err=%v", attempt, err)
	}
	if session.CancelCount() != 1 {
		t.Fatalf("session cancel count=%d, want one", session.CancelCount())
	}
	lease, err := s.Leases().Get(context.Background(), executionID, "one")
	if err != nil {
		t.Fatal(err)
	}
	if expiry, err := time.Parse(time.RFC3339, lease.ExpiresAt); err != nil || expiry.After(time.Now().UTC()) {
		t.Fatalf("lease expiry=%q err=%v, want expired", lease.ExpiresAt, err)
	}
	if err := d.engine.ExecuteAttempt(context.Background(), attemptID); err == nil {
		t.Fatal("cancelled attempt was executable")
	}
	// A second cancel is explicitly idempotent and does not cancel again.
	if _, rpcErr := d.handleStepCancel(context.Background(), mustJSON(t, map[string]string{"execution_id": executionID, "step_id": "one"})); rpcErr != nil {
		t.Fatalf("second cancel: %+v", rpcErr)
	}
	if session.CancelCount() != 1 {
		t.Fatalf("second cancel count=%d, want one", session.CancelCount())
	}
}

func newInteractionDaemon(t *testing.T) (*Daemon, *store.SQLiteStore, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	workflowPath := writeTestWorkflow(t, root, "interaction", `version: 2
name: interaction
steps:
  - id: one
    type: command
    run: echo one
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	d := testDaemonWithRunner(t, root, s, &execution.FakeRunner{})
	start, rpcErr := d.handleExecutionStart(context.Background(), mustJSON(t, map[string]string{"workflow_path": workflowPath}))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	executionID := start.(map[string]string)["execution_id"]
	attempt, err := d.engine.StartStep(context.Background(), executionID, "one", "headless")
	if err != nil {
		t.Fatal(err)
	}
	return d, s, executionID, attempt.ID
}

type recordingSession struct {
	mu      sync.Mutex
	cancels int
}

func (s *recordingSession) Prompt(context.Context, adapter.PromptInput) (<-chan adapter.SessionEvent, error) {
	return nil, errors.New("not used")
}
func (s *recordingSession) Cancel(context.Context) error {
	s.mu.Lock()
	s.cancels++
	s.mu.Unlock()
	return nil
}
func (s *recordingSession) LoadPrevious(context.Context, string) error {
	return adapter.ErrUnsupportedCapability
}
func (s *recordingSession) Terminal(context.Context) (adapter.TerminalHandle, error) {
	return adapter.TerminalHandle{}, adapter.ErrUnsupportedCapability
}
func (s *recordingSession) CancelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancels
}
