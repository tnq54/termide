package git

import (
	"strings"
	"testing"
)

func TestWorktreeRejectsPathLikeIDsBeforeGit(t *testing.T) {
	m := NewWorktreeManager(t.TempDir())
	for _, id := range []string{"../escape", "/absolute", "a/b", "", "-option"} {
		if _, err := m.Create(id, "a", "HEAD"); err == nil || !strings.Contains(err.Error(), "invalid identifier") {
			t.Fatalf("%q: %v", id, err)
		}
	}
}
