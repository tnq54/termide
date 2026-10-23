package security

import (
	"path/filepath"
	"strings"
)

type Decision string
const (
	Safe Decision = "safe"
	ApprovalRequired Decision = "approval_required"
	Blocked Decision = "blocked"
)

func Classify(command, workspaceRoot string) Decision {
	trimmed := strings.TrimSpace(strings.ToLower(command))
	if trimmed == "" { return Blocked }
	for _, token := range []string{"rm -rf", "sudo ", "git push --force", "deploy --production", "systemctl restart", "chmod 777"} {
		if strings.Contains(trimmed, token) { return ApprovalRequired }
	}
	if strings.Contains(trimmed, "..") && filepath.IsAbs(trimmed) { return ApprovalRequired }
	for _, prefix := range []string{"git status", "pytest", "ruff check", "python -m pytest", "git diff", "ls", "pwd"} {
		if trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ") { return Safe }
	}
	return ApprovalRequired
}
