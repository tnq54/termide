package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Message is one JSON-RPC 2.0 frame: request (ID+Method), response
// (ID+Result/Error) or notification (Method only).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("lsp error %d: %s", e.Code, e.Message) }

type Response Message
type Notification Message

// NotifyFunc receives server notifications (diagnostics, progress, ...).
type NotifyFunc func(method string, params json.RawMessage)

// Conn multiplexes JSON-RPC over one framed stream: Call matches responses
// by ID while the read loop dispatches notifications. Not safe for concurrent
// writes from multiple goroutines beyond Call's internal lock.
type Conn struct {
	stream  io.ReadWriteCloser
	notify  NotifyFunc
	mu      sync.Mutex
	next    atomic.Int64
	pending map[string]chan Message
	closed  chan struct{}
	once    sync.Once
}

func NewConn(stream io.ReadWriteCloser, notify NotifyFunc) *Conn {
	c := &Conn{stream: stream, notify: notify, pending: make(map[string]chan Message), closed: make(chan struct{})}
	go c.readLoop()
	return c
}

func idKey(id any) string {
	raw, _ := json.Marshal(id)
	return string(raw)
}

func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var payload json.RawMessage
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		payload = raw
	}
	id := c.next.Add(1)
	reply := make(chan Message, 1)
	key := idKey(id)
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return nil, fmt.Errorf("lsp connection closed")
	default:
	}
	c.pending[key] = reply
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
	}()
	if err := Write(c.stream, Message{JSONRPC: "2.0", ID: id, Method: method, Params: payload}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, fmt.Errorf("lsp connection closed")
	case response := <-reply:
		if response.Error != nil {
			return nil, response.Error
		}
		return response.Result, nil
	}
}

func (c *Conn) Notify(method string, params any) error {
	var payload json.RawMessage
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return err
		}
		payload = raw
	}
	return Write(c.stream, Message{JSONRPC: "2.0", Method: method, Params: payload})
}

func (c *Conn) readLoop() {
	defer c.Close()
	for {
		message, err := Read(c.stream)
		if err != nil {
			return
		}
		if message.ID != nil && message.Method == "" {
			c.mu.Lock()
			reply, ok := c.pending[idKey(message.ID)]
			c.mu.Unlock()
			if ok {
				reply <- message
			}
			continue
		}
		if message.Method != "" && message.ID == nil && c.notify != nil {
			c.notify(message.Method, message.Params)
		}
	}
}

// Close ends the connection and unblocks pending calls.
func (c *Conn) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		err = c.stream.Close()
	})
	return err
}
