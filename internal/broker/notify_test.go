package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
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
