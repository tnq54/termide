package server

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPTYEchoesWrittenInput(t *testing.T) {
	r := NewPTYRegistry()
	defer r.CloseAll()
	ctx := context.Background()
	id, err := r.Open(ctx, t.TempDir(), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Write(ctx, id, []byte("echo hello-pty\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, next, err := r.Read(ctx, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "hello-pty") {
			if next != uint64(len(data)) {
				t.Fatalf("next = %d, len = %d", next, len(data))
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no echo: %q", data)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestPTYReadReturnsOnlyNewBytes(t *testing.T) {
	r := NewPTYRegistry()
	defer r.CloseAll()
	ctx := context.Background()
	id, err := r.Open(ctx, t.TempDir(), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Write(ctx, id, []byte("echo marker-one\n")); err != nil {
		t.Fatal(err)
	}
	var next uint64
	deadline := time.Now().Add(5 * time.Second)
	for {
		full, cursor, err := r.Read(ctx, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(full), "marker-one") {
			next = cursor
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing marker-one in %q", full)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := r.Write(ctx, id, []byte("echo marker-two\n")); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		data, _, err := r.Read(ctx, id, next)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "marker-two") {
			if strings.Contains(string(data), "marker-one") {
				t.Fatalf("stale bytes returned: %q", data)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing marker-two in %q", data)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestPTYUnknownSessionErrors(t *testing.T) {
	r := NewPTYRegistry()
	defer r.CloseAll()
	ctx := context.Background()
	if err := r.Write(ctx, "missing", []byte("x")); err == nil {
		t.Fatal("write to missing session accepted")
	}
	if _, _, err := r.Read(ctx, "missing", 0); err == nil {
		t.Fatal("read from missing session accepted")
	}
	if err := r.Resize(ctx, "missing", 24, 80); err == nil {
		t.Fatal("resize of missing session accepted")
	}
	if err := r.Close("missing"); err == nil {
		t.Fatal("close of missing session accepted")
	}
}

func TestPTYReapsIdleSessions(t *testing.T) {
	r := NewPTYRegistry()
	r.idle = 100 * time.Millisecond
	defer r.CloseAll()
	ctx := context.Background()
	id, err := r.Open(ctx, t.TempDir(), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := r.Open(ctx, t.TempDir(), 24, 80); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Read(ctx, id, 0); err == nil {
		t.Fatal("idle session was not reaped")
	}
}
