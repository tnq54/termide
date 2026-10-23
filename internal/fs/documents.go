package fs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
	"termide/internal/domain"
)

const MaxDocumentBytes = 1024 * 1024
const MaxDirectoryEntries = 2000

var ErrConflict = errors.New("file changed on server; reopen before saving")

// Serializes compare-and-save operations in this server process. The version
// check detects changes already present before saving, not arbitrary writes by
// other processes between checking and rename.
var documentSaveMu sync.Mutex

type Documents struct{ root string }

func NewDocuments(root string) *Documents { return &Documents{root: root} }

func documentPath(name string) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("path must remain inside workspace")
	}
	return nil
}
func textAllowed(text string) bool {
	if strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\r") {
		return false
	}
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' || r == 127 || r >= 128 && r <= 159 {
			return false
		}
	}
	return true
}
func readDocument(root *os.Root, name string) (domain.FileDocument, os.FileMode, error) {
	if err := documentPath(name); err != nil {
		return domain.FileDocument{}, 0, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return domain.FileDocument{}, 0, err
	}
	if !info.Mode().IsRegular() {
		return domain.FileDocument{}, 0, fmt.Errorf("only regular text files are editable")
	}
	f, err := root.Open(name)
	if err != nil {
		return domain.FileDocument{}, 0, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return domain.FileDocument{}, 0, err
	}
	if !info.Mode().IsRegular() {
		return domain.FileDocument{}, 0, fmt.Errorf("not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxDocumentBytes+1))
	if err != nil {
		return domain.FileDocument{}, 0, err
	}
	if len(data) > MaxDocumentBytes {
		return domain.FileDocument{}, 0, fmt.Errorf("file exceeds 1 MiB editor limit")
	}
	text := string(data)
	if !textAllowed(text) {
		return domain.FileDocument{}, 0, fmt.Errorf("file is not supported UTF-8 text")
	}
	return domain.FileDocument{Path: name, Text: text, Version: fmt.Sprintf("%x", sha256.Sum256(data))}, info.Mode().Perm(), nil
}
func (d *Documents) Read(name string) (domain.FileDocument, error) {
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return domain.FileDocument{}, err
	}
	defer root.Close()
	doc, _, err := readDocument(root, name)
	return doc, err
}
func (d *Documents) List(name string) ([]domain.FileEntry, error) {
	if err := documentPath(name); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(MaxDirectoryEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > MaxDirectoryEntries {
		return nil, fmt.Errorf("directory exceeds 2000-entry limit")
	}
	out := make([]domain.FileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.Name() == ".git" {
			continue
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			continue
		}
		out = append(out, domain.FileEntry{Name: entry.Name(), Directory: entry.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Directory != out[j].Directory {
			return out[i].Directory
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
func (d *Documents) Save(name, version, text string) (domain.FileDocument, error) {
	if err := documentPath(name); err != nil {
		return domain.FileDocument{}, err
	}
	if len(text) > MaxDocumentBytes || !textAllowed(text) {
		return domain.FileDocument{}, fmt.Errorf("unsupported or oversized text")
	}
	documentSaveMu.Lock()
	defer documentSaveMu.Unlock()
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return domain.FileDocument{}, err
	}
	defer root.Close()
	// Pin the containing directory for the complete save operation.
	parent, err := root.OpenRoot(filepath.Dir(name))
	if err != nil {
		return domain.FileDocument{}, err
	}
	defer parent.Close()
	base := filepath.Base(name)
	doc, mode, err := readDocument(parent, base)
	if err != nil {
		return domain.FileDocument{}, err
	}
	if version == "" || doc.Version != version {
		return domain.FileDocument{}, ErrConflict
	}
	temp := ".termide-save-" + uuid.NewString()
	f, err := parent.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return domain.FileDocument{}, err
	}
	defer parent.Remove(temp)
	_, writeErr := io.Copy(f, strings.NewReader(text))
	if writeErr == nil {
		writeErr = f.Chmod(mode)
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return domain.FileDocument{}, err
	}
	if err := parent.Rename(temp, base); err != nil {
		return domain.FileDocument{}, err
	}
	return domain.FileDocument{Path: name, Text: text, Version: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))}, nil
}
