package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/store"
)

func TestHubBroadcastsCursorBearingNotificationsInOrder(t *testing.T) {
	hub := NewHub()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	server := NewServer(NewDispatcher())
	server.Dispatcher().Register("health", func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
		return map[string]bool{"ok": true}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.ServeConn(ctx, serverConn, hub) }()

	if err := jsonrpc.EncodeRequest(clientConn, 1, "health", nil); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(clientConn)
	if _, err := jsonrpc.DecodeMessage(reader); err != nil {
		t.Fatal(err)
	}
	messageCh := make(chan *jsonrpc.Message, 2)
	go func() {
		for i := 0; i < 2; i++ {
			msg, err := jsonrpc.DecodeMessage(reader)
			if err != nil {
				return
			}
			messageCh <- msg
		}
	}()
	hub.Broadcast("step.status_changed", map[string]any{"cursor": 1, "to": "running"})
	hub.Broadcast("step.interaction_required", map[string]any{"cursor": 2, "interaction_id": "i-1"})
	for _, want := range []string{"step.status_changed", "step.interaction_required"} {
		select {
		case msg := <-messageCh:
			if msg == nil || msg.Method != want || msg.ID != nil {
				t.Fatalf("notification=%+v, want %s without id", msg, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}

func TestSubscribedServerReceivesPersistedStatusAndInteractionNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()
	s, err := store.Open(ctx, filepath.Join(root, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, root, root); err != nil {
		t.Fatal(err)
	}
	if err := s.Executions().Create(ctx, &store.Execution{ID: "exec-notify", ProjectID: root, Status: "running", WorkspaceMode: "shared", WorkspaceRoot: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.Steps().Create(ctx, &store.ExecutionStep{ExecutionID: "exec-notify", StepID: "step-notify", Type: "command", Status: "running", WorkspaceMode: "shared"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Generations().Create(ctx, &store.Generation{ID: "gen-notify", ExecutionID: "exec-notify", StepID: "step-notify", Number: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "attempt-notify", ExecutionID: "exec-notify", StepID: "step-notify", GenerationID: "gen-notify", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Events().CreateTransition(ctx, &store.StepTransitionEvent{ExecutionID: "exec-notify", StepID: "step-notify", Cursor: 0, FromStatus: stringPtr("pending"), ToStatus: "running"}); err != nil {
		t.Fatal(err)
	}

	hub := NewHub()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	server := NewServer(NewDispatcher())
	server.Dispatcher().Register("health", func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
		return map[string]bool{"ok": true}, nil
	})
	done := make(chan error, 1)
	go func() { done <- server.ServeConn(ctx, serverConn, hub) }()
	if err := jsonrpc.EncodeRequest(clientConn, 1, "health", nil); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(clientConn)
	if msg, err := jsonrpc.DecodeMessage(reader); err != nil || msg.Error != nil {
		t.Fatalf("health response = %+v, err=%v", msg, err)
	}
	notifications := make(chan *jsonrpc.Message, 2)
	go func() {
		for i := 0; i < 2; i++ {
			msg, err := jsonrpc.DecodeMessage(reader)
			if err != nil {
				return
			}
			notifications <- msg
		}
	}()

	d := &Daemon{st: s, hub: hub, runtime: NewRuntime(s, hub)}
	statusDone := make(chan struct{})
	go func() {
		d.broadcastStepStatus(ctx, "exec-notify", "step-notify", "pending", "running")
		close(statusDone)
	}()
	statusMsg := receiveNotification(t, notifications)
	<-statusDone
	if statusMsg.Method != "step.status_changed" {
		t.Fatalf("first notification method = %q, want step.status_changed", statusMsg.Method)
	}
	var statusParams struct {
		Cursor int `json:"cursor"`
	}
	if err := json.Unmarshal(statusMsg.Params, &statusParams); err != nil || statusParams.Cursor != 0 {
		t.Fatalf("status notification params = %s, err=%v", statusMsg.Params, err)
	}

	registry := NewSessionRegistry()
	host := NewBrokerSessionHostWithHub(s, registry, hub, "attempt-notify")
	permissionDone := make(chan error, 1)
	go func() {
		_, requestErr := host.RequestPermission(ctx, adapter.PermissionRequest{Kind: "permission", Description: "allow change", Options: []string{"allow"}})
		permissionDone <- requestErr
	}()
	interactionMsg := receiveNotification(t, notifications)
	if interactionMsg.Method != "step.interaction_required" {
		t.Fatalf("second notification method = %q, want step.interaction_required", interactionMsg.Method)
	}
	var interactionParams struct {
		ExecutionID   string `json:"execution_id"`
		StepID        string `json:"step_id"`
		InteractionID string `json:"interaction_id"`
		Cursor        int    `json:"cursor"`
	}
	if err := json.Unmarshal(interactionMsg.Params, &interactionParams); err != nil || interactionParams.ExecutionID != "exec-notify" || interactionParams.StepID != "step-notify" || interactionParams.InteractionID == "" || interactionParams.Cursor < statusParams.Cursor {
		t.Fatalf("interaction notification params = %s, err=%v", interactionMsg.Params, err)
	}
	registry.ResolveInteraction(interactionParams.InteractionID, adapter.PermissionDecision{Option: "allow"})
	if err := <-permissionDone; err != nil {
		t.Fatalf("permission request: %v", err)
	}

	events, err := s.Events().ListAttemptEvents(ctx, "attempt-notify", 0, 10)
	if err != nil || len(events) != 1 || events[0].EventType != "interaction_required" {
		t.Fatalf("persisted interaction events = %+v, err=%v", events, err)
	}
	_ = clientConn.Close()
	if err := <-done; err != nil {
		t.Fatalf("ServeConn: %v", err)
	}
}

func receiveNotification(t *testing.T, notifications <-chan *jsonrpc.Message) *jsonrpc.Message {
	t.Helper()
	select {
	case msg := <-notifications:
		if msg == nil || msg.ID != nil {
			t.Fatalf("invalid notification = %+v", msg)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
		return nil
	}
}

func stringPtr(value string) *string { return &value }
