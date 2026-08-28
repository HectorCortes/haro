package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthRoundtrip(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "h.sock")

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() {
		l.Close()
		// Remove is not strictly needed after Close, but ensures cleanup.
		// net.Listen creates a socket file; we remove it.
		// Error is ignored if already removed.
		_ = removeSocket(sock)
	})

	// Accept and handle one connection.
	done := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			done <- err
			return
		}
		done <- HandleConn(conn)
	}()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := Request{JSONRPC: "2.0", Method: "health", ID: 1}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JSONRPC != "2.0" || resp.ID != 1 || resp.Result == nil || !resp.Result.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if err := <-done; err != nil {
		t.Fatalf("HandleConn: %v", err)
	}
}

// TestHealthDecodeError ensures decode failures return errors.
func TestHealthDecodeError(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "h.sock")

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() {
		l.Close()
		_ = removeSocket(sock)
	})

	done := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			done <- err
			return
		}
		done <- HandleConn(conn)
	}()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON.
	if _, err := conn.Write([]byte("not json\n")); err != nil {
		t.Fatalf("write invalid: %v", err)
	}

	// Handler should return error.
	if err := <-done; err == nil {
		t.Fatalf("expected decode error, got nil")
	}
	// Ensure socket is still cleaned up via t.Cleanup.
}

func TestHealthUnsupportedMethod(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "h.sock")

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() {
		l.Close()
		_ = removeSocket(sock)
	})

	done := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			done <- err
			return
		}
		done <- HandleConn(conn)
	}()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := Request{JSONRPC: "2.0", Method: "unknown", ID: 1}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	if err := <-done; err == nil {
		t.Fatalf("expected unsupported method error, got nil")
	}
}

func removeSocket(path string) error {
	return os.Remove(path)
}
