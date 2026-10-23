package python

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPrefersProjectVenv(t *testing.T) {
	root := t.TempDir()
	venv := filepath.Join(root, ".venv", "bin")
	if err := os.MkdirAll(venv, 0o700); err != nil { t.Fatal(err) }
	pythonPath := filepath.Join(venv, "python")
	if err := os.WriteFile(pythonPath, []byte(""), 0o700); err != nil { t.Fatal(err) }
	got := Discover(root)
	if got.Interpreter != pythonPath { t.Fatalf("interpreter = %q", got.Interpreter) }
}

func TestDiscoverDegradesWithoutEnvironment(t *testing.T) {
	got := Discover(t.TempDir())
	if got.Available { t.Fatal("environment should be unavailable") }
}
