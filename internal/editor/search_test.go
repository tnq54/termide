package editor

import "testing"

func TestSearchFindsAllMatches(t *testing.T) {
	got := Search("print('x')\nprint('y')\n", "print")
	if len(got) != 2 || got[0].Line != 1 || got[1].Line != 2 { t.Fatalf("matches = %#v", got) }
}
