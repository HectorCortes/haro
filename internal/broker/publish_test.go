package broker

import (
	"context"
	"errors"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
)

func TestEventSinkRollsBackBeforeCommit(t *testing.T) {
	ctx := context.Background()
	s, attemptID := eventSinkFixture(t)
	sink := NewEventSink(s, NewHub())
	sink.SetFailpoints(EventSinkFailpoints{BeforeCommit: func() error { return errors.New("crash before commit") }})

	err := sink.Commit(ctx, func(tx store.Store) (EventNotification, error) {
		cursor, err := tx.Events().NextAttemptCursor(ctx, attemptID)
		if err != nil {
			return EventNotification{}, err
		}
		payload := "not-visible"
		if err := tx.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attemptID, Cursor: cursor, EventType: "output_delta", Payload: &payload}); err != nil {
			return EventNotification{}, err
		}
		return EventNotification{Method: "step.status_changed"}, nil
	})
	if err == nil {
		t.Fatal("expected pre-commit failpoint error")
	}
	events, err := s.Events().ListAttemptEvents(ctx, attemptID, 0, 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("pre-commit rollback left %d visible events", len(events))
	}
}

func TestEventSinkPostCommitFailureIsRecoverableByPolling(t *testing.T) {
	ctx := context.Background()
	s, attemptID := eventSinkFixture(t)
	received := 0
	hub := NewHub()
	hub.Subscribe(func(string, any) error {
		received++
		return nil
	})
	sink := NewEventSink(s, hub)
	sink.SetFailpoints(EventSinkFailpoints{AfterCommit: func() error { return errors.New("crash before fanout") }})

	err := sink.Commit(ctx, func(tx store.Store) (EventNotification, error) {
		cursor, err := tx.Events().NextAttemptCursor(ctx, attemptID)
		if err != nil {
			return EventNotification{}, err
		}
		payload := "durable"
		if err := tx.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attemptID, Cursor: cursor, EventType: "output_delta", Payload: &payload}); err != nil {
			return EventNotification{}, err
		}
		return EventNotification{Method: "step.status_changed", Params: map[string]any{"cursor": cursor}}, nil
	})
	if err == nil {
		t.Fatal("expected post-commit failpoint error")
	}
	if received != 0 {
		t.Fatalf("post-commit failure fanned out %d notifications", received)
	}
	events, err := s.Events().ListAttemptEvents(ctx, attemptID, 0, 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Cursor != 0 {
		t.Fatalf("polling recovered events = %+v", events)
	}
}

func TestEventSinkCommitsBeforeFanout(t *testing.T) {
	ctx := context.Background()
	s, attemptID := eventSinkFixture(t)
	hub := NewHub()
	seen := false
	hub.Subscribe(func(method string, params any) error {
		if method != "step.status_changed" {
			return errors.New("unexpected method")
		}
		seenEvents, err := s.Events().ListAttemptEvents(ctx, attemptID, 0, 128)
		if err != nil {
			return err
		}
		seen = len(seenEvents) == 1
		return nil
	})
	sink := NewEventSink(s, hub)
	if err := sink.Commit(ctx, func(tx store.Store) (EventNotification, error) {
		cursor, err := tx.Events().NextAttemptCursor(ctx, attemptID)
		if err != nil {
			return EventNotification{}, err
		}
		payload := "ordered"
		if err := tx.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: attemptID, Cursor: cursor, EventType: "output_delta", Payload: &payload}); err != nil {
			return EventNotification{}, err
		}
		return EventNotification{Method: "step.status_changed", Params: map[string]any{"cursor": cursor}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("fanout occurred before committed event was visible")
	}
}

func eventSinkFixture(t *testing.T) (*store.FakeStore, string) {
	t.Helper()
	ctx := context.Background()
	s := store.NewFakeStore()
	if err := s.Projects().Create(ctx, "project", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := s.Executions().Create(ctx, &store.Execution{ID: "execution", ProjectID: "project", Status: "running", WorkspaceMode: "shared"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Steps().Create(ctx, &store.ExecutionStep{ExecutionID: "execution", StepID: "step", Type: "command", Status: "running", WorkspaceMode: "shared"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Generations().Create(ctx, &store.Generation{ID: "generation", ExecutionID: "execution", StepID: "step", Number: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "attempt", ExecutionID: "execution", StepID: "step", GenerationID: "generation", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	return s, "attempt"
}
