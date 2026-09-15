//go:build linux

package broker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/ipc"
	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/store"
)

func TestLinuxBrokerProcessSharedAndRestart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Unix-domain socket E2E is Linux-only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root := t.TempDir()
	wfPath := filepath.Join(root, ".haro", "workflows", "demo", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(wfPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wfPath, []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo e2e\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "haro-e2e")
	build := exec.CommandContext(ctx, "go", "build", "-p=1", "-o", bin, ".")
	build.Dir = repoRootForE2E(t)
	build.Env = append(os.Environ(), "GOMAXPROCS=2", "GOMEMLIMIT=512MiB")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build broker binary: %v\n%s", err, output)
	}

	transport := ipc.DefaultTransport()
	endpoint, err := transport.Endpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	start := func() (*exec.Cmd, *ipc.Client) {
		t.Helper()
		cmd := exec.Command(bin, "broker", "--project", root)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		configureLinuxBrokerCommand(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start broker: %v", err)
		}
		t.Cleanup(func() {
			_ = clientCloseIfStarted(cmd)
		})
		conn := waitForBrokerEndpoint(t, ctx, transport, endpoint)
		client := ipc.NewClient(conn)
		t.Cleanup(func() {
			_ = client.Close()
			_ = clientCloseIfStarted(cmd)
		})
		if _, err := client.Call(ctx, "health", map[string]any{}); err != nil {
			_ = client.Close()
			_ = killAndWaitLinuxBroker(cmd)
			t.Fatalf("health: %v (stderr=%s)", err, stderr.String())
		}
		return cmd, client
	}
	stop := func(cmd *exec.Cmd, client *ipc.Client) {
		t.Helper()
		_ = client.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("broker wait: %v", err)
		}
		waitForBrokerEndpointGone(t, ctx, transport, endpoint)
	}

	cmd, clientA := start()
	clientB := ipc.NewClient(mustDialE2E(t, transport, endpoint))
	startResult, err := clientA.Call(ctx, "execution.start", map[string]any{"workflow_path": wfPath})
	if err != nil {
		stop(cmd, clientA)
		_ = clientB.Close()
		t.Fatal(err)
	}
	var executionResult struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(startResult, &executionResult); err != nil || executionResult.ExecutionID == "" {
		stop(cmd, clientA)
		_ = clientB.Close()
		t.Fatalf("execution.start result = %s, err=%v", startResult, err)
	}
	if _, err := clientB.Call(ctx, "execution.status", map[string]any{"execution_id": executionResult.ExecutionID}); err != nil {
		stop(cmd, clientA)
		_ = clientB.Close()
		t.Fatalf("second client status: %v", err)
	}
	stop(cmd, clientA)
	_ = clientB.Close()

	cmd, client := start()
	defer func() {
		_ = client.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
	}()
	if _, err := client.Call(ctx, "execution.status", map[string]any{"execution_id": executionResult.ExecutionID}); err != nil {
		t.Fatalf("status after restart: %v", err)
	}
}

func clientCloseIfStarted(cmd *exec.Cmd) error {
	if cmd == nil || cmd.ProcessState != nil || cmd.Process == nil {
		return nil
	}
	return killAndWaitLinuxBroker(cmd)
}

func repoRootForE2E(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func waitForBrokerEndpoint(t *testing.T, ctx context.Context, tr ipc.Transport, endpoint string) net.Conn {
	t.Helper()
	for {
		conn, err := tr.DialTimeout(endpoint, 100*time.Millisecond)
		if err == nil {
			return conn
		}
		select {
		case <-ctx.Done():
			t.Fatalf("broker endpoint did not become ready: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func waitForBrokerEndpointGone(t *testing.T, ctx context.Context, tr ipc.Transport, endpoint string) {
	t.Helper()
	for {
		conn, err := tr.DialTimeout(endpoint, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		select {
		case <-ctx.Done():
			t.Fatalf("broker endpoint remained live: %s", endpoint)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func mustDialE2E(t *testing.T, tr ipc.Transport, endpoint string) net.Conn {
	t.Helper()
	conn, err := tr.DialTimeout(endpoint, 2*time.Second)
	if err != nil {
		t.Fatal(fmt.Errorf("dial broker: %w", err))
	}
	return conn
}

// TestLinuxBrokerProcessE2EMatrix exercises the remaining U7.11 process
// boundary cases with one bounded, serial binary build. The unit tests cover
// the lower-level seams; this test deliberately uses the compiled CLI and
// Linux UDS transport for the cross-process matrix.
func TestLinuxBrokerProcessE2EMatrix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Unix-domain socket E2E is Linux-only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	bin := buildLinuxBrokerBinary(t, ctx)
	tr := ipc.DefaultTransport()

	t.Run("F-01 shared broker serves two CLIs", func(t *testing.T) {
		root := t.TempDir()
		writeLinuxE2EWorkflow(t, root, "shared", "echo shared")
		brokerCmd, endpoint := startLinuxBrokerProcess(t, ctx, bin, root)

		idA := runLinuxCLIForExecution(t, ctx, bin, root, "shared")
		idB := runLinuxCLIForExecution(t, ctx, bin, root, "shared")
		if idA == idB {
			t.Fatalf("two CLI invocations returned the same execution ID %q", idA)
		}
		client := newLinuxE2EClient(t, tr, endpoint)
		defer client.Close()
		for _, id := range []string{idA, idB} {
			status := getLinuxExecutionStatus(t, ctx, client, id)
			if status.Status != "running" || len(status.Steps) != 1 || status.Steps[0].Status != "pending" {
				t.Fatalf("execution %s status = %+v, want running/pending", id, status)
			}
		}
		stopLinuxBrokerProcess(t, tr, endpoint, brokerCmd, syscall.SIGTERM, true)
	})

	t.Run("F-02 relaunches after broker death", func(t *testing.T) {
		root := t.TempDir()
		var mu sync.Mutex
		var spawned []*exec.Cmd
		spawn := func(_ context.Context, canonicalRoot string) error {
			cmd := exec.Command(bin, "broker", "--project", canonicalRoot)
			cmd.Env = boundedLinuxE2EEnv()
			configureLinuxBrokerCommand(cmd)
			if err := cmd.Start(); err != nil {
				return err
			}
			mu.Lock()
			spawned = append(spawned, cmd)
			mu.Unlock()
			return nil
		}
		launcher := NewLauncher(tr, spawn)
		endpoint, err := launcher.Ensure(ctx, root)
		if err != nil {
			t.Fatalf("initial lazy launch: %v", err)
		}
		t.Cleanup(func() {
			mu.Lock()
			processes := append([]*exec.Cmd(nil), spawned...)
			mu.Unlock()
			cleanupLinuxBrokerProcesses(processes, endpoint)
		})
		mu.Lock()
		if len(spawned) != 1 {
			n := len(spawned)
			mu.Unlock()
			t.Fatalf("initial lazy launch spawned %d brokers, want one", n)
		}
		first := spawned[0]
		mu.Unlock()
		if err := killLinuxBrokerGroup(first); err != nil {
			t.Fatalf("kill first broker: %v", err)
		}
		if err := first.Wait(); err == nil {
			t.Fatal("SIGKILL unexpectedly returned a clean broker exit")
		}
		waitForBrokerEndpointRefused(t, ctx, tr, endpoint)

		if _, err := launcher.Ensure(ctx, root); err != nil {
			t.Fatalf("relaunch after death: %v", err)
		}
		mu.Lock()
		count := len(spawned)
		second := spawned[1]
		mu.Unlock()
		if count != 2 {
			t.Fatalf("relaunch spawned %d brokers, want two total", count)
		}
		client := newLinuxE2EClient(t, tr, endpoint)
		_ = client.Close()
		stopLinuxBrokerProcess(t, tr, endpoint, second, syscall.SIGTERM, true)
		_ = os.Remove(endpoint)
	})

	t.Run("F-03 parallel executions isolate state", func(t *testing.T) {
		root := t.TempDir()
		workflowPath := writeLinuxE2EWorkflow(t, root, "parallel", "echo parallel")
		brokerCmd, endpoint := startLinuxBrokerProcess(t, ctx, bin, root)
		clientA := newLinuxE2EClient(t, tr, endpoint)
		defer clientA.Close()
		clientB := newLinuxE2EClient(t, tr, endpoint)
		defer clientB.Close()
		idA := startLinuxExecution(t, ctx, clientA, workflowPath)
		idB := startLinuxExecution(t, ctx, clientB, workflowPath)

		type runResult struct {
			id      string
			attempt string
			err     error
		}
		results := make(chan runResult, 2)
		var wg sync.WaitGroup
		for i, client := range []*ipc.Client{clientA, clientB} {
			wg.Add(1)
			go func(i int, c *ipc.Client) {
				defer wg.Done()
				id := []string{idA, idB}[i]
				raw, err := c.Call(ctx, "step.run", map[string]string{"execution_id": id, "step_id": "s1"})
				if err != nil {
					results <- runResult{id: id, err: err}
					return
				}
				var result struct {
					AttemptID string `json:"attempt_id"`
				}
				err = json.Unmarshal(raw, &result)
				results <- runResult{id: id, attempt: result.AttemptID, err: err}
			}(i, client)
		}
		wg.Wait()
		close(results)
		attempts := make(map[string]string, 2)
		for result := range results {
			if result.err != nil || result.attempt == "" {
				t.Fatalf("parallel step.run for %s = attempt %q, err=%v", result.id, result.attempt, result.err)
			}
			attempts[result.id] = result.attempt
		}
		if attempts[idA] == attempts[idB] {
			t.Fatalf("parallel executions shared attempt %q", attempts[idA])
		}
		for _, id := range []string{idA, idB} {
			status := waitForLinuxStepStatus(t, ctx, clientA, id, "completed")
			if status.Steps[0].Status != "completed" {
				t.Fatalf("execution %s final status = %+v", id, status)
			}
			events := getLinuxStepEvents(t, ctx, clientA, id, 0)
			if len(events.Events) == 0 || events.NextCursor <= 0 {
				t.Fatalf("execution %s events = %+v, want its persisted output", id, events)
			}
		}
		stopLinuxBrokerProcess(t, tr, endpoint, brokerCmd, syscall.SIGTERM, true)
	})

	t.Run("F-04 independent projects survive peer shutdown", func(t *testing.T) {
		rootA, rootB := t.TempDir(), t.TempDir()
		pathA := writeLinuxE2EWorkflow(t, rootA, "a", "echo project-a")
		pathB := writeLinuxE2EWorkflow(t, rootB, "b", "echo project-b")
		cmdA, endpointA := startLinuxBrokerProcess(t, ctx, bin, rootA)
		cmdB, endpointB := startLinuxBrokerProcess(t, ctx, bin, rootB)
		if endpointA == endpointB {
			t.Fatalf("independent projects share endpoint %q", endpointA)
		}
		clientA := newLinuxE2EClient(t, tr, endpointA)
		defer clientA.Close()
		clientB := newLinuxE2EClient(t, tr, endpointB)
		defer clientB.Close()
		idA := startLinuxExecution(t, ctx, clientA, pathA)
		idB := startLinuxExecution(t, ctx, clientB, pathB)
		if idA == idB {
			t.Fatalf("independent projects returned the same execution ID %q", idA)
		}
		stopLinuxBrokerProcess(t, tr, endpointA, cmdA, syscall.SIGTERM, true)
		status := getLinuxExecutionStatus(t, ctx, clientB, idB)
		if status.Status != "running" {
			t.Fatalf("project B status after project A shutdown = %+v", status)
		}
		stopLinuxBrokerProcess(t, tr, endpointB, cmdB, syscall.SIGTERM, true)
	})

	t.Run("F-05 launch herd starts one process", func(t *testing.T) {
		root := t.TempDir()
		var mu sync.Mutex
		var spawned []*exec.Cmd
		launcher := NewLauncher(tr, func(_ context.Context, canonicalRoot string) error {
			cmd := exec.Command(bin, "broker", "--project", canonicalRoot)
			cmd.Env = boundedLinuxE2EEnv()
			configureLinuxBrokerCommand(cmd)
			if err := cmd.Start(); err != nil {
				return err
			}
			mu.Lock()
			spawned = append(spawned, cmd)
			mu.Unlock()
			return nil
		})
		const clients = 4
		endpoints := make([]string, clients)
		errs := make([]error, clients)
		var wg sync.WaitGroup
		for i := 0; i < clients; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				endpoints[i], errs[i] = launcher.Ensure(ctx, root)
			}(i)
		}
		wg.Wait()
		t.Cleanup(func() {
			mu.Lock()
			processes := append([]*exec.Cmd(nil), spawned...)
			mu.Unlock()
			cleanupLinuxBrokerProcesses(processes, endpoints[0])
		})
		for i, err := range errs {
			if err != nil {
				t.Fatalf("herd Ensure[%d]: %v", i, err)
			}
			if endpoints[i] != endpoints[0] {
				t.Fatalf("herd endpoint[%d] = %q, want %q", i, endpoints[i], endpoints[0])
			}
		}
		mu.Lock()
		count := len(spawned)
		process := spawned[0]
		mu.Unlock()
		if count != 1 {
			t.Fatalf("launch herd spawned %d brokers, want one", count)
		}
		client := newLinuxE2EClient(t, tr, endpoints[0])
		_ = client.Close()
		stopLinuxBrokerProcess(t, tr, endpoints[0], process, syscall.SIGTERM, true)
		_ = os.Remove(endpoints[0])
	})

	t.Run("F-06 kill nine fences stale writer", func(t *testing.T) {
		root := t.TempDir()
		workflowPath := writeLinuxE2EWorkflow(t, root, "fenced", "sleep 2")
		brokerCmd, endpoint := startLinuxBrokerProcess(t, ctx, bin, root)
		client := newLinuxE2EClient(t, tr, endpoint)
		defer client.Close()
		executionID := startLinuxExecution(t, ctx, client, workflowPath)
		raw, err := client.Call(ctx, "step.run", map[string]string{"execution_id": executionID, "step_id": "s1"})
		if err != nil {
			t.Fatalf("slow step.run: %v", err)
		}
		var runResult struct {
			AttemptID string `json:"attempt_id"`
		}
		if err := json.Unmarshal(raw, &runResult); err != nil || runResult.AttemptID == "" {
			t.Fatalf("slow step.run result = %s, err=%v", raw, err)
		}
		leaseStore, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
		if err != nil {
			t.Fatal(err)
		}
		lease, err := leaseStore.Leases().Get(ctx, executionID, "s1")
		_ = leaseStore.Close()
		if err != nil {
			t.Fatalf("read attempt lease: %v", err)
		}
		if err := killLinuxBrokerGroup(brokerCmd); err != nil {
			t.Fatalf("kill broker: %v", err)
		}
		if err := brokerCmd.Wait(); err == nil {
			t.Fatal("SIGKILL unexpectedly returned a clean broker exit")
		}
		waitForBrokerEndpointRefused(t, ctx, tr, endpoint)

		staleStore, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := staleStore.Leases().Release(ctx, executionID, "s1", lease.Holder, lease.FencingToken); err != nil {
			_ = staleStore.Close()
			t.Fatalf("expire killed broker lease: %v", err)
		}
		stale := execution.NewFencedStore(staleStore, executionID, "s1", lease.Holder, lease.FencingToken)
		ended := "now"
		reason := "stale writer probe"
		if err := stale.Attempts().UpdateStatus(ctx, runResult.AttemptID, "completed", &ended, &reason, nil); !errors.Is(err, execution.ErrStaleLease) {
			_ = staleStore.Close()
			t.Fatalf("stale write error = %v, want ErrStaleLease", err)
		}
		attempt, err := staleStore.Attempts().Get(ctx, runResult.AttemptID)
		_ = staleStore.Close()
		if err != nil || attempt.Status != "running" {
			t.Fatalf("stale write changed attempt = %+v, err=%v", attempt, err)
		}

		replacement, replacementEndpoint := startLinuxBrokerProcess(t, ctx, bin, root)
		if replacementEndpoint != endpoint {
			t.Fatalf("replacement endpoint = %q, want %q", replacementEndpoint, endpoint)
		}
		replacementClient := newLinuxE2EClient(t, tr, replacementEndpoint)
		defer replacementClient.Close()
		if _, err := replacementClient.Call(ctx, "step.reopen", map[string]any{"execution_id": executionID, "step_id": "s1", "cascade": true}); err != nil {
			t.Fatalf("reopen after fenced death: %v", err)
		}
		if _, err := replacementClient.Call(ctx, "step.run", map[string]string{"execution_id": executionID, "step_id": "s1"}); err != nil {
			t.Fatalf("rerun after reopen: %v", err)
		}
		waitForLinuxStepStatus(t, ctx, replacementClient, executionID, "completed")
		stopLinuxBrokerProcess(t, tr, replacementEndpoint, replacement, syscall.SIGTERM, true)
		_ = os.Remove(endpoint)
	})

	t.Run("U-02 malformed and guarded frames", func(t *testing.T) {
		root := t.TempDir()
		workflowPath := writeLinuxE2EWorkflow(t, root, "guards", "echo guarded")
		brokerCmd, endpoint := startLinuxBrokerProcess(t, ctx, bin, root)
		conn := mustDialE2E(t, tr, endpoint)
		reader := bufio.NewReader(conn)
		if _, err := conn.Write([]byte(`{"jsonrpc":"2.0","method":"health","id":1` + "\n")); err != nil {
			t.Fatal(err)
		}
		parseError := readRPCMessage(t, reader)
		if parseError.Error == nil || parseError.Error.Code != jsonrpc.ParseErrorCode {
			t.Fatalf("malformed frame response = %+v", parseError.Error)
		}
		if err := jsonrpc.EncodeRequest(conn, 2, "health", nil); err != nil {
			t.Fatal(err)
		}
		valid := readRPCMessage(t, reader)
		if valid.Error != nil || valid.ID == nil || valid.ID.Num != "2" {
			t.Fatalf("valid frame after malformed response = %+v", valid)
		}
		_ = conn.Close()

		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := tr.DialTimeout(endpoint, 2*time.Second)
				if err != nil {
					errs <- err
					return
				}
				client := ipc.NewClient(conn)
				defer client.Close()
				_, err = client.Call(ctx, "health", map[string]any{})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent health frame: %v", err)
			}
		}

		client := newLinuxE2EClient(t, tr, endpoint)
		defer client.Close()
		executionID := startLinuxExecution(t, ctx, client, workflowPath)
		for _, mode := range []string{"supervised", "terminal"} {
			_, err := client.Call(ctx, "step.run", map[string]string{"execution_id": executionID, "step_id": "s1", "mode": mode})
			var remote *ipc.RemoteError
			if !errors.As(err, &remote) || remote.Code != -32003 || remote.Message != mode+" mode not supported" {
				t.Fatalf("mode %q error = %v, want -32003", mode, err)
			}
		}
		status := getLinuxExecutionStatus(t, ctx, client, executionID)
		if status.Steps[0].Status != "pending" {
			t.Fatalf("guarded modes changed step status: %+v", status)
		}
		checkStore, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
		if err != nil {
			t.Fatal(err)
		}
		count, err := checkStore.Attempts().CountByStep(ctx, executionID, "s1")
		_ = checkStore.Close()
		if err != nil || count != 0 {
			t.Fatalf("guarded modes created %d attempts, err=%v", count, err)
		}
		stopLinuxBrokerProcess(t, tr, endpoint, brokerCmd, syscall.SIGTERM, true)
	})
}

type linuxExecutionStatus struct {
	Status string `json:"status"`
	Steps  []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"steps"`
}

type linuxStepEvents struct {
	Events     []json.RawMessage `json:"events"`
	NextCursor int               `json:"next_cursor"`
}

func buildLinuxBrokerBinary(t *testing.T, ctx context.Context) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "haro-linux-e2e")
	cmd := exec.CommandContext(ctx, "go", "build", "-p=1", "-o", bin, ".")
	cmd.Dir = repoRootForE2E(t)
	cmd.Env = boundedLinuxE2EEnv()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bounded broker build: %v\n%s", err, output)
	}
	return bin
}

func boundedLinuxE2EEnv() []string {
	return append(os.Environ(), "GOMAXPROCS=2", "GOMEMLIMIT=512MiB")
}

func cleanupLinuxBrokerProcesses(processes []*exec.Cmd, endpoint string) {
	for _, cmd := range processes {
		_ = killAndWaitLinuxBroker(cmd)
	}
	_ = os.Remove(endpoint)
}

func writeLinuxE2EWorkflow(t *testing.T, root, name, command string) string {
	t.Helper()
	path := filepath.Join(root, ".haro", "workflows", name, "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := fmt.Sprintf("version: 2\nname: %s\nsteps:\n  - id: s1\n    type: command\n    run: %s\n", name, command)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func startLinuxBrokerProcess(t *testing.T, ctx context.Context, bin, root string) (*exec.Cmd, string) {
	t.Helper()
	tr := ipc.DefaultTransport()
	endpoint, err := tr.Endpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "broker", "--project", root)
	cmd.Env = boundedLinuxE2EEnv()
	configureLinuxBrokerCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = killAndWaitLinuxBroker(cmd)
		_ = os.Remove(endpoint)
	})
	waitForBrokerEndpoint(t, ctx, tr, endpoint)
	waitForLinuxBrokerHealth(t, ctx, tr, endpoint)
	return cmd, endpoint
}

func stopLinuxBrokerProcess(t *testing.T, tr ipc.Transport, endpoint string, cmd *exec.Cmd, signal syscall.Signal, requireCleanExit bool) {
	t.Helper()
	if cmd.ProcessState == nil && cmd.Process != nil {
		if err := cmd.Process.Signal(signal); err != nil {
			t.Fatalf("signal broker: %v", err)
		}
	}
	if cmd.ProcessState == nil {
		err := cmd.Wait()
		if requireCleanExit && err != nil {
			t.Fatalf("broker exit after %s: %v", signal, err)
		}
	}
	if requireCleanExit {
		waitForBrokerEndpointRemoved(t, tr, endpoint)
	}
}

func configureLinuxBrokerCommand(cmd *exec.Cmd) {
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Setpgid = true
	cmd.SysProcAttr = attr
}

func killLinuxBrokerGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.ProcessState != nil || cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func killAndWaitLinuxBroker(cmd *exec.Cmd) error {
	if err := killLinuxBrokerGroup(cmd); err != nil {
		return err
	}
	if cmd == nil || cmd.ProcessState != nil || cmd.Process == nil {
		return nil
	}
	return cmd.Wait()
}

func waitForLinuxBrokerHealth(t *testing.T, ctx context.Context, tr ipc.Transport, endpoint string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tr.DialTimeout(endpoint, 250*time.Millisecond)
		if err == nil {
			client := ipc.NewClient(conn)
			callCtx, cancel := context.WithTimeout(ctx, time.Second)
			_, callErr := client.Call(callCtx, "health", map[string]any{})
			cancel()
			_ = client.Close()
			if callErr == nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("broker health wait: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatalf("broker health did not become ready: %s", endpoint)
}

func waitForBrokerEndpointRemoved(t *testing.T, tr ipc.Transport, endpoint string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tr.DialTimeout(endpoint, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
		} else if _, statErr := os.Stat(endpoint); os.IsNotExist(statErr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_, statErr := os.Stat(endpoint)
	t.Fatalf("broker endpoint was not removed: %s (stat=%v)", endpoint, statErr)
}

func waitForBrokerEndpointRefused(t *testing.T, ctx context.Context, tr ipc.Transport, endpoint string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tr.DialTimeout(endpoint, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		select {
		case <-ctx.Done():
			t.Fatalf("broker endpoint refusal wait: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatalf("broker endpoint remained responsive after death: %s", endpoint)
}

func newLinuxE2EClient(t *testing.T, tr ipc.Transport, endpoint string) *ipc.Client {
	t.Helper()
	return ipc.NewClient(mustDialE2E(t, tr, endpoint))
}

func runLinuxCLIForExecution(t *testing.T, ctx context.Context, bin, root, workflowName string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, "run", workflowName, "--json")
	cmd.Dir = root
	cmd.Env = boundedLinuxE2EEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI run %s: %v\n%s", workflowName, err, output)
	}
	var result struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil || result.ExecutionID == "" {
		t.Fatalf("CLI run result = %q, err=%v", output, err)
	}
	return result.ExecutionID
}

func startLinuxExecution(t *testing.T, ctx context.Context, client *ipc.Client, workflowPath string) string {
	t.Helper()
	raw, err := client.Call(ctx, "execution.start", map[string]string{"workflow_path": workflowPath})
	if err != nil {
		t.Fatalf("execution.start: %v", err)
	}
	var result struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.ExecutionID == "" {
		t.Fatalf("execution.start result = %s, err=%v", raw, err)
	}
	return result.ExecutionID
}

func getLinuxExecutionStatus(t *testing.T, ctx context.Context, client *ipc.Client, executionID string) linuxExecutionStatus {
	t.Helper()
	raw, err := client.Call(ctx, "execution.status", map[string]string{"execution_id": executionID})
	if err != nil {
		t.Fatalf("execution.status %s: %v", executionID, err)
	}
	var result linuxExecutionStatus
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("execution.status result = %s: %v", raw, err)
	}
	return result
}

func waitForLinuxStepStatus(t *testing.T, ctx context.Context, client *ipc.Client, executionID, want string) linuxExecutionStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status := getLinuxExecutionStatus(t, ctx, client, executionID)
		if len(status.Steps) > 0 && status.Steps[0].Status == want {
			return status
		}
		time.Sleep(25 * time.Millisecond)
	}
	status := getLinuxExecutionStatus(t, ctx, client, executionID)
	t.Fatalf("execution %s did not reach %s: %+v", executionID, want, status)
	return status
}

func getLinuxStepEvents(t *testing.T, ctx context.Context, client *ipc.Client, executionID string, since int) linuxStepEvents {
	t.Helper()
	raw, err := client.Call(ctx, "step.events", map[string]any{"execution_id": executionID, "step_id": "s1", "since_cursor": since})
	if err != nil {
		t.Fatalf("step.events %s: %v", executionID, err)
	}
	var result linuxStepEvents
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("step.events result = %s: %v", raw, err)
	}
	return result
}
