package lsp

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestConnCallMatchesResponseByID(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		for {
			request, err := Read(right)
			if err != nil {
				return
			}
			if request.ID == nil || request.Method != "textDocument/hover" {
				return
			}
			payload, _ := json.Marshal(map[string]any{"contents": "help text"})
			_ = Write(right, Message{JSONRPC: "2.0", ID: request.ID, Result: payload})
		}
	}()
	conn := NewConn(left, nil)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := conn.Call(ctx, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": "file:///x.py"}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded["contents"] != "help text" {
		t.Fatalf("raw = %s", raw)
	}
}

func TestConnDispatchesNotification(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	got := make(chan string, 1)
	conn := NewConn(left, func(method string, _ json.RawMessage) {
		got <- method
	})
	defer conn.Close()
	_ = Write(right, Message{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics"})
	select {
	case method := <-got:
		if method != "textDocument/publishDiagnostics" {
			t.Fatalf("method = %q", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification not dispatched")
	}
}

func TestConnCallFailsOnErrorResponse(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		request, err := Read(right)
		if err != nil {
			return
		}
		_ = Write(right, Message{JSONRPC: "2.0", ID: request.ID, Error: &Error{Code: -32601, Message: "unknown method"}})
	}()
	conn := NewConn(left, nil)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := conn.Call(ctx, "nope/method", nil); err == nil {
		t.Fatal("error response accepted")
	}
}
