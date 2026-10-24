package server

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

// TestStdioBootstrapsDaemonWhenNoneRuns exercises the real executable:
// with no supervisor listening, --stdio must spawn the daemon itself and
// then serve open_workspace over the framed transport.
func TestStdioBootstrapsDaemonWhenNoneRuns(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "termide-server")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/termide-server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build test binary: %v\n%s", err, out)
	}
	dir := t.TempDir()
	sock := filepath.Join(dir, "termide.sock")
	db := filepath.Join(dir, "state.db")
	root := t.TempDir()

	cmd := exec.Command(bin, "--stdio", "--socket", sock, "--database", db, "--opencode", "true")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	payload, _ := json.Marshal(domain.Workspace{ID: "w", Root: root, Name: "demo"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := protocol.Write(stdin, protocol.Message{Type: protocol.MessageOpenWorkspace, WorkspaceID: "w", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	type reply struct {
		message protocol.Message
		err     error
	}
	done := make(chan reply, 1)
	go func() {
		message, err := protocol.Read(stdout)
		done <- reply{message, err}
	}()
	select {
	case <-ctx.Done():
		t.Fatal("no response from bootstrapped supervisor")
	case result := <-done:
		if result.err != nil {
			t.Fatalf("read response: %v", result.err)
		}
		if result.message.Type != protocol.MessageWorkspaceReady {
			t.Fatalf("response = %q, want %q", result.message.Type, protocol.MessageWorkspaceReady)
		}
		if result.message.WorkspaceID != "w" {
			t.Fatalf("workspace = %q, want %q", result.message.WorkspaceID, "w")
		}
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("daemon socket missing after bootstrap: %v", err)
	}
}
