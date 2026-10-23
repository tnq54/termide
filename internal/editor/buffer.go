package editor

import (
	"fmt"
	"strings"
)

type edit struct { start, end int; inserted, removed string }
type Buffer struct { name, text, saved string; undo, redo []edit }

func NewBuffer(name, text string) *Buffer { return &Buffer{name: name, text: text, saved: text} }
func (b *Buffer) Text() string { return b.text }
func (b *Buffer) Dirty() bool { return b.text != b.saved }

func (b *Buffer) ApplyEdit(startLine, startCol, endLine, endCol int, replacement string) error {
	start, end, err := b.offsets(startLine, startCol, endLine, endCol)
	if err != nil { return err }
	e := edit{start: start, end: end, inserted: replacement, removed: b.text[start:end]}
	b.text = b.text[:start] + replacement + b.text[end:]
	b.undo = append(b.undo, e); b.redo = nil
	return nil
}
func (b *Buffer) Undo() error {
	if len(b.undo) == 0 { return fmt.Errorf("nothing to undo") }
	e := b.undo[len(b.undo)-1]; b.undo = b.undo[:len(b.undo)-1]
	start := e.start; b.text = b.text[:start] + e.removed + b.text[start+len(e.inserted):]; b.redo = append(b.redo, e); return nil
}
func (b *Buffer) Redo() error {
	if len(b.redo) == 0 { return fmt.Errorf("nothing to redo") }
	e := b.redo[len(b.redo)-1]; b.redo = b.redo[:len(b.redo)-1]
	b.text = b.text[:e.start] + e.inserted + b.text[e.start+len(e.removed):]; b.undo = append(b.undo, e); return nil
}
func (b *Buffer) Save() { b.saved = b.text }
func (b *Buffer) offsets(sl, sc, el, ec int) (int, int, error) {
	lines := strings.Split(b.text, "\n")
	if sl < 0 || el < sl || sl >= len(lines) || el >= len(lines) || sc < 0 || ec < 0 || sc > len(lines[sl]) || ec > len(lines[el]) { return 0, 0, fmt.Errorf("invalid edit range") }
	offset := func(line, col int) int { n := 0; for i := 0; i < line; i++ { n += len(lines[i])+1 }; return n+col }
	return offset(sl, sc), offset(el, ec), nil
}
