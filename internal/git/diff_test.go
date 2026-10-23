package git

import (
	"os"
	"os/exec"
	"testing"
)

func TestDiffManagerReturnsChangedFiles(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) { c := exec.Command("git", args...); c.Dir = root; if out, err := c.CombinedOutput(); err != nil { t.Fatalf("git %v: %v %s", args, err, out) } }
	run("init", "-q"); run("config", "user.email", "test@example.com"); run("config", "user.name", "Test")
	if err := os.WriteFile(root+"/a.py", []byte("print(1)\n"), 0o600); err != nil { t.Fatal(err) }
	run("add", "."); run("commit", "-qm", "base")
	if err := os.WriteFile(root+"/a.py", []byte("print(2)\n"), 0o600); err != nil { t.Fatal(err) }
	got, err := NewDiffManager(root).ChangedFiles()
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0] != "a.py" { t.Fatalf("changed = %#v", got) }
}
