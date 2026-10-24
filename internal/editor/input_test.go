package editor

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestInputPreservesTabsCRLFAndUnicode(t *testing.T) {
	m := NewInput()
	original := "def hello():\r\n\tprint('chào')\r\n"
	m.SetValue(original)
	if m.Value() != original {
		t.Fatal("source normalized on open")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("# sửa")})
	if m.Value() != original+"# sửa" {
		t.Fatal(m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.HasSuffix(m.Value(), "# sửa\r\n") {
		t.Fatal("newline convention lost")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.HasSuffix(m.Value(), "# sửa") {
		t.Fatal("CRLF not removed as pair")
	}
}

func TestReplaceWordBeforeCursor(t *testing.T) {
	m := NewInput()
	m.SetValue("x = foo")
	m.ReplaceWordBeforeCursor("foobar")
	if m.Value() != "x = foobar" {
		t.Fatalf("value = %q", m.Value())
	}
	m.ReplaceWordBeforeCursor("")
	if m.Value() != "x = " {
		t.Fatalf("value = %q", m.Value())
	}
	m.SetValue("x = 1 + bar2")
	// Move cursor left twice: inside "bar2".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m.ReplaceWordBeforeCursor("baz(")
	if m.Value() != "x = 1 + baz(r2" {
		t.Fatalf("value = %q", m.Value())
	}
}

func TestInputNavigationAndDelete(t *testing.T) {
	m := NewInput()
	m.SetValue("ab\ncd")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Value() != "abcd" {
		t.Fatal(m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if m.Value() != "abd" {
		t.Fatal(m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ế")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Value() != "abd" {
		t.Fatal(m.Value())
	}
}

func TestInputViewportIsBounded(t *testing.T) {
	m := NewInput()
	m.SetWidth(12)
	m.SetHeight(3)
	m.SetValue(strings.Repeat("long long line 界\n", 200) + "end")
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > 3 {
		t.Fatal("viewport overflow")
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > 12 {
			t.Fatal("horizontal overflow")
		}
	}
	if !strings.Contains(ansi.Strip(view), "end") {
		t.Fatal("cursor is off-screen")
	}
}

func TestPasteNormalizesStandaloneCarriageReturn(t *testing.T) {
	m := NewInput()
	m.SetValue("")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\rb\r\nc"), Paste: true})
	if m.Value() != "a\nb\nc" {
		t.Fatalf("hidden carriage return: %q", m.Value())
	}
}
