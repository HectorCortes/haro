package ipc

import (
	"encoding/json"
	"fmt"
	"net"
)

// Request is the minimal JSON-RPC 2.0 health request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int    `json:"id"`
}

// Response is the JSON-RPC 2.0 health response.
type Response struct {
	JSONRPC string  `json:"jsonrpc"`
	Result  *Result `json:"result,omitempty"`
	ID      int     `json:"id"`
}

// Result is the health payload.
type Result struct {
	OK bool `json:"ok"`
}

// HandleConn handles a single JSON-RPC connection.
// It expects exactly {"jsonrpc":"2.0","method":"health","id":1}
// and responds with {"jsonrpc":"2.0","result":{"ok":true},"id":1}.
// Transport, decode or write failures return errors.
func HandleConn(c net.Conn) error {
	defer c.Close()

	dec := json.NewDecoder(c)
	var req Request
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if req.JSONRPC != "2.0" || req.Method != "health" {
		return fmt.Errorf("unsupported request: %+v", req)
	}

	resp := Response{
		JSONRPC: "2.0",
		Result:  &Result{OK: true},
		ID:      req.ID,
	}
	enc := json.NewEncoder(c)
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
