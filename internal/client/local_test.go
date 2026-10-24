package client

import (
	"context"
	"reflect"
	"testing"
)

func TestLocalCommandUsesFramedStdioWithoutShell(t *testing.T) {
	cmd := LocalCommand(context.Background())
	if want := []string{"termide-server", "--stdio"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %q, want %q", cmd.Args, want)
	}
}
