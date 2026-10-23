package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeRunnerBuildsSafeProcessSpec(t *testing.T) {
	r := NewOpenCodeRunner("opencode", []string{"--model", "provider/model"})
	cmd, err := r.Command(Spec{ID: "a1", Role: "coder", Command: "implement feature"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path == "" {
		t.Fatal("expected executable path")
	}
	if len(cmd.Args) != 5 || cmd.Args[1] != "--model" || cmd.Args[3] != "run" {
		t.Fatalf("args = %#v", cmd.Args)
	}
}

func TestOpenCodeRunnerUsesWorktreeAndCapturesStderr(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(t.TempDir(), "fake-opencode")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\npwd\nprintf 'diagnostic\\n' >&2\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r := NewOpenCodeRunner(binary, nil)
	var output []string
	err := r.Run(context.Background(), Spec{ID: "a", WorkingDir: root, Command: "task"}, func(e Event) { output = append(output, e.Output) })
	if err == nil {
		t.Fatal("process exit failure lost")
	}
	text := strings.Join(output, "\n")
	if !strings.Contains(text, root) || !strings.Contains(text, "diagnostic") {
		t.Fatalf("output=%q", text)
	}
}
