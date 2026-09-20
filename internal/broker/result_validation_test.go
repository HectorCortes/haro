package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

func TestDispatcherValidatesProductionResultSchemas(t *testing.T) {
	cases := []struct {
		method string
		result any
	}{
		{method: "health", result: map[string]bool{"ok": true}},
		{method: "execution.start", result: map[string]string{"execution_id": "exec-1"}},
		{method: "execution.status", result: executionStatusResult{Status: "running", Steps: []executionStatusStep{}}},
		{method: "step.run", result: stepRunResult{AttemptID: "attempt-1", NextCursor: 0}},
		{method: "step.events", result: stepEventsResult{Events: []stepEventResult{}, NextCursor: 0}},
		{method: "step.approve", result: stepApproveResult{Resolved: true}},
		{method: "step.cancel", result: stepCancelResult{}},
		{method: "step.reopen", result: stepReopenResult{Invalidated: []string{"step-1"}}},
	}
	for _, tt := range cases {
		t.Run(tt.method, func(t *testing.T) {
			d := NewDispatcher()
			d.Register(tt.method, func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
				return tt.result, nil
			})
			got, rpcErr := d.Dispatch(context.Background(), tt.method, nil)
			if rpcErr != nil {
				t.Fatalf("valid result rejected: %+v", rpcErr)
			}
			want, _ := json.Marshal(tt.result)
			actual, _ := json.Marshal(got)
			if string(actual) != string(want) {
				t.Fatalf("result changed: got %s want %s", actual, want)
			}
		})
	}
}

func TestServerRejectsInvalidOutgoingResultWithoutSendingPayload(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	var calls int
	d := NewDispatcher()
	d.Register("execution.start", func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
		calls++
		if calls == 1 {
			return map[string]any{"execution_id": "exec-1", "unexpected": true}, nil
		}
		return map[string]string{"execution_id": "exec-2"}, nil
	})
	done := make(chan error, 1)
	go func() { done <- NewServer(d).ServeConn(context.Background(), server) }()
	br := bufio.NewReader(client)

	if _, err := fmt.Fprint(client, `{"jsonrpc":"2.0","method":"execution.start","id":1}`+"\n"); err != nil {
		t.Fatal(err)
	}
	first := readRPCMessage(t, br)
	if first.Error == nil || first.Error.Code != jsonrpc.InvalidParamsCode || len(first.Result) != 0 {
		t.Fatalf("invalid result response = %+v, want -32602 error without result", first)
	}
	var data map[string]any
	dataJSON, _ := json.Marshal(first.Error.Data)
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		t.Fatalf("error data: %v", err)
	}
	if first.Error.Message != "unknown field" || data["field"] != "result.unexpected" {
		t.Fatalf("outgoing validation error = message %q data %v, want stable field path", first.Error.Message, data)
	}

	if _, err := fmt.Fprint(client, `{"jsonrpc":"2.0","method":"execution.start","id":2}`+"\n"); err != nil {
		t.Fatal(err)
	}
	second := readRPCMessage(t, br)
	if second.Error != nil || string(second.Result) != `{"execution_id":"exec-2"}` {
		t.Fatalf("valid result response = %+v, want unchanged result", second)
	}
	_ = client.Close()
	if err := <-done; err != nil {
		t.Fatalf("ServeConn: %v", err)
	}
}

func TestDispatcherRejectsMalformedOutgoingResults(t *testing.T) {
	cases := []struct {
		name   string
		method string
		result any
		field  string
		msg    string
	}{
		{name: "invalid enum", method: "execution.status", result: map[string]any{"status": "unknown", "steps": []any{}}, field: "result.status", msg: "invalid enum"},
		{name: "negative cursor", method: "step.run", result: map[string]any{"attempt_id": "attempt-1", "next_cursor": -1}, field: "result.next_cursor", msg: "invalid value"},
		{name: "missing field", method: "step.approve", result: map[string]any{}, field: "result.resolved", msg: "missing field"},
		{name: "nested unknown field", method: "step.events", result: map[string]any{"events": []any{map[string]any{"cursor": 0, "event_type": "log", "unexpected": true}}, "next_cursor": 1}, field: "result.events[0].unexpected", msg: "unknown field"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher()
			d.Register(tt.method, func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
				return tt.result, nil
			})
			_, rpcErr := d.Dispatch(context.Background(), tt.method, nil)
			if rpcErr == nil || rpcErr.Code != jsonrpc.InvalidParamsCode || rpcErr.Message != tt.msg {
				t.Fatalf("error = %+v, want -32602 %q", rpcErr, tt.msg)
			}
			data, ok := rpcErr.Data.(map[string]string)
			if !ok || data["field"] != tt.field {
				t.Fatalf("error data = %#v, want field %q", rpcErr.Data, tt.field)
			}
		})
	}
}
