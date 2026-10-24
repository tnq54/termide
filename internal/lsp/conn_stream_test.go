package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestConnReceivesCoalescedNotifications(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	received := make(chan string, 2)
	conn := NewConn(local, func(method string, _ json.RawMessage) { received <- method })
	defer conn.Close()
	var frames bytes.Buffer
	for _, method := range []string{"first", "second"} {
		if err := Write(&frames, Message{Method: method}); err != nil {
			t.Fatal(err)
		}
	}
	go func() { _, _ = peer.Write(frames.Bytes()) }()
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("notification = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing notification %q from coalesced frames", want)
		}
	}
}

// Yield after each transport write to exercise scheduling between a frame's
// header and body without changing the bytes sent over the pipe.
type yieldingConn struct{ net.Conn }

func (c yieldingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	runtime.Gosched()
	return n, err
}

func TestConnConcurrentCallsAndNotifications(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	conn := NewConn(yieldingConn{local}, nil)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Closing the stream also unblocks writes if framing is corrupted.
	go func() { <-ctx.Done(); _ = conn.Close() }()
	const count = 24
	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(peer)
		seen := make(map[string]bool)
		for i := 0; i < count*2; i++ {
			msg, err := Read(reader)
			if err != nil {
				serverDone <- err
				_ = peer.Close()
				return
			}
			if seen[msg.Method] {
				serverDone <- fmt.Errorf("duplicate method %q", msg.Method)
				_ = peer.Close()
				return
			}
			seen[msg.Method] = true
			if msg.ID != nil {
				result, _ := json.Marshal(msg.Method)
				if err := Write(peer, Message{ID: msg.ID, Result: result}); err != nil {
					serverDone <- err
					return
				}
			}
		}
		serverDone <- nil
	}()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			<-start
			method := fmt.Sprintf("call/%d", i)
			result, err := conn.Call(ctx, method, nil)
			if err != nil {
				t.Errorf("%s: %v", method, err)
				return
			}
			var got string
			if err := json.Unmarshal(result, &got); err != nil || got != method {
				t.Errorf("%s response = %s, error = %v", method, result, err)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			<-start
			if err := conn.Notify(fmt.Sprintf("notify/%d", i), nil); err != nil {
				t.Errorf("notify: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if err := <-serverDone; err != nil {
		t.Fatalf("server could not decode concurrent frames: %v", err)
	}
}
