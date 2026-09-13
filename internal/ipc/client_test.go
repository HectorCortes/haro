package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

func TestClientMultiplexesMatchedResponsesAndNotifications(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	client := NewClient(clientConn)
	defer client.Close()
	go func() {
		br := bufio.NewReader(serverConn)
		defer serverConn.Close()
		for i := 0; i < 2; i++ {
			msg, err := jsonrpc.DecodeMessage(br)
			if err != nil {
				return
			}
			id := json.Number(msg.ID.Num)
			_ = jsonrpc.EncodeNotification(serverConn, "step.status_changed", map[string]any{"cursor": i})
			_ = jsonrpc.EncodeResponse(serverConn, id, map[string]int{"value": i + 10})
		}
	}()

	results := make(chan json.RawMessage, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			result, err := client.Call(context.Background(), "echo", map[string]int{"value": i})
			results <- result
			errs <- err
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("Call: %v", err)
		}
		var got map[string]int
		if err := json.Unmarshal(<-results, &got); err != nil || got["value"] < 10 {
			t.Fatalf("result = %v (%v)", got, err)
		}
	}
	notification := <-client.Notifications()
	if notification.Method != "step.status_changed" || notification.Params == nil {
		t.Fatalf("notification = %+v", notification)
	}
}

func TestClientReturnsRemoteErrors(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	client := NewClient(clientConn)
	defer client.Close()
	go func() {
		br := bufio.NewReader(serverConn)
		msg, err := jsonrpc.DecodeMessage(br)
		if err == nil {
			_ = jsonrpc.EncodeError(serverConn, json.Number(msg.ID.Num), jsonrpc.RPCError{Code: -32003, Message: "headless mode not supported"})
		}
		_ = serverConn.Close()
	}()
	_, err := client.Call(context.Background(), "step.run", nil)
	if err == nil || !strings.Contains(err.Error(), "-32003") {
		t.Fatalf("remote error = %v", err)
	}
}

func TestClientCallWithLauncherUsesEndpointSeam(t *testing.T) {
	if _, err := CallWithLauncher(context.Background(), nil, nil, t.TempDir(), "health", nil); err == nil {
		t.Fatal("nil transport should fail before attempting a call")
	}
}
