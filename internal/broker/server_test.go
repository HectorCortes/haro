package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

func TestServerRecoversMalformedAndOversizedFrames(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{name: "malformed", frame: `{"jsonrpc":"2.0","method":"health","id":1` + "\n"},
		{name: "oversized", frame: strings.Repeat("x", jsonrpc.MaxMessageSize+1) + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			s := NewServer(NewDispatcher())
			s.Dispatcher().Register("health", func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
				return map[string]bool{"ok": true}, nil
			})
			done := make(chan error, 1)
			go func() { done <- s.ServeConn(context.Background(), server) }()

			writeDone := make(chan error, 1)
			go func() {
				_, err := fmt.Fprintf(client, "%s", tt.frame)
				if err == nil {
					_, err = fmt.Fprint(client, `{"jsonrpc":"2.0","method":"health","id":2}`+"\n")
				}
				writeDone <- err
			}()
			br := bufio.NewReader(client)
			first := readRPCMessage(t, br)
			if first.Error == nil || first.Error.Code != jsonrpc.ParseErrorCode {
				t.Fatalf("first response = %+v, want parse error", first.Error)
			}
			second := readRPCMessage(t, br)
			if second.ID == nil || second.ID.Num != "2" || second.Error != nil {
				t.Fatalf("valid response = %+v, want id 2 success", second)
			}
			if err := <-writeDone; err != nil {
				t.Fatalf("write frames: %v", err)
			}
			_ = client.Close()
			if err := <-done; err != nil {
				t.Fatalf("ServeConn: %v", err)
			}
		})
	}
}

func TestServerRejectsInvalidEnvelopeAndUnknownMethod(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	s := NewServer(NewDispatcher())
	done := make(chan error, 1)
	go func() { done <- s.ServeConn(context.Background(), server) }()
	for _, frame := range []string{
		`{"jsonrpc":"1.0","method":"health","id":1}` + "\n",
		`{"jsonrpc":"2.0","id":2}` + "\n",
		`{"jsonrpc":"2.0","method":"missing","id":3}` + "\n",
	} {
		if _, err := fmt.Fprint(client, frame); err != nil {
			t.Fatal(err)
		}
		msg := readRPCMessage(t, bufio.NewReader(client))
		if msg.Error == nil {
			t.Fatalf("frame %s returned no error", frame)
		}
		want := map[string]int{"1.0": jsonrpc.InvalidRequestCode, "": jsonrpc.InvalidRequestCode, "missing": jsonrpc.MethodNotFoundCode}
		var raw struct {
			JSONRPC string `json:"jsonrpc"`
			Method  string `json:"method"`
		}
		_ = json.Unmarshal([]byte(frame), &raw)
		if msg.Error.Code != want[raw.Method] && !(raw.JSONRPC == "1.0" && msg.Error.Code == jsonrpc.InvalidRequestCode) {
			t.Fatalf("frame %s code = %d", frame, msg.Error.Code)
		}
	}
	_ = client.Close()
	if err := <-done; err != nil {
		t.Fatalf("ServeConn: %v", err)
	}
}

func TestServerMatchesConcurrentConnectionResponses(t *testing.T) {
	s := NewServer(NewDispatcher())
	s.Dispatcher().Register("echo", func(_ context.Context, params json.RawMessage) (any, *jsonrpc.RPCError) {
		var p struct {
			N int `json:"n"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: err.Error()}
		}
		return map[string]int{"n": p.N}, nil
	})
	const n = 12
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			client, server := net.Pipe()
			defer client.Close()
			go func() { _ = s.ServeConn(context.Background(), server) }()
			if _, err := fmt.Fprintf(client, `{"jsonrpc":"2.0","method":"echo","params":{"n":%d},"id":%d}`+"\n", i, i); err != nil {
				t.Errorf("write %d: %v", i, err)
				return
			}
			msg := readRPCMessage(t, bufio.NewReader(client))
			if msg.Error != nil || msg.ID == nil || msg.ID.Num != fmt.Sprint(i) {
				t.Errorf("response %d = %+v", i, msg)
				return
			}
			var result map[string]int
			if err := json.Unmarshal(msg.Result, &result); err != nil || result["n"] != i {
				t.Errorf("result %d = %s (%v)", i, msg.Result, err)
			}
			_ = client.Close()
		}(i)
	}
	wg.Wait()
}

func readRPCMessage(t *testing.T, br *bufio.Reader) *jsonrpc.Message {
	t.Helper()
	msg, err := jsonrpc.DecodeMessage(br)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return msg
}
