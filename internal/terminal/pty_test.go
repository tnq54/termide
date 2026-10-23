package terminal

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunnerCapturesCommandOutput(t *testing.T) {
	r := NewRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := r.Run(ctx, "printf 'hello\\n'")
	if err != nil { t.Fatal(err) }
	if strings.TrimSpace(string(out)) != "hello" { t.Fatalf("output = %q", out) }
}
