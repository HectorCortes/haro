package broker

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
)

type executionStartParams struct {
	WorkflowPath      string  `json:"workflow_path"`
	WorkspaceOverride *string `json:"workspace_override,omitempty"`
}

type executionStatusParams struct {
	ExecutionID string `json:"execution_id"`
}

type executionStatusStep struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type executionStatusResult struct {
	Status string                `json:"status"`
	Steps  []executionStatusStep `json:"steps"`
}

func (d *Daemon) registerExecutionHandlers() {
	if d.rpc == nil {
		return
	}
	d.rpc.Dispatcher().Register("execution.start", d.handleExecutionStart)
	d.rpc.Dispatcher().Register("execution.status", d.handleExecutionStatus)
}

func (d *Daemon) handleExecutionStart(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params executionStartParams
	if err := DecodeParams(raw, &params, "workflow_path"); err != nil {
		return nil, invalidParamsError(err)
	}
	if params.WorkspaceOverride != nil {
		if err := ValidateEnum(*params.WorkspaceOverride, "workspace_override", "isolated", "shared"); err != nil {
			return nil, invalidParamsError(err)
		}
	}
	canonical, err := canonicalWorkflowPath(d.root, params.WorkflowPath)
	if err != nil {
		return nil, workflowNotFoundError()
	}
	discovered, err := workflow.Discover(d.root)
	if err != nil {
		return nil, workflowRPCError(err)
	}
	var workflowName string
	for _, entry := range discovered {
		entryPath, pathErr := canonicalWorkflowPath(d.root, entry.Path)
		if pathErr == nil && entryPath == canonical && entry.Workflow != nil {
			workflowName = entry.Workflow.Name
			break
		}
	}
	if workflowName == "" {
		return nil, workflowNotFoundError()
	}
	if d.engine == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	id, err := d.engine.CreateExecution(ctx, workflowName)
	if err != nil {
		return nil, workflowRPCError(err)
	}
	return map[string]string{"execution_id": id}, nil
}

func (d *Daemon) handleExecutionStatus(ctx context.Context, raw json.RawMessage) (any, *jsonrpc.RPCError) {
	var params executionStatusParams
	if err := DecodeParams(raw, &params, "execution_id"); err != nil {
		return nil, invalidParamsError(err)
	}
	if d.st == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
	}
	exec, err := d.st.Executions().Get(ctx, params.ExecutionID)
	if err != nil {
		return nil, referenceRPCError(err, "execution_id")
	}
	steps, err := d.st.Steps().List(ctx, params.ExecutionID)
	if err != nil {
		return nil, referenceRPCError(err, "execution_id")
	}
	result := executionStatusResult{Status: exec.Status, Steps: make([]executionStatusStep, 0, len(steps))}
	for _, step := range steps {
		result.Steps = append(result.Steps, executionStatusStep{ID: step.StepID, Status: step.Status})
	}
	return result, nil
}

func canonicalWorkflowPath(root, path string) (string, error) {
	if path == "" {
		return "", errors.New("empty workflow path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)
	eval, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(eval), nil
}

func workflowNotFoundError() *jsonrpc.RPCError {
	return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: "workflow_not_found", Data: map[string]string{"field": "params.workflow_path"}}
}

func workflowRPCError(err error) *jsonrpc.RPCError {
	var validation *workflow.ValidationError
	if errors.As(err, &validation) {
		data := map[string]string{}
		if validation.Field != "" {
			data["field"] = validation.Field
		}
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: validation.Code, Data: data}
	}
	return &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": err.Error()}}
}

func referenceRPCError(err error, field string) *jsonrpc.RPCError {
	if errors.Is(err, store.ErrNotFound) {
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: "not_found", Data: map[string]string{"field": field}}
	}
	return &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error", Data: map[string]string{"detail": err.Error()}}
}
