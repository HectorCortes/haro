package jsonrpc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCodec_RequestIDs(t *testing.T) {
	cases := []struct {
		name string
		id   any
		json string
	}{
		{"string id", "abc-123", `{"jsonrpc":"2.0","id":"abc-123","method":"initialize","params":{"protocolVersion":1}}`},
		{"number id", json.Number("42"), `{"jsonrpc":"2.0","id":42,"method":"session/new","params":{}}`},
		{"null id", nil, `{"jsonrpc":"2.0","id":null,"method":"health","params":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			var id any
			switch v := tc.id.(type) {
			case json.Number:
				id = v
			default:
				id = tc.id
			}
			if err := EncodeRequest(&buf, id, methodFromJSON(tc.json), nil); err != nil {
				t.Fatalf("EncodeRequest: %v", err)
			}
			req, err := DecodeMessage(&buf)
			if err != nil {
				t.Fatalf("DecodeMessage: %v", err)
			}
			if req.Method != methodFromJSON(tc.json) {
				t.Fatalf("Method = %q, want %q", req.Method, methodFromJSON(tc.json))
			}
			// Check ID round-trip
			if tc.id == nil {
				if req.ID != nil && !req.ID.IsNull() {
					t.Fatalf("ID expected null, got %v", req.ID)
				}
			} else if n, ok := tc.id.(json.Number); ok {
				if req.ID == nil || req.ID.Num != n.String() {
					t.Fatalf("ID number mismatch: got %v, want %v", req.ID, n)
				}
			} else {
				if req.ID == nil || req.ID.Str != tc.id.(string) {
					t.Fatalf("ID string mismatch: got %v, want %v", req.ID, tc.id)
				}
			}
			// UseNumber: params numbers should be json.Number if present
			if tc.name == "number id" {
				// also test params number preservation
				var buf2 bytes.Buffer
				params := map[string]any{"n": json.Number("12345678901234567890")}
				if err := EncodeRequest(&buf2, json.Number("1"), "test", params); err != nil {
					t.Fatalf("EncodeRequest params: %v", err)
				}
				msg, err := DecodeMessage(&buf2)
				if err != nil {
					t.Fatalf("DecodeMessage params: %v", err)
				}
				var p map[string]json.Number
				if err := json.Unmarshal(msg.Params, &p); err != nil {
					// Try generic
					var raw map[string]any
					dec := json.NewDecoder(bytes.NewReader(msg.Params))
					dec.UseNumber()
					if err2 := dec.Decode(&raw); err2 != nil {
						t.Fatalf("params decode: %v", err)
					}
					if _, ok := raw["n"].(json.Number); !ok {
						t.Fatalf("UseNumber not preserved, got %T", raw["n"])
					}
				}
			}
		})
	}
}

func TestCodec_Response(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeResponse(&buf, "req-1", map[string]any{"ok": true}); err != nil {
		t.Fatalf("EncodeResponse: %v", err)
	}
	msg, err := DecodeMessage(&buf)
	if err != nil {
		t.Fatalf("DecodeMessage response: %v", err)
	}
	if msg.Result == nil {
		t.Fatalf("Result nil")
	}
	var res map[string]any
	if err := json.Unmarshal(msg.Result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res["ok"] != true {
		t.Fatalf("result ok = %v, want true", res["ok"])
	}
	// Triangulate with number ID
	var buf2 bytes.Buffer
	if err := EncodeResponse(&buf2, json.Number("99"), "hello"); err != nil {
		t.Fatalf("EncodeResponse number id: %v", err)
	}
	msg2, err := DecodeMessage(&buf2)
	if err != nil {
		t.Fatalf("DecodeMessage2: %v", err)
	}
	if msg2.ID == nil || msg2.ID.Num != "99" {
		t.Fatalf("response number id mismatch: %v", msg2.ID)
	}
}

func TestCodec_Error(t *testing.T) {
	var buf bytes.Buffer
	rpcErr := RPCError{Code: -32600, Message: "Invalid Request", Data: "bad"}
	if err := EncodeError(&buf, "id-err", rpcErr); err != nil {
		t.Fatalf("EncodeError: %v", err)
	}
	msg, err := DecodeMessage(&buf)
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}
	if msg.Error == nil {
		t.Fatalf("Error nil")
	}
	if msg.Error.Code != -32600 {
		t.Fatalf("code = %d, want -32600", msg.Error.Code)
	}
	if msg.Error.Message != "Invalid Request" {
		t.Fatalf("message = %q", msg.Error.Message)
	}
	// Triangulate error with null id
	var buf2 bytes.Buffer
	if err := EncodeError(&buf2, nil, RPCError{Code: -32700, Message: "Parse error"}); err != nil {
		t.Fatalf("EncodeError null: %v", err)
	}
	msg2, err := DecodeMessage(&buf2)
	if err != nil {
		t.Fatalf("DecodeMessage null err: %v", err)
	}
	if msg2.Error.Code != -32700 {
		t.Fatalf("code2 = %d", msg2.Error.Code)
	}
}

func TestCodec_Notification(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeNotification(&buf, "session/update", map[string]any{"cursor": 1}); err != nil {
		t.Fatalf("EncodeNotification: %v", err)
	}
	msg, err := DecodeMessage(&buf)
	if err != nil {
		t.Fatalf("DecodeMessage notification: %v", err)
	}
	if msg.Method != "session/update" {
		t.Fatalf("Method = %q, want session/update", msg.Method)
	}
	if msg.ID != nil {
		t.Fatalf("notification ID should be nil, got %v", msg.ID)
	}
	if msg.Params == nil {
		t.Fatalf("params nil")
	}
}

func TestCodec_MaxSize(t *testing.T) {
	// Create payload just under 10 MiB should succeed
	justUnder := MaxMessageSize - 200
	largeStr := strings.Repeat("a", justUnder)
	var buf bytes.Buffer
	if err := EncodeRequest(&buf, "id", "test", map[string]string{"data": largeStr}); err != nil {
		t.Fatalf("EncodeRequest large just under: %v", err)
	}
	if buf.Len() > MaxMessageSize {
		t.Fatalf("encoded size %d exceeds limit %d", buf.Len(), MaxMessageSize)
	}
	// Now oversize: 10 MiB + 1
	oversize := strings.Repeat("b", MaxMessageSize+1)
	var buf2 bytes.Buffer
	// Encode should fail for oversize? Or Decode should fail?
	// Design says codec rejects frames over 10 MiB with structured error.
	// We test that encoding oversize returns error
	err := EncodeRequest(&buf2, "id", "test", map[string]string{"data": oversize})
	if err == nil {
		t.Fatalf("expected error for oversize encode, got none (size %d)", buf2.Len())
	}
	if !strings.Contains(err.Error(), "too large") && !strings.Contains(err.Error(), "10") {
		t.Fatalf("oversize error = %q, want contains too large", err.Error())
	}
	// Triangulate: Decode oversize frame should also fail
	var buf3 bytes.Buffer
	// Manually write oversize JSON without using Encode (to test decode path)
	bigObj := map[string]any{"jsonrpc": "2.0", "id": "x", "method": "test", "params": map[string]string{"data": oversize}}
	data, _ := json.Marshal(bigObj)
	buf3.Write(data)
	buf3.WriteByte('\n')
	_, err = DecodeMessage(&buf3)
	if err == nil {
		t.Fatalf("expected decode error for oversize, got none")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("decode oversize error = %q, want too large", err.Error())
	}
}

func TestCodec_UseNumber(t *testing.T) {
	var buf bytes.Buffer
	// Number larger than int64
	bigNum := "9007199254740993"
	raw := `{"jsonrpc":"2.0","id":1,"method":"test","params":{"big":` + bigNum + `}}`
	buf.WriteString(raw + "\n")
	msg, err := DecodeMessage(&buf)
	if err != nil {
		t.Fatalf("DecodeMessage UseNumber: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(msg.Params))
	dec.UseNumber()
	var p map[string]json.Number
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("decode params with UseNumber: %v", err)
	}
	if p["big"].String() != bigNum {
		t.Fatalf("big number = %q, want %q", p["big"].String(), bigNum)
	}
}

func methodFromJSON(s string) string {
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	if v, ok := m["method"].(string); ok {
		return v
	}
	return ""
}
