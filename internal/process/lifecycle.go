// Package process provides bounded ownership for external process lifecycles.
package process

import (
	"context"
	"os/exec"
	"sync"
)

// Handle owns one started command and reaps it exactly once.
type Handle struct {
	cmd *exec.Cmd

	done chan struct{}

	waitOnce sync.Once
	waitMu   sync.Mutex
	waitErr  error
	waited   bool

	killOnce sync.Once
	killErr  error
}

// Start configures and starts cmd, then watches ctx for cancellation. A
// cancellation terminates the process according to the current platform's
// process-tree policy; the caller remains responsible for calling Wait.
func Start(ctx context.Context, cmd *exec.Cmd) (*Handle, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	configure(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	h := &Handle{cmd: cmd, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = h.Kill()
		case <-h.done:
		}
	}()
	return h, nil
}

// Wait reaps the started process. It is safe for only one logical wait to be
// performed even when cleanup and normal consumption converge.
func (h *Handle) Wait() error {
	if h == nil {
		return nil
	}
	h.waitOnce.Do(func() {
		err := h.cmd.Wait()
		h.waitMu.Lock()
		h.waitErr = err
		h.waited = true
		close(h.done)
		h.waitMu.Unlock()
	})
	h.waitMu.Lock()
	defer h.waitMu.Unlock()
	return h.waitErr
}

// Kill terminates the owned process tree, or the direct process on platforms
// where process groups are unavailable.
func (h *Handle) Kill() error {
	if h == nil {
		return nil
	}
	h.waitMu.Lock()
	waited := h.waited
	h.waitMu.Unlock()
	if waited {
		return nil
	}
	h.killOnce.Do(func() { h.killErr = kill(h.cmd) })
	return h.killErr
}

// TerminateAndWait is used when setup fails after a child was started.
func (h *Handle) TerminateAndWait() error {
	if h == nil {
		return nil
	}
	_ = h.Kill()
	return h.Wait()
}
