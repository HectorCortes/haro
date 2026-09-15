package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/store"
)

type stepRunParams struct {
	ExecutionID string `json:"execution_id"`
	StepID      string `json:"step_id"`
	Mode        string `json:"mode,omitempty"`
}

type stepRunResult struct {
	AttemptID  string `json:"attempt_id"`
	NextCursor int    `json:"next_cursor"`
}

type stepEventsParams struct {
	ExecutionID string `json:"execution_id"`
	StepID      string `json:"step_id"`
	SinceCursor int    `json:"since_cursor"`
}

type stepEventResult struct {
	Cursor     int    `json:"cursor"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload,omitempty"`
	PayloadRef string `json:"payload_ref,omitempty"`
}

type stepEventsResult struct {
	Events     []stepEventResult `json:"events"`
	NextCursor int               `json:"next_cursor"`
}

type stepApproveParams struct {
	ExecutionID    string `json:"execution_id"`
	StepID         string `json:"step_id"`
	InteractionID  string `json:"interaction_id"`
	Decision       string `json:"decision"`
	IdempotencyKey string `json:"idempotency_key"`
}

type stepApproveResult struct {
	Resolved bool `json:"resolved"`
}

type stepCancelParams struct {
	ExecutionID string `json:"execution_id"`
	StepID      string `json:"step_id"`
}

type stepCancelResult struct{}

type stepReopenParams struct {
	ExecutionID string `json:"execution_id"`
	StepID      string `json:"step_id"`
	Cascade     *bool  `json:"cascade,omitempty"`
	Feedback    string `json:"feedback,omitempty"`
}

type stepReopenResult struct {
	Invalidated []string `json:"invalidated"`
}

func (d *Daemon) registerStepHandlers() {
	if d.rpc == nil {
		return
	}
	d.rpc.Dispatcher().Register("step.run", d.handleStepRun)
	d.rpc.Dispatcher().Register("step.events", d.handleStepEvents)
	d.rpc.Dispatcher().Register("step.approve", d.handleStepApprove)
	d.rpc.Dispatcher().Register("step.cancel", d.handleStepCancel)
	d.rpc.Dispatcher().Register("step.reopen", d.handleStepReopen)
}

func (d *Daemon) handleStepRun(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepRunParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id"); err != nil {
		return nil, invalidParamsError(err)
	}
	if err := d.rejectDraining(); err != nil {
		return nil, err
	}
	if d.engine == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	attempt, err := d.engine.StartStep(ctx, params.ExecutionID, params.StepID, params.Mode)
	if err != nil {
		return nil, stepRPCError(err)
	}
	attemptCtx, cancel := context.WithCancel(ctx)
	if d.sessions == nil {
		d.sessions = NewSessionRegistry()
	}
	d.broadcastStepStatus(ctx, params.ExecutionID, params.StepID, "pending", "running")
	d.sessions.RegisterContext(attempt.ID, cancel)
	if !d.addWorker() {
		cancel()
		d.sessions.Remove(attempt.ID)
		_ = d.engine.CancelAttempt(context.WithoutCancel(ctx), attempt.ID)
		return nil, &jsonrpc.RPCError{Code: -32001, Message: "broker is draining; reopen and rerun"}
	}
	go func() {
		defer d.wg.Done()
		defer cancel()
		defer d.sessions.Remove(attempt.ID)
		_ = d.engine.ExecuteAttempt(attemptCtx, attempt.ID)
		if d.st != nil {
			if step, err := d.st.Steps().Get(context.WithoutCancel(attemptCtx), attempt.ExecutionID, attempt.StepID); err == nil && step.Status != "running" {
				d.broadcastStepStatus(context.WithoutCancel(attemptCtx), attempt.ExecutionID, attempt.StepID, "running", step.Status)
			}
		}
	}()
	return stepRunResult{AttemptID: attempt.ID, NextCursor: 0}, nil
}

func (d *Daemon) handleStepApprove(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepApproveParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id", "interaction_id", "decision", "idempotency_key"); err != nil {
		return nil, invalidParamsError(err)
	}
	if err := d.rejectDraining(); err != nil {
		return nil, err
	}
	if d.st == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	interaction, err := d.st.Interactions().Get(ctx, params.InteractionID)
	if err != nil {
		return nil, interactionConflictError()
	}
	current, err := d.st.Attempts().CurrentByStep(ctx, params.ExecutionID, params.StepID)
	if err != nil || current.ID != interaction.AttemptID || current.Status != "running" {
		return nil, interactionConflictError()
	}
	if params.IdempotencyKey != interaction.ID || params.IdempotencyKey != interaction.IdempotencyKey {
		return nil, interactionConflictError()
	}
	available, err := availableDecisions(ctx, d.st, interaction.AttemptID, interaction.ID)
	if err != nil || !contains(available, params.Decision) {
		return nil, interactionConflictError()
	}
	if interaction.Status == "resolved" && (interaction.Decision == nil || *interaction.Decision != params.Decision) {
		return nil, interactionConflictError()
	}
	resolved, err := d.st.Interactions().Resolve(ctx, interaction.ID, params.IdempotencyKey, params.Decision)
	if err != nil {
		return nil, interactionConflictError()
	}
	if d.sessions != nil {
		d.sessions.ResolveInteraction(interaction.ID, adapter.PermissionDecision{Option: params.Decision})
	}
	return stepApproveResult{Resolved: resolved.Status == "resolved"}, nil
}

func (d *Daemon) handleStepCancel(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepCancelParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id"); err != nil {
		return nil, invalidParamsError(err)
	}
	if err := d.rejectDraining(); err != nil {
		return nil, err
	}
	if d.st == nil || d.engine == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	attempt, err := d.st.Attempts().CurrentByStep(ctx, params.ExecutionID, params.StepID)
	if errors.Is(err, store.ErrNotFound) {
		return stepCancelResult{}, nil
	}
	if err != nil {
		return nil, referenceRPCError(err, "step_id")
	}
	if attempt.Status != "running" {
		return stepCancelResult{}, nil
	}
	if d.sessions != nil {
		if err := d.sessions.Cancel(ctx, attempt.ID); err != nil {
			return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": err.Error()}}
		}
	}
	if err := d.engine.CancelAttempt(ctx, attempt.ID); err != nil {
		return nil, stepRPCError(err)
	}
	d.broadcastStepStatus(ctx, params.ExecutionID, params.StepID, "running", "failed")
	return stepCancelResult{}, nil
}

func (d *Daemon) handleStepReopen(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepReopenParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id"); err != nil {
		return nil, invalidParamsError(err)
	}
	if err := d.rejectDraining(); err != nil {
		return nil, err
	}
	if len(params.Feedback) > execution.FallbackLimit {
		return nil, invalidParamsError(&ParamError{Message: "value exceeds limit", Field: "feedback"})
	}
	if d.st == nil || d.engine == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	steps, err := d.st.Steps().List(ctx, params.ExecutionID)
	if err != nil {
		return nil, referenceRPCError(err, "execution_id")
	}
	beforeStatus := make(map[string]string, len(steps))
	beforeInvalidated := make(map[string]map[string]bool, len(steps))
	for _, step := range steps {
		beforeStatus[step.StepID] = step.Status
		generations, err := d.st.Generations().ListByStep(ctx, params.ExecutionID, step.StepID)
		if err != nil {
			return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
		}
		beforeInvalidated[step.StepID] = make(map[string]bool, len(generations))
		for _, generation := range generations {
			beforeInvalidated[step.StepID][generation.ID] = generation.InvalidatedAt != nil
		}
	}
	cascade := true
	if params.Cascade != nil {
		cascade = *params.Cascade
	}
	if err := d.engine.ReopenStep(ctx, params.ExecutionID, params.StepID, cascade, params.Feedback); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, referenceRPCError(err, "step_id")
		}
		if strings.Contains(err.Error(), "feedback exceeds") {
			return nil, invalidParamsError(&ParamError{Message: "value exceeds limit", Field: "feedback"})
		}
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": err.Error()}}
	}
	after, err := d.st.Steps().List(ctx, params.ExecutionID)
	if err != nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	invalidatedSet := map[string]bool{}
	for _, step := range after {
		if step.StepID == params.StepID && beforeStatus[step.StepID] != "pending" && step.Status == "pending" {
			invalidatedSet[step.StepID] = true
		}
		generations, err := d.st.Generations().ListByStep(ctx, params.ExecutionID, step.StepID)
		if err != nil {
			return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
		}
		for _, generation := range generations {
			if generation.InvalidatedAt != nil && !beforeInvalidated[step.StepID][generation.ID] && generation.InvalidatedByStep != nil && *generation.InvalidatedByStep == params.StepID {
				invalidatedSet[step.StepID] = true
			}
		}
	}
	invalidated := make([]string, 0, len(invalidatedSet))
	for stepID := range invalidatedSet {
		invalidated = append(invalidated, stepID)
	}
	sort.Strings(invalidated)
	for _, stepID := range invalidated {
		d.broadcastStepStatus(ctx, params.ExecutionID, stepID, "completed", "pending")
	}
	return stepReopenResult{Invalidated: invalidated}, nil
}

func (d *Daemon) broadcastStepStatus(ctx context.Context, executionID, stepID, from, to string) {
	if d == nil || d.hub == nil || d.st == nil {
		return
	}
	transitions, err := d.st.Events().ListTransitions(ctx, executionID, stepID)
	if err != nil || len(transitions) == 0 {
		return
	}
	transition := transitions[len(transitions)-1]
	if transition.ToStatus != to {
		return
	}
	notification := EventNotification{
		Method: "step.status_changed",
		Params: map[string]any{
			"execution_id": executionID,
			"step_id":      stepID,
			"from":         from,
			"to":           to,
			"cursor":       transition.Cursor,
		},
	}
	if d.runtime != nil && d.runtime.EventSink != nil {
		// The transition was committed by the engine before this call. Routing
		// fanout through EventSink preserves commit-before-visible ordering and
		// serializes status notifications with interaction publications.
		_ = d.runtime.EventSink.Commit(ctx, func(store.Store) (EventNotification, error) {
			return notification, nil
		})
		return
	}
	d.hub.Broadcast(notification.Method, notification.Params)
}

func availableDecisions(ctx context.Context, s store.Store, attemptID, interactionID string) ([]string, error) {
	since := 0
	for {
		events, err := s.Events().ListAttemptEvents(ctx, attemptID, since, 128)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if event.EventType != "interaction_required" || event.Payload == nil {
				continue
			}
			var payload struct {
				InteractionID      string   `json:"interaction_id"`
				AvailableDecisions []string `json:"available_decisions"`
			}
			if err := json.Unmarshal([]byte(*event.Payload), &payload); err == nil && payload.InteractionID == interactionID {
				return payload.AvailableDecisions, nil
			}
		}
		if len(events) == 0 {
			break
		}
		next := events[len(events)-1].Cursor + 1
		if next <= since {
			break
		}
		since = next
	}
	return nil, store.ErrNotFound
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func interactionConflictError() *jsonrpc.RPCError {
	return &jsonrpc.RPCError{Code: -32002, Message: "interaction conflict"}
}

func (d *Daemon) rejectDraining() *jsonrpc.RPCError {
	if d != nil && (d.State() == StateDraining || d.State() == StateStopped) {
		return &jsonrpc.RPCError{Code: -32001, Message: "broker is draining; reopen and rerun"}
	}
	return nil
}

func (d *Daemon) handleStepEvents(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepEventsParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id", "since_cursor"); err != nil {
		return nil, invalidParamsError(err)
	}
	if params.SinceCursor < 0 {
		return nil, invalidParamsError(&ParamError{Message: "invalid value", Field: "since_cursor"})
	}
	if d.st == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	attempt, err := d.st.Attempts().CurrentByStep(ctx, params.ExecutionID, params.StepID)
	if err != nil {
		return nil, referenceRPCError(err, "step_id")
	}
	events, err := d.st.Events().ListAttemptEvents(ctx, attempt.ID, params.SinceCursor, 128)
	if err != nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	result := stepEventsResult{Events: make([]stepEventResult, 0, len(events)), NextCursor: params.SinceCursor}
	for _, event := range events {
		wire := stepEventResult{Cursor: event.Cursor, EventType: event.EventType}
		if event.Payload != nil {
			wire.Payload = execution.VisibleEvidence(*event.Payload)
		} else if event.PayloadRef != nil {
			wire.PayloadRef = *event.PayloadRef
		}
		result.Events = append(result.Events, wire)
		if event.Cursor >= result.NextCursor {
			result.NextCursor = event.Cursor + 1
		}
	}
	return result, nil
}

func stepRPCError(err error) *jsonrpc.RPCError {
	if strings.Contains(err.Error(), "mode not supported") {
		return &jsonrpc.RPCError{Code: -32003, Message: err.Error()}
	}
	if errors.Is(err, store.ErrNotFound) {
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: "not_found"}
	}
	if strings.Contains(err.Error(), "not pending") || strings.Contains(err.Error(), "dependency") || strings.Contains(err.Error(), "requires") {
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: err.Error()}
	}
	if strings.Contains(err.Error(), "stale") || strings.Contains(err.Error(), "cancelled") {
		return &jsonrpc.RPCError{Code: -32001, Message: "stale attempt; reopen and rerun"}
	}
	return &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": fmt.Sprint(err)}}
}
