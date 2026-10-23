package editor

import "testing"

func TestBufferUndoRedoAndDirtyState(t *testing.T) {
	b := NewBuffer("x.py", "print('a')\n")
	if err := b.ApplyEdit(0, 0, 0, 0, "# header\n"); err != nil { t.Fatal(err) }
	if !b.Dirty() { t.Fatal("buffer should be dirty") }
	if err := b.Undo(); err != nil { t.Fatal(err) }
	if b.Text() != "print('a')\n" { t.Fatalf("undo text = %q", b.Text()) }
	if err := b.Redo(); err != nil { t.Fatal(err) }
	if b.Text() != "# header\nprint('a')\n" { t.Fatalf("redo text = %q", b.Text()) }
}
