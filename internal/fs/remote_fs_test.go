package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRemoteFSRestrictsWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("print('ok')\n"), 0o600); err != nil { t.Fatal(err) }
	fs := NewLocalFS(root)
	data, err := fs.ReadFile("main.py")
	if err != nil { t.Fatal(err) }
	if string(data) != "print('ok')\n" { t.Fatalf("data = %q", data) }
	if _, err := fs.ReadFile("../outside"); err == nil { t.Fatal("expected traversal to fail") }
}
