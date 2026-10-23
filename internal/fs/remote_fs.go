package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

type LocalFS struct { root string }

func NewLocalFS(root string) *LocalFS { return &LocalFS{root: filepath.Clean(root)} }

func (f *LocalFS) path(name string) (string, error) {
	clean := filepath.Clean(filepath.Join(f.root, name))
	rel, err := filepath.Rel(f.root, clean)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("path outside workspace: %s", name)
	}
	return clean, nil
}

func (f *LocalFS) ReadFile(name string) ([]byte, error) {
	path, err := f.path(name); if err != nil { return nil, err }
	return os.ReadFile(path)
}
func (f *LocalFS) WriteFile(name string, data []byte) error {
	path, err := f.path(name); if err != nil { return err }
	return os.WriteFile(path, data, 0o600)
}
func (f *LocalFS) List(name string) ([]os.DirEntry, error) {
	path, err := f.path(name); if err != nil { return nil, err }
	return os.ReadDir(path)
}
