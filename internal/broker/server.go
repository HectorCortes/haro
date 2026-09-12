package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

// Server serves strict NDJSON JSON-RPC requests on one connection at a time.
// The dispatcher is shared safely by all daemon connection goroutines.
type Server struct {
	dispatcher *Dispatcher
}

// NewServer creates a server backed by dispatcher.
func NewServer(dispatcher *Dispatcher) *Server {
	if dispatcher == nil {
		dispatcher = NewDispatcher()
	}
	return &Server{dispatcher: dispatcher}
}

// Dispatcher exposes the server's registration seam for daemon wiring and
// focused tests.
func (s *Server) Dispatcher() *Dispatcher { return s.dispatcher }

// ServeConn processes requests until the peer closes, the context is done, or
// a transport write fails. Parse errors are recoverable at the frame boundary.
func (s *Server) ServeConn(ctx context.Context, conn net.Conn) error {
	if conn == nil {
		return errors.New("nil connection")
	}
	defer func() { _ = conn.Close() }()
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-closed:
		}
	}()
	br := bufio.NewReader(conn)
	var writeMu sync.Mutex
	writeError := func(id any, rpcErr jsonrpc.RPCError) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return jsonrpc.EncodeError(conn, id, rpcErr)
	}
	writeResult := func(id any, result any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return jsonrpc.EncodeResponse(conn, id, result)
	}
	for {
		msg, err := jsonrpc.DecodeMessage(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if writeErr := writeError(nil, jsonrpc.RPCError{Code: jsonrpc.ParseErrorCode, Message: "Parse error"}); writeErr != nil {
				return writeErr
			}
			continue
		}
		id := wireID(msg.ID)
		if err := validateEnvelope(msg); err != nil {
			if msg.ID == nil {
				continue
			}
			if writeErr := writeError(id, *err); writeErr != nil {
				return writeErr
			}
			continue
		}
		result, rpcErr := s.dispatcher.Dispatch(ctx, msg.Method, msg.Params)
		if msg.ID == nil {
			continue
		}
		if rpcErr != nil {
			if writeErr := writeError(id, *rpcErr); writeErr != nil {
				return writeErr
			}
			continue
		}
		if err := writeResult(id, result); err != nil {
			return err
		}
	}
}

func validateEnvelope(msg *jsonrpc.Message) *jsonrpc.RPCError {
	if msg == nil || msg.JSONRPC != "2.0" || msg.Method == "" || msg.Result != nil || msg.Error != nil {
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidRequestCode, Message: "Invalid Request"}
	}
	return nil
}

func wireID(id *jsonrpc.ID) any {
	if id == nil || id.IsNull() {
		return nil
	}
	if id.IsString() {
		return id.Str
	}
	return json.Number(id.Num)
}
