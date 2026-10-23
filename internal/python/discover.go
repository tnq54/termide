package python

import (
	"os"
	"path/filepath"
)

type Environment struct { Interpreter string; Available bool; Source string }

func Discover(root string) Environment {
	root = filepath.Clean(root)
	for _, candidate := range []struct{ path, source string }{
		{filepath.Join(root, ".venv", "bin", "python"), ".venv"},
		{filepath.Join(root, "venv", "bin", "python"), "venv"},
	} {
		if info, err := os.Stat(candidate.path); err == nil && !info.IsDir() { return Environment{Interpreter: candidate.path, Available: true, Source: candidate.source} }
	}
	return Environment{}
}
