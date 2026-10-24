package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func terminalFixture(t *testing.T) (*store.Store, *Handler, string) {
	t.Helper()
	root := t.TempDir()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(context.Background(), domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	return s, NewHandler(s), root
}

func runCommand(t *testing.T, h *Handler, workspace, command string) protocol.CommandResult {
	t.Helper()
	response, err := h.Handle(context.Background(), protocol.Message{
		Type: protocol.MessageRunCommand, WorkspaceID: workspace,
		Payload: mustJSON(t, protocol.CommandRequest{Command: command}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageCommandResult || response.WorkspaceID != workspace {
		t.Fatalf("response = %#v", response)
	}
	var result protocol.CommandResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRunCommandExecutesSafeCommandInWorkspaceRoot(t *testing.T) {
	_, h, root := terminalFixture(t)
	result := runCommand(t, h, "w", "pwd")
	if result.ExitCode != 0 || strings.TrimSpace(result.Output) != root {
		t.Fatalf("result = %#v, root = %q", result, root)
	}
}

func TestRunCommandRejectsBlockedAndApprovalCommands(t *testing.T) {
	_, h, _ := terminalFixture(t)
	for _, command := range []string{"", "sudo reboot", "rm -rf /tmp/x"} {
		payload, _ := json.Marshal(protocol.CommandRequest{Command: command})
		if _, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageRunCommand, WorkspaceID: "w", Payload: payload}); err == nil {
			t.Fatalf("command accepted: %q", command)
		}
	}
}

func TestRunCommandRejectsForeignWorkspace(t *testing.T) {
	s, h, _ := terminalFixture(t)
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "other", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(protocol.CommandRequest{Command: "pwd"})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageRunCommand, WorkspaceID: "other", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageRunCommand, WorkspaceID: "missing", Payload: payload}); err == nil {
		t.Fatal("unknown workspace accepted")
	}
}

func TestRunCommandTruncatesLargeOutput(t *testing.T) {
	_, h, root := terminalFixture(t)
	for i := 0; i < 6000; i++ {
		name := filepath.Join(root, "file-"+strings.Repeat("x", 20)+"-"+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+string(rune('a'+(i/676)%26)))
		if err := os.WriteFile(name, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
	}
	result := runCommand(t, h, "w", "ls")
	if !result.Truncated || len(result.Output) > 128*1024 {
		t.Fatalf("result not bounded: len=%d truncated=%v", len(result.Output), result.Truncated)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
