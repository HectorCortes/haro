package broker

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

// RPCHandler handles one method after envelope parsing. Implementations must
// validate params before performing effects; DecodeParams is the shared gate.
type RPCHandler func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError)

// Dispatcher maps method names to handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]RPCHandler
}

// NewDispatcher creates an empty dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]RPCHandler)}
}

// Register installs or replaces a method handler.
func (d *Dispatcher) Register(method string, handler RPCHandler) {
	if d == nil || method == "" || handler == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handlers == nil {
		d.handlers = make(map[string]RPCHandler)
	}
	d.handlers[method] = handler
}

// Dispatch calls a registered method or returns the standard method error.
func (d *Dispatcher) Dispatch(ctx context.Context, method string, params json.RawMessage) (result any, rpcErr *jsonrpc.RPCError) {
	if d == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.MethodNotFoundCode, Message: "method not found"}
	}
	d.mu.RLock()
	handler := d.handlers[method]
	d.mu.RUnlock()
	if handler == nil {
		return nil, &jsonrpc.RPCError{Code: jsonrpc.MethodNotFoundCode, Message: "method not found"}
	}
	return invokeHandler(ctx, handler, params)
}

func invokeHandler(ctx context.Context, handler RPCHandler, params json.RawMessage) (result any, rpcErr *jsonrpc.RPCError) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			rpcErr = &jsonrpc.RPCError{Code: jsonrpc.InternalErrorCode, Message: "internal error"}
		}
	}()
	return handler(ctx, params)
}
