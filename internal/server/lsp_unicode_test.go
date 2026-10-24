package server

import (
	"testing"

	"termide/internal/lsp"
)

func TestApplyTextEditsUTF16(t *testing.T) {
	tests := []struct {
		name, text, want string
		edits            []lsp.TextEdit
	}{
		{
			name:  "rename after emoji",
			text:  "😀name = 1\n",
			want:  "😀total = 1\n",
			edits: []lsp.TextEdit{{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 0, Character: 6}, NewText: "total"}},
		},
		{
			name:  "replace emoji",
			text:  "a😀b",
			want:  "aXb",
			edits: []lsp.TextEdit{{Start: lsp.Position{Line: 0, Character: 1}, End: lsp.Position{Line: 0, Character: 3}, NewText: "X"}},
		},
		{
			name:  "insert at UTF16 line end",
			text:  "😀",
			want:  "😀!",
			edits: []lsp.TextEdit{{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 0, Character: 2}, NewText: "!"}},
		},
		{
			name:  "multiline CRLF",
			text:  "😀a\r\nβ😀z\r\n",
			want:  "😀Xz\r\n",
			edits: []lsp.TextEdit{{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 1, Character: 3}, NewText: "X"}},
		},
		{
			name: "multiple edits retain original offsets",
			text: "😀one 😀two",
			want: "😀first 😀second",
			edits: []lsp.TextEdit{
				{Start: lsp.Position{Line: 0, Character: 2}, End: lsp.Position{Line: 0, Character: 5}, NewText: "first"},
				{Start: lsp.Position{Line: 0, Character: 8}, End: lsp.Position{Line: 0, Character: 11}, NewText: "second"},
			},
		},
		{
			name:  "BMP combining marks stay separate",
			text:  "ếe\u0301x",
			want:  "ếe\u0301y",
			edits: []lsp.TextEdit{{Start: lsp.Position{Line: 0, Character: 3}, End: lsp.Position{Line: 0, Character: 4}, NewText: "y"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyTextEdits(tt.text, tt.edits)
			if err != nil || got != tt.want {
				t.Fatalf("got %q, error %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestApplyTextEditsRejectsInvalidUTF16Boundaries(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		start, end int
	}{
		{"start splits surrogate", "😀abc", 1, 2},
		{"end splits surrogate", "😀abc", 0, 1},
		{"negative", "abc", -1, 0},
		{"past line end", "😀", 3, 3},
		{"inside CRLF", "a\r\nb", 2, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := applyTextEdits(tt.text, []lsp.TextEdit{{
				Start: lsp.Position{Line: 0, Character: tt.start},
				End:   lsp.Position{Line: 0, Character: tt.end}, NewText: "X",
			}})
			if err == nil {
				t.Fatal("invalid UTF-16 boundary accepted")
			}
		})
	}
}
