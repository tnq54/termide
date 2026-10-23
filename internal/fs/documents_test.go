package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentSavePreservesModeAndDetectsExternalChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.py")
	if err := os.WriteFile(path, []byte("print('xin chào')\n"), 0750); err != nil {
		t.Fatal(err)
	}
	files := NewDocuments(root)
	doc, err := files.Read("main.py")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := files.Save("main.py", doc.Version, "print('updated')\n")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != saved.Text || saved.Version == doc.Version {
		t.Fatal(saved)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0750 {
		t.Fatal(info.Mode())
	}
	if err := os.WriteFile(path, []byte("external edit"), 0750); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Save("main.py", saved.Version, "overwrite"); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "external edit" {
		t.Fatal("external changes lost")
	}
}

func TestDocumentsRefuseEscapesAndNonText(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	files := NewDocuments(root)
	for _, name := range []string{"../secret", "escape/secret", "/etc/passwd"} {
		if _, err := files.Read(name); err == nil {
			t.Fatalf("read escaped root: %s", name)
		}
	}
	for name, text := range map[string]string{"bare-cr": "a\rb", "binary": "a\x00b", "oversize": strings.Repeat("x", MaxDocumentBytes+1)} {
		os.WriteFile(filepath.Join(root, name), []byte(text), 0600)
		if _, err := files.Read(name); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestDocumentsListDirectoriesFirstAndHideSymlinks(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "src"), 0700)
	os.WriteFile(filepath.Join(root, "main.py"), []byte("pass"), 0600)
	os.Symlink("main.py", filepath.Join(root, "link"))
	entries, err := NewDocuments(root).List(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "src" || !entries[0].Directory || entries[1].Name != "main.py" {
		t.Fatal(entries)
	}
}
