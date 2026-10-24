//go:build linux

package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOpenCodeCancelStopsChildHoldingOutputPipe(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "fake-opencode")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nsleep 60 &\necho $!\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := make(chan int, 1)
	done := make(chan error, 1)
	go func() {
		done <- NewOpenCodeRunner(binary, nil).Run(ctx, Spec{Command: "task"}, func(e Event) {
			pid, err := strconv.Atoi(e.Output)
			if err == nil {
				child <- pid
			}
		})
	}()
	var pid int
	select {
	case pid = <-child:
	case <-time.After(3 * time.Second):
		t.Fatal("child did not start")
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel reported success")
		}
	case <-time.After(2 * time.Second):
		syscall.Kill(pid, syscall.SIGKILL)
		<-done
		t.Fatal("runner hung on output pipe held by child after cancellation")
	}
	// A killed orphan may remain a zombie until the container init reaps it.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(data)[strings.LastIndex(string(data), ")")+1:])
		if len(fields) > 0 && fields[0] == "Z" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child remains alive after Run returned")
}
