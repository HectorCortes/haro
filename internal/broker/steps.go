package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

func (d *Daemon) registerStepHandlers() {
	if d.rpc == nil {
		return
	}
	d.rpc.Dispatcher().Register("step.run", d.handleStepRun)
	d.rpc.Dispatcher().Register("step.events", d.handleStepEvents)
}

func (d *Daemon) handleStepRun(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params stepRunParams
	if err := DecodeParams(raw, &params, "execution_id", "step_id"); err != nil {
		return nil, invalidParamsError(err)
	}
	if d.engine == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	attempt, err := d.engine.StartStep(ctx, params.ExecutionID, params.StepID, params.Mode)
	if err != nil {
		return nil, stepRPCError(err)
	}
	go func() {
		_ = d.engine.ExecuteAttempt(context.Background(), attempt.ID)
	}()
	return stepRunResult{AttemptID: attempt.ID, NextCursor: 0}, nil
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
	return &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": fmt.Sprint(err)}}
}
