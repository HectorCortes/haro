package broker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
)

func TestExecutionStartResolvesCanonicalWorkflowPath(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	path := writeTestWorkflow(t, root, "named", `version: 2
name: product_name
steps:
  - id: build
    type: command
    run: echo build
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := testDaemon(t, root, s)

	result, rpcErr := d.handleExecutionStart(context.Background(), json.RawMessage(`{"workflow_path":"`+path+`"}`))
	if rpcErr != nil {
		t.Fatalf("start error: %+v", rpcErr)
	}
	id := result.(map[string]string)["execution_id"]
	got, err := s.Executions().Get(context.Background(), id)
	if err != nil || got.WorkflowSource != path {
		t.Fatalf("execution = %+v (%v), workflow source should be %q", got, err, path)
	}

	link := filepath.Join(root, "workflow-link.yaml")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result, rpcErr = d.handleExecutionStart(context.Background(), json.RawMessage(`{"workflow_path":"`+link+`"}`))
	if rpcErr != nil || result == nil {
		t.Fatalf("symlink start = %#v, %+v", result, rpcErr)
	}
}

func TestExecutionStartRejectsUnknownPathAndInvalidWorkflowWithoutRows(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	writeTestWorkflow(t, root, "valid", `version: 2
name: valid
steps:
  - id: one
    type: command
    run: echo one
`)
	cycle := writeTestWorkflow(t, root, "cycle", `version: 2
name: cycle
steps:
  - id: one
    type: command
    run: echo one
    depends_on: [two]
  - id: two
    type: command
    run: echo two
    depends_on: [one]
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := testDaemon(t, root, s)
	unknown := filepath.Join(root, "outside.yaml")
	result, rpcErr := d.handleExecutionStart(context.Background(), json.RawMessage(`{"workflow_path":"`+unknown+`"}`))
	if result != nil || rpcErr == nil || rpcErr.Code != -32602 || rpcErr.Message != "workflow_not_found" {
		t.Fatalf("unknown path = %#v, %+v", result, rpcErr)
	}
	if countExecutions(t, s) != 0 {
		t.Fatal("unknown workflow created an execution")
	}
	result, rpcErr = d.handleExecutionStart(context.Background(), json.RawMessage(`{"workflow_path":"`+cycle+`"}`))
	if result != nil || rpcErr == nil || rpcErr.Message != "cycle_detected" {
		t.Fatalf("invalid workflow = %#v, %+v", result, rpcErr)
	}
	if countExecutions(t, s) != 0 {
		t.Fatal("invalid workflow created an execution")
	}
}

func TestExecutionStatusReturnsPersistedAggregateAndSteps(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	writeTestWorkflow(t, root, "status", `version: 2
name: status
steps:
  - id: one
    type: command
    run: echo one
`)
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := testDaemon(t, root, s)
	start, rpcErr := d.handleExecutionStart(context.Background(), json.RawMessage(`{"workflow_path":".haro/workflows/status/workflow.yaml"}`))
	if rpcErr != nil {
		t.Fatalf("start: %+v", rpcErr)
	}
	id := start.(map[string]string)["execution_id"]
	status, rpcErr := d.handleExecutionStatus(context.Background(), json.RawMessage(`{"execution_id":"`+id+`"}`))
	if rpcErr != nil {
		t.Fatalf("status: %+v", rpcErr)
	}
	got := status.(executionStatusResult)
	if got.Status != "running" || len(got.Steps) != 1 || got.Steps[0].ID != "one" || got.Steps[0].Status != "pending" {
		t.Fatalf("status = %+v", got)
	}
}

func TestExecutionStatusRejectsUnknownExecutionWithoutMutation(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := testDaemon(t, root, s)

	result, rpcErr := d.handleExecutionStatus(context.Background(), json.RawMessage(`{"execution_id":"missing"}`))
	if result != nil || rpcErr == nil || rpcErr.Code != -32602 || rpcErr.Message != "not_found" {
		t.Fatalf("unknown status = %#v, %+v", result, rpcErr)
	}
	if countExecutions(t, s) != 0 {
		t.Fatal("status lookup created an execution")
	}
}

func testDaemon(t *testing.T, root string, s store.Store) *Daemon {
	t.Helper()
	d, err := NewDaemon(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.st = s
	d.engine = execution.NewEngine(s, &execution.FakeRunner{}, root)
	return d
}

func writeTestWorkflow(t *testing.T, root, name, contents string) string {
	t.Helper()
	dir := filepath.Join(root, ".haro", "workflows", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func countExecutions(t *testing.T, s *store.SQLiteStore) int {
	t.Helper()
	rows, err := s.QueryForTest(context.Background(), "SELECT COUNT(*) FROM executions")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("missing count row")
	}
	var n int
	if err := rows.Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var _ = errors.Is
var _ workflow.Workflow
