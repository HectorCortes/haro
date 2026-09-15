// Package limits contains shared memory limits for external process output and
// adapter event retention.
package limits

import (
	"bytes"
	"sync"
)

const (
	// CommandOutputTotalCaptureLimit bounds the combined stdout and stderr
	// retained for one command invocation.
	CommandOutputTotalCaptureLimit = 2 * 1024 * 1024

	// AdapterEventRetentionLimit bounds raw JSONL frames retained by one
	// adapter parser while preserving the existing 10 MiB frame limit.
	AdapterEventRetentionLimit = 12 * 1024 * 1024

	outputTruncationNotice = "\n[output truncated: capture limit exceeded]"
)

type captureBudget struct {
	mu        sync.Mutex
	remaining int
}

func (b *captureBudget) reserve(size int) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return 0
	}
	if size > b.remaining {
		size = b.remaining
	}
	b.remaining -= size
	return size
}

// CaptureWriter retains at most the share available in its capture budget.
// It reports every input byte as consumed so a noisy child cannot block while
// the excess output is discarded.
type CaptureWriter struct {
	budget    *captureBudget
	mu        sync.Mutex
	buf       bytes.Buffer
	truncated bool
}

// NewCaptureWriter creates a bounded writer with an independent total budget.
func NewCaptureWriter() *CaptureWriter {
	return newCaptureWriter(&captureBudget{remaining: CommandOutputTotalCaptureLimit})
}

// NewCaptureWriters creates stdout and stderr writers sharing one total budget.
func NewCaptureWriters() (*CaptureWriter, *CaptureWriter) {
	budget := &captureBudget{remaining: CommandOutputTotalCaptureLimit}
	return newCaptureWriter(budget), newCaptureWriter(budget)
}

func newCaptureWriter(budget *captureBudget) *CaptureWriter {
	return &CaptureWriter{budget: budget}
}

func (w *CaptureWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := w.budget.reserve(len(p))
	w.mu.Lock()
	if n > 0 {
		_, _ = w.buf.Write(p[:n])
	}
	if n != len(p) {
		w.truncated = true
	}
	w.mu.Unlock()
	return len(p), nil
}

// String returns retained output and makes truncation explicit without
// exceeding the configured capture capacity.
func (w *CaptureWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	value := w.buf.String()
	if !w.truncated {
		return value
	}
	if len(value) == 0 {
		return ""
	}
	if len(value) >= len(outputTruncationNotice) {
		return value[:len(value)-len(outputTruncationNotice)] + outputTruncationNotice
	}
	// A marker cannot fit without exceeding the bytes already retained.
	return outputTruncationNotice[:len(value)]
}

// Len reports the retained bytes, excluding any truncation notice.
func (w *CaptureWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Len()
}
