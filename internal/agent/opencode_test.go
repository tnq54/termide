package agent

import (
	"context"
	"testing"
)

func TestOpenCodeRunnerBuildsSafeProcessSpec(t *testing.T) {
	r := NewOpenCodeRunner("opencode", []string{"--model", "provider/model"})
	cmd, err := r.Command(Spec{ID: "a1", Role: "coder", Command: "implement feature"})
	if err != nil { t.Fatal(err) }
	if cmd.Path == "" { t.Fatal("expected executable path") }
	if len(cmd.Args) != 5 || cmd.Args[1] != "--model" || cmd.Args[3] != "run" { t.Fatalf("args = %#v", cmd.Args) }
	if err := r.Run(context.Background(), Spec{ID: "a1", Role: "coder", Command: "printf ok"}, func(Event) {}); err == nil { t.Fatal("expected missing/invalid process result in test environment") }
}
