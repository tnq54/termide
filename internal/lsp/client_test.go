package lsp

import (
	"context"
	"testing"
)

func TestClientReportsUnavailableLanguageServer(t *testing.T) {
	client := NewClient("binary-that-does-not-exist", nil)
	if err := client.Start(context.Background(), t.TempDir()); err == nil {
		t.Fatal("expected unavailable server error")
	}
}
