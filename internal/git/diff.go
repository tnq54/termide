package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type DiffManager struct { root string }
func NewDiffManager(root string) *DiffManager { return &DiffManager{root: root} }
func (m *DiffManager) ChangedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = m.root
	data, err := cmd.Output(); if err != nil { return nil, fmt.Errorf("git diff: %w", err) }
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" { return nil, nil }
	return lines, nil
}
func (m *DiffManager) Patch() (string, error) { cmd := exec.Command("git", "diff", "--no-ext-diff"); cmd.Dir = m.root; data, err := cmd.Output(); return string(data), err }
