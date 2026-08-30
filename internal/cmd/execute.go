package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
)

var runnerOverride execution.CommandRunner

// SetRunnerForTest overrides the runner used by Execute (for E2E tests).
func SetRunnerForTest(r execution.CommandRunner) { runnerOverride = r }

// ClearRunnerForTest clears the override.
func ClearRunnerForTest() { runnerOverride = nil }

func getRunner() execution.CommandRunner {
	if runnerOverride != nil {
		return runnerOverride
	}
	return execution.NewRunner()
}

// Execute is the thin CLI entry point. It routes args and returns exit code.
func Execute(ctx context.Context, args []string, cwd string, out, errOut io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	if len(args) == 0 {
		_ = writeJSONError(out, "no command", "unknown_command")
		return 1
	}
	// Helper to write error and return 1, handling EPIPE as success
	writeErr := func(msg, code string) int {
		err := writeJSONError(out, msg, code)
		if err != nil && isEPIPE(err) {
			return 0
		}
		if err != nil && !isEPIPE(err) {
			if err2 := writeJSONError(errOut, msg, code); err2 != nil && isEPIPE(err2) {
				return 0
			}
		}
		// If write succeeded or failed non-EPIPE, return 1 (error)
		// But if out was EPIPE, we already treat as success 0
		if err != nil && isEPIPE(err) {
			return 0
		}
		return 1
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "init":
		return handleInit(ctx, rest, cwd, out, writeErr)
	case "workflows":
		return handleWorkflows(ctx, rest, cwd, out, writeErr)
	case "run":
		return handleRun(ctx, rest, cwd, out, writeErr)
	case "steps":
		return handleSteps(ctx, rest, cwd, out, writeErr)
	case "step":
		return handleStep(ctx, rest, cwd, out, writeErr)
	case "status":
		return handleStatus(ctx, rest, cwd, out, writeErr)
	default:
		return writeErr(fmt.Sprintf("unknown command %q", cmd), "unknown_command")
	}
}

func handleInit(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// no flags for init
	// Check for -- terminator
	if idx := indexOf(args, "--"); idx != -1 {
		if idx+1 < len(args) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(args[idx+1:], " ")), "unexpected_argument")
		}
		args = args[:idx]
	}
	if err := fs.Parse(args); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	if err := project.Init(cwd); err != nil {
		return writeErr(err.Error(), "init_failed")
	}
	_ = writeJSON(out, map[string]string{"status": "initialized"})
	return 0
}

func handleWorkflows(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing workflows subcommand", "unknown_command")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		return handleWorkflowsList(ctx, rest, cwd, out, writeErr)
	case "describe":
		return handleWorkflowsDescribe(ctx, rest, cwd, out, writeErr)
	default:
		return writeErr(fmt.Sprintf("unknown workflows subcommand %q", sub), "unknown_command")
	}
}

func handleWorkflowsList(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	fs := flag.NewFlagSet("workflows list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "")
	// Handle -- terminator
	if idx := indexOf(args, "--"); idx != -1 {
		if idx+1 < len(args) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(args[idx+1:], " ")), "unexpected_argument")
		}
		args = args[:idx]
	}
	if err := fs.Parse(args); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	discovered, err := workflow.Discover(cwd)
	if err != nil {
		return writeErr(err.Error(), "discover_failed")
	}
	if jsonOut {
		type entry struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		var list []entry
		for _, d := range discovered {
			list = append(list, entry{Name: d.Name, Path: d.Path})
		}
		if list == nil {
			list = []entry{}
		}
		if err := writeJSON(out, map[string]any{"workflows": list}); err != nil && !isEPIPE(err) {
			return 1
		}
		return 0
	}
	// Human output
	var buf bytes.Buffer
	for _, d := range discovered {
		buf.WriteString(d.Name + "\n")
	}
	if _, err := io.Copy(out, &buf); err != nil && !isEPIPE(err) {
		return 1
	}
	return 0
}

func handleWorkflowsDescribe(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	// workflows describe requires NAME as first positional
	if len(args) == 0 {
		return writeErr("missing workflow name", "invalid_argument")
	}
	// Check for -- terminator before flag parsing? The spec says leaves consume required operands first, then parse option tail.
	// So NAME is first, rest is option tail
	name := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("workflows describe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	discovered, err := workflow.Discover(cwd)
	if err != nil {
		return writeErr(err.Error(), "discover_failed")
	}
	for _, d := range discovered {
		if d.Name == name || d.Workflow.Name == name {
			if jsonOut {
				// Return workflow details
				if err := writeJSON(out, map[string]any{"name": d.Name, "path": d.Path, "steps": d.Workflow.Steps}); err != nil && !isEPIPE(err) {
					return 1
				}
				return 0
			}
			// Human
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "name: %s\npath: %s\n", d.Name, d.Path)
			for _, s := range d.Workflow.Steps {
				fmt.Fprintf(&buf, "  - %s (%s)\n", s.ID, s.Type)
			}
			if _, err := io.Copy(out, &buf); err != nil && !isEPIPE(err) {
				return 1
			}
			return 0
		}
	}
	return writeErr(fmt.Sprintf("workflow %q not found", name), "not_found")
}

func handleRun(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing workflow name", "invalid_argument")
	}
	name := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	// Open store
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	eng := execution.NewEngine(s, getRunner(), cwd)
	id, err := eng.CreateExecution(ctx, name)
	if err != nil {
		return writeErr(err.Error(), "create_failed")
	}
	if jsonOut {
		_ = writeJSON(out, map[string]string{"execution_id": id})
	} else {
		_, _ = fmt.Fprintln(out, id)
	}
	return 0
}

func handleSteps(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing steps subcommand", "unknown_command")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "next":
		return handleStepsNext(ctx, rest, cwd, out, writeErr)
	default:
		return writeErr(fmt.Sprintf("unknown steps subcommand %q", sub), "unknown_command")
	}
}

func handleStepsNext(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing execution id", "invalid_argument")
	}
	execID := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("steps next", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	steps, err := s.Steps().List(ctx, execID)
	if err != nil {
		return writeErr(err.Error(), "not_found")
	}
	// Find next pending whose deps satisfied
	var next *store.ExecutionStep
	for _, st := range steps {
		if st.Status != "pending" {
			continue
		}
		// Check deps
		var deps []string
		_ = jsonUnmarshal(st.DependsOn, &deps)
		satisfied := true
		for _, d := range deps {
			for _, other := range steps {
				if other.StepID == d && other.Status != "completed" && other.Status != "skipped" {
					satisfied = false
					break
				}
			}
		}
		if satisfied {
			next = st
			break
		}
	}
	if next == nil {
		// No next step, maybe all done
		if jsonOut {
			_ = writeJSON(out, map[string]any{"next": nil})
		} else {
			_, _ = fmt.Fprintln(out, "no next step")
		}
		return 0
	}
	if jsonOut {
		_ = writeJSON(out, map[string]string{"step_id": next.StepID})
	} else {
		_, _ = fmt.Fprintln(out, next.StepID)
	}
	return 0
}

func handleStep(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing step subcommand", "unknown_command")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "run":
		return handleStepRun(ctx, rest, cwd, out, writeErr)
	case "reopen":
		return handleStepReopen(ctx, rest, cwd, out, writeErr)
	case "skip":
		return handleStepSkip(ctx, rest, cwd, out, writeErr)
	default:
		return writeErr(fmt.Sprintf("unknown step subcommand %q", sub), "unknown_command")
	}
}

func handleStepRun(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) < 2 {
		return writeErr("missing execution and step", "invalid_argument")
	}
	execID := args[0]
	stepID := args[1]
	rest := args[2:]
	fs := flag.NewFlagSet("step run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	var feedback string
	fs.BoolVar(&jsonOut, "json", false, "")
	fs.StringVar(&feedback, "feedback", "", "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	eng := execution.NewEngine(s, getRunner(), cwd)
	if err := eng.RunStep(ctx, execID, stepID, feedback); err != nil {
		// Check if error is due to missing dependency etc. Return json error
		// For engine errors, we return {"error":..., "code":...}
		code := "run_failed"
		if strings.Contains(err.Error(), "unsatisfied") {
			code = "unsatisfied_dependency"
		} else if strings.Contains(err.Error(), "missing") {
			code = "missing_artifact"
		} else if strings.Contains(err.Error(), "requires") {
			code = "requires_failed"
		}
		return writeErr(err.Error(), code)
	}
	if jsonOut {
		_ = writeJSON(out, map[string]string{"status": "ok"})
	} else {
		_, _ = fmt.Fprintln(out, "ok")
	}
	return 0
}

func handleStepReopen(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) < 2 {
		return writeErr("missing execution and step", "invalid_argument")
	}
	execID := args[0]
	stepID := args[1]
	rest := args[2:]
	fs := flag.NewFlagSet("step reopen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cascade bool
	var feedback string
	fs.BoolVar(&cascade, "cascade", false, "")
	fs.StringVar(&feedback, "feedback", "", "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	if !cascade {
		return writeErr("reopen requires --cascade", "invalid_argument")
	}
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	eng := execution.NewEngine(s, getRunner(), cwd)
	if err := eng.ReopenStep(ctx, execID, stepID, cascade, feedback); err != nil {
		return writeErr(err.Error(), "reopen_failed")
	}
	_ = writeJSON(out, map[string]string{"status": "reopened"})
	return 0
}

func handleStepSkip(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) < 2 {
		return writeErr("missing execution and step", "invalid_argument")
	}
	execID := args[0]
	stepID := args[1]
	rest := args[2:]
	fs := flag.NewFlagSet("step skip", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var reason string
	fs.StringVar(&reason, "reason", "", "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	if strings.TrimSpace(reason) == "" {
		return writeErr("skip requires --reason", "invalid_argument")
	}
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	eng := execution.NewEngine(s, getRunner(), cwd)
	if err := eng.SkipStep(ctx, execID, stepID, reason); err != nil {
		return writeErr(err.Error(), "skip_failed")
	}
	_ = writeJSON(out, map[string]string{"status": "skipped"})
	return 0
}

func handleStatus(ctx context.Context, args []string, cwd string, out io.Writer, writeErr func(string, string) int) int {
	if len(args) == 0 {
		return writeErr("missing execution id", "invalid_argument")
	}
	execID := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "")
	if idx := indexOf(rest, "--"); idx != -1 {
		if idx+1 < len(rest) {
			return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(rest[idx+1:], " ")), "unexpected_argument")
		}
		rest = rest[:idx]
	}
	if err := fs.Parse(rest); err != nil {
		return writeErr(err.Error(), "invalid_argument")
	}
	if fs.NArg() != 0 {
		return writeErr(fmt.Sprintf("unexpected_argument %q", strings.Join(fs.Args(), " ")), "unexpected_argument")
	}
	dbPath := filepath.Join(cwd, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return writeErr(err.Error(), "store_failed")
	}
	defer func() { _ = s.Close() }()
	exec, err := s.Executions().Get(ctx, execID)
	if err != nil {
		return writeErr(err.Error(), "not_found")
	}
	steps, _ := s.Steps().List(ctx, execID)
	if jsonOut {
		type stepInfo struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		var list []stepInfo
		for _, st := range steps {
			list = append(list, stepInfo{ID: st.StepID, Status: st.Status})
		}
		_ = writeJSON(out, map[string]any{"execution_id": exec.ID, "status": exec.Status, "steps": list})
		return 0
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "execution %s status %s\n", exec.ID, exec.Status)
	for _, st := range steps {
		fmt.Fprintf(&buf, "  %s: %s\n", st.StepID, st.Status)
	}
	if _, err := io.Copy(out, &buf); err != nil && !isEPIPE(err) {
		return 1
	}
	return 0
}

// helpers

func indexOf(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

// Ensure imports used
var _ = os.Stderr
var _ = filepath.Join
