package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/google/uuid"
)

type attemptSession struct {
	cancel  context.CancelFunc
	session adapter.Session
}

type interactionWaiter struct {
	attemptID string
	result    chan interactionResult
}

type interactionResult struct {
	decision adapter.PermissionDecision
	err      error
}

// SessionRegistry owns cancelable broker attempts and pending permission waits.
// It is deliberately independent from the adapter manager so headless command
// attempts and future supervised adapter sessions share one cancellation path.
type SessionRegistry struct {
	mu           sync.Mutex
	attempts     map[string]attemptSession
	interactions map[string]interactionWaiter
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		attempts:     make(map[string]attemptSession),
		interactions: make(map[string]interactionWaiter),
	}
}

// Register records an adapter session for an attempt.
func (r *SessionRegistry) Register(attemptID string, session adapter.Session) {
	if r == nil || attemptID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts[attemptID] = attemptSession{session: session}
}

// RegisterContext records the cancellation function for a headless attempt.
func (r *SessionRegistry) RegisterContext(attemptID string, cancel context.CancelFunc) {
	if r == nil || attemptID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts[attemptID] = attemptSession{cancel: cancel}
}

func (r *SessionRegistry) Remove(attemptID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.attempts, attemptID)
	r.mu.Unlock()
}

func (r *SessionRegistry) Cancel(ctx context.Context, attemptID string) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	control, ok := r.attempts[attemptID]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	if control.cancel != nil {
		control.cancel()
	}
	if control.session != nil {
		return control.session.Cancel(ctx)
	}
	return nil
}

func (r *SessionRegistry) CancelAll(ctx context.Context) {
	if r == nil {
		return
	}
	r.mu.Lock()
	attemptIDs := make([]string, 0, len(r.attempts))
	for attemptID := range r.attempts {
		attemptIDs = append(attemptIDs, attemptID)
	}
	r.mu.Unlock()
	for _, attemptID := range attemptIDs {
		_ = r.Cancel(ctx, attemptID)
	}
}

func (r *SessionRegistry) BeginInteraction(interactionID, attemptID string) <-chan interactionResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan interactionResult, 1)
	r.interactions[interactionID] = interactionWaiter{attemptID: attemptID, result: ch}
	return ch
}

func (r *SessionRegistry) WaitInteraction(ctx context.Context, interactionID string, ch <-chan interactionResult) (adapter.PermissionDecision, error) {
	select {
	case result := <-ch:
		return result.decision, result.err
	case <-ctx.Done():
		return adapter.PermissionDecision{}, ctx.Err()
	}
}

func (r *SessionRegistry) ResolveInteraction(interactionID string, decision adapter.PermissionDecision) {
	if r == nil {
		return
	}
	r.mu.Lock()
	waiter, ok := r.interactions[interactionID]
	if ok {
		delete(r.interactions, interactionID)
	}
	r.mu.Unlock()
	if ok {
		waiter.result <- interactionResult{decision: decision}
	}
}

func (r *SessionRegistry) ForgetInteraction(interactionID string, ch <-chan interactionResult) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	waiter, ok := r.interactions[interactionID]
	if ok && waiter.result == ch {
		delete(r.interactions, interactionID)
	}
}

// BrokerSessionHost persists a bounded permission interaction and waits for
// step.approve to resolve it. The attempt ID binds the host to one session.
type BrokerSessionHost struct {
	store     store.Store
	registry  *SessionRegistry
	attemptID string
	hub       *Hub
	sink      *EventSink
}

func NewBrokerSessionHost(s store.Store, registry *SessionRegistry, attemptID ...string) *BrokerSessionHost {
	host := &BrokerSessionHost{store: s, registry: registry, sink: NewEventSink(s, nil)}
	if len(attemptID) > 0 {
		host.attemptID = attemptID[0]
	}
	return host
}

func NewBrokerSessionHostWithHub(s store.Store, registry *SessionRegistry, hub *Hub, attemptID string) *BrokerSessionHost {
	return &BrokerSessionHost{store: s, registry: registry, hub: hub, sink: NewEventSink(s, hub), attemptID: attemptID}
}

func (h *BrokerSessionHost) SetHub(hub *Hub) {
	if h != nil {
		h.hub = hub
		if h.sink == nil {
			h.sink = NewEventSink(h.store, hub)
		} else {
			h.sink.SetHub(hub)
		}
	}
}

type interactionRequiredPayload struct {
	InteractionID      string   `json:"interaction_id"`
	Kind               string   `json:"kind"`
	Description        string   `json:"description"`
	Options            []string `json:"options"`
	AvailableDecisions []string `json:"available_decisions"`
}

func (h *BrokerSessionHost) RequestPermission(ctx context.Context, req adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	if h == nil || h.store == nil || h.registry == nil || h.attemptID == "" {
		return adapter.PermissionDecision{}, errors.New("permission host is not bound to an attempt")
	}
	attempt, err := h.store.Attempts().Get(ctx, h.attemptID)
	if err != nil {
		return adapter.PermissionDecision{}, err
	}
	if attempt.Status != "running" {
		return adapter.PermissionDecision{}, fmt.Errorf("attempt %q is not running", h.attemptID)
	}
	if len(req.Options) == 0 {
		return adapter.PermissionDecision{}, errors.New("permission request has no decisions")
	}
	interactionID := uuid.NewString()
	payload, err := boundedInteractionPayload(interactionRequiredPayload{
		InteractionID:      interactionID,
		Kind:               req.Kind,
		Description:        req.Description,
		Options:            append([]string(nil), req.Options...),
		AvailableDecisions: append([]string(nil), req.Options...),
	})
	if err != nil {
		return adapter.PermissionDecision{}, err
	}
	wait := h.registry.BeginInteraction(interactionID, h.attemptID)
	interaction := &store.Interaction{ID: interactionID, AttemptID: h.attemptID, Type: "permission", Status: "pending", IdempotencyKey: interactionID}
	var eventCursor int
	sink := h.sink
	if sink == nil {
		sink = NewEventSink(h.store, h.hub)
	}
	if err := sink.Commit(ctx, func(tx store.Store) (EventNotification, error) {
		if err := tx.Interactions().Create(ctx, interaction); err != nil {
			return EventNotification{}, err
		}
		cursor, err := tx.Events().NextAttemptCursor(ctx, h.attemptID)
		if err != nil {
			return EventNotification{}, err
		}
		eventCursor = cursor
		if err := tx.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{
			AttemptID: h.attemptID,
			Cursor:    cursor,
			EventType: "interaction_required",
			Payload:   &payload,
		}); err != nil {
			return EventNotification{}, err
		}
		return EventNotification{
			Method: "step.interaction_required",
			Params: map[string]any{
				"execution_id":   attempt.ExecutionID,
				"step_id":        attempt.StepID,
				"attempt_id":     h.attemptID,
				"interaction_id": interactionID,
				"kind":           req.Kind,
				"description":    execution.Redact(req.Description),
				"options":        req.Options,
				"cursor":         eventCursor,
			},
		}, nil
	}); err != nil {
		return adapter.PermissionDecision{}, err
	}
	decision, err := h.registry.WaitInteraction(ctx, interactionID, wait)
	if err != nil {
		h.registry.ForgetInteraction(interactionID, wait)
	}
	return decision, err
}

func boundedInteractionPayload(payload interactionRequiredPayload) (string, error) {
	payload.Description = execution.Redact(payload.Description)
	for i := range payload.Options {
		payload.Options[i] = execution.Redact(payload.Options[i])
		payload.AvailableDecisions[i] = payload.Options[i]
	}
	const max = execution.VisibleLimit
	encode := func(description string) ([]byte, error) {
		payload.Description = description
		return json.Marshal(payload)
	}
	encoded, err := encode(payload.Description)
	if err != nil {
		return "", err
	}
	if len(encoded) <= max {
		return string(encoded), nil
	}
	low, high := 0, len(payload.Description)
	best := ""
	for low <= high {
		mid := low + (high-low)/2
		encoded, err := encode(payload.Description[:mid])
		if err != nil {
			return "", err
		}
		if len(encoded) <= max {
			best = string(encoded)
			low = mid + 1
			continue
		}
		high = mid - 1
	}
	if best != "" {
		return best, nil
	}
	return "", fmt.Errorf("permission request exceeds %d byte payload limit", max)
}
