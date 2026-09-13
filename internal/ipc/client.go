package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

// EndpointEnsurer is the small seam the CLI uses for lazy broker startup.
type EndpointEnsurer interface {
	Ensure(context.Context, string) (string, error)
}

// RemoteError is an error returned by the broker's JSON-RPC boundary.
type RemoteError struct {
	Code    int
	Message string
	Data    any
}

func (e *RemoteError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("rpc error %d", e.Code)
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

type clientResponse struct {
	result json.RawMessage
	err    error
}

// Client multiplexes JSON-RPC calls and receives cursor-bearing notifications.
type Client struct {
	conn net.Conn
	br   *bufio.Reader

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[string]chan clientResponse

	notifications chan *jsonrpc.Message
	closed        chan struct{}
	closeOnce     sync.Once
}

// NewClient starts a response/notification reader for conn.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		conn:          conn,
		br:            bufio.NewReader(conn),
		pending:       make(map[string]chan clientResponse),
		notifications: make(chan *jsonrpc.Message, 64),
		closed:        make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Notifications returns the bounded notification stream. Consumers should
// drain it while calls are active so cursor-bearing status updates are kept.
func (c *Client) Notifications() <-chan *jsonrpc.Message { return c.notifications }

// Call sends one request and waits for its matched response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	key := strconv.FormatInt(id, 10)
	response := make(chan clientResponse, 1)
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return nil, net.ErrClosed
	default:
	}
	c.pending[key] = response
	c.mu.Unlock()

	c.writeMu.Lock()
	err := jsonrpc.EncodeRequest(c.conn, json.Number(key), method, params)
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(key)
		return nil, err
	}
	select {
	case reply := <-response:
		return reply.result, reply.err
	case <-ctx.Done():
		c.removePending(key)
		return nil, ctx.Err()
	case <-c.closed:
		return nil, net.ErrClosed
	}
}

// Close releases the transport and wakes all pending calls.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		err = c.conn.Close()
		c.mu.Lock()
		for key, pending := range c.pending {
			pending <- clientResponse{err: net.ErrClosed}
			delete(c.pending, key)
		}
		c.mu.Unlock()
	})
	return err
}

func (c *Client) readLoop() {
	defer close(c.notifications)
	for {
		msg, err := jsonrpc.DecodeMessage(c.br)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.failPending(err)
			} else {
				c.failPending(net.ErrClosed)
			}
			return
		}
		if msg.ID == nil {
			select {
			case c.notifications <- msg:
			case <-c.closed:
				return
			}
			continue
		}
		key := idKey(msg.ID)
		c.mu.Lock()
		pending := c.pending[key]
		if pending != nil {
			delete(c.pending, key)
		}
		c.mu.Unlock()
		if pending == nil {
			continue
		}
		if msg.Error != nil {
			pending <- clientResponse{err: &RemoteError{Code: msg.Error.Code, Message: msg.Error.Message, Data: msg.Error.Data}}
		} else {
			pending <- clientResponse{result: msg.Result}
		}
	}
}

func (c *Client) removePending(key string) {
	c.mu.Lock()
	delete(c.pending, key)
	c.mu.Unlock()
}

func (c *Client) failPending(err error) {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, pending := range c.pending {
		pending <- clientResponse{err: err}
		delete(c.pending, key)
	}
}

func idKey(id *jsonrpc.ID) string {
	if id == nil {
		return ""
	}
	if id.IsNull() {
		return "null"
	}
	if id.IsString() {
		return "s:" + id.Str
	}
	return id.Num
}

// CallWithLauncher lazily ensures an endpoint, dials it, performs one call,
// and closes the short-lived client. A nil launcher uses the transport's
// endpoint derivation without spawning.
func CallWithLauncher(ctx context.Context, launcher EndpointEnsurer, tr Transport, root, method string, params any) (json.RawMessage, error) {
	if tr == nil {
		return nil, errors.New("nil IPC transport")
	}
	var endpoint string
	var err error
	if launcher != nil {
		endpoint, err = launcher.Ensure(ctx, root)
	} else {
		endpoint, err = tr.Endpoint(root)
	}
	if err != nil {
		return nil, err
	}
	conn, err := tr.Dial(endpoint)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	defer client.Close()
	return client.Call(ctx, method, params)
}
