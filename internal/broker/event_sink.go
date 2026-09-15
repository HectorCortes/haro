package broker

import (
	"context"
	"errors"
	"sync"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/store"
)

// EventNotification is emitted only after the transaction that created the
// corresponding state/event rows commits.
type EventNotification struct {
	Method string
	Params any
}

// EventMutation performs the state and event writes in one transaction and
// returns the notification to fan out after commit.
type EventMutation func(store.Store) (EventNotification, error)

// EventSinkFailpoints are test-only crash windows. BeforeCommit rolls back
// the transaction; AfterCommit simulates a crash after durable commit and
// before fanout, leaving consumers to recover by polling step.events.
type EventSinkFailpoints struct {
	BeforeCommit func() error
	AfterCommit  func() error
}

// EventSink serializes persist-then-fanout publication. Keeping the mutex
// across commit and Broadcast gives notifications the same order as their
// committed cursor sequence for a project runtime.
type EventSink struct {
	store store.Store
	hub   *Hub

	mu         sync.Mutex
	failpoints EventSinkFailpoints
}

func NewEventSink(s store.Store, hub *Hub) *EventSink {
	return &EventSink{store: s, hub: hub}
}

func (s *EventSink) SetHub(hub *Hub) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.hub = hub
	s.mu.Unlock()
}

func (s *EventSink) SetFailpoints(fp EventSinkFailpoints) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.failpoints = fp
	s.mu.Unlock()
}

func (s *EventSink) Commit(ctx context.Context, mutate EventMutation) error {
	if s == nil || s.store == nil {
		return errors.New("event sink is not configured")
	}
	if mutate == nil {
		return errors.New("event mutation is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	fp := s.failpoints
	var notification EventNotification
	if err := s.store.WithTx(ctx, func(tx store.Store) error {
		var err error
		notification, err = mutate(tx)
		if err != nil {
			return err
		}
		if fp.BeforeCommit != nil {
			return fp.BeforeCommit()
		}
		return nil
	}); err != nil {
		return err
	}
	if fp.AfterCommit != nil {
		if err := fp.AfterCommit(); err != nil {
			return err
		}
	}
	if notification.Method != "" && s.hub != nil {
		s.hub.Broadcast(notification.Method, notification.Params)
	}
	return nil
}

// Runtime owns the per-project event sink used by broker handlers.
type Runtime struct {
	EventSink *EventSink
}

func NewRuntime(s store.Store, hub *Hub) *Runtime {
	return &Runtime{EventSink: NewEventSink(s, hub)}
}

// EventSinkForAttempt preserves the daemon hub while routing the transaction
// through execution.FencedStore, so a stale attempt cannot publish a write.
func (r *Runtime) EventSinkForAttempt(execID, stepID, holder string, fencingToken int64) *EventSink {
	if r == nil || r.EventSink == nil {
		return nil
	}
	return NewEventSink(execution.NewFencedStore(r.EventSink.store, execID, stepID, holder, fencingToken), r.EventSink.hub)
}
