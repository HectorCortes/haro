package broker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

func TestDecodeParamsFailsClosedBeforeHandlerEffects(t *testing.T) {
	type params struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name  string
		raw   string
		want  string
		field string
	}{
		{name: "unknown field", raw: `{"name":"ok","extra":true}`, want: "unknown field", field: "extra"},
		{name: "missing field", raw: `{}`, want: "missing field", field: "name"},
		{name: "trailing data", raw: `{"name":"ok"}{"name":"again"}`, want: "trailing data", field: "params"},
		{name: "non object", raw: `[]`, want: "params must be an object", field: "params"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var got params
			err := DecodeParams(json.RawMessage(tt.raw), &got, "name")
			var pe *ParamError
			if !errors.As(err, &pe) {
				t.Fatalf("DecodeParams error = %v, want ParamError", err)
			}
			if pe.Message != tt.want || pe.Field != tt.field {
				t.Fatalf("ParamError = %+v, want message=%q field=%q", pe, tt.want, tt.field)
			}
			if got.Name != "" {
				t.Fatalf("invalid params mutated destination: %+v", got)
			}
		})
	}

	var effects int
	d := NewDispatcher()
	d.Register("validated", func(_ context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
		var p params
		if err := DecodeParams(raw, &p, "name"); err != nil {
			return nil, invalidParamsError(err)
		}
		effects++
		return p, nil
	})
	_, rpcErr := d.Dispatch(context.Background(), "validated", json.RawMessage(`{"name":"ok","unknown":1}`))
	if rpcErr == nil || rpcErr.Code != jsonrpc.InvalidParamsCode || effects != 0 {
		t.Fatalf("invalid dispatch = %+v effects=%d, want -32602 and zero effects", rpcErr, effects)
	}
}

func TestValidateEnumRejectsUnknownValues(t *testing.T) {
	if err := ValidateEnum("allow", "decision", "allow", "deny"); err != nil {
		t.Fatalf("allowed enum rejected: %v", err)
	}
	err := ValidateEnum("later", "decision", "allow", "deny")
	var pe *ParamError
	if !errors.As(err, &pe) || pe.Message != "invalid enum" || pe.Field != "decision" {
		t.Fatalf("invalid enum = %+v, want stable boundary error", err)
	}
}
