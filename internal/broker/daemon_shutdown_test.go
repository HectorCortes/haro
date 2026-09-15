package broker

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDaemonShutdownRemovesSocketAndAllowsReplacement(t *testing.T) {
	root := t.TempDir()
	daemon, err := NewDaemon(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx) }()
	waitForEndpoint(t, daemon.Endpoint(), true)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	waitForEndpoint(t, daemon.Endpoint(), false)

	replacement, err := NewDaemon(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacementCtx, replacementCancel := context.WithCancel(context.Background())
	replacementDone := make(chan error, 1)
	go func() { replacementDone <- replacement.Run(replacementCtx) }()
	waitForEndpoint(t, replacement.Endpoint(), true)
	replacementCancel()
	if err := <-replacementDone; err != nil {
		t.Fatal(err)
	}
	waitForEndpoint(t, replacement.Endpoint(), false)
}

func TestDaemonShutdownWaitsForTrackedWorker(t *testing.T) {
	d, err := NewDaemon(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d.state.Store(StateRunning)
	if !d.addWorker() {
		t.Fatal("running daemon must accept a worker")
	}
	release := make(chan struct{})
	go func() {
		defer d.wg.Done()
		<-release
	}()

	shutdownDone := make(chan struct{})
	go func() {
		d.shutdown()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before the tracked worker exited")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wait for the tracked worker")
	}
}

func waitForEndpoint(t *testing.T, endpoint string, wantPresent bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(endpoint)
		present := err == nil
		if present == wantPresent {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, err := os.Stat(endpoint)
	t.Fatalf("endpoint %q present=%v err=%v, want %v", endpoint, err == nil, err, wantPresent)
}
