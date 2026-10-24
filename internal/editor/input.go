package editor

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Input is a source-preserving multiline editor. Columns are rune offsets;
// rendering uses terminal-cell widths. Tabs/CRLF are never rewritten on open.
type Input struct {
	text                     []rune
	cursor                   int
	width, height, top, left int
	newline                  string
}

func NewInput() Input            { return Input{width: 40, height: 10, newline: "\n"} }
func (m *Input) SetWidth(n int)  { m.width = max(1, n); m.ensureVisible() }
func (m *Input) SetHeight(n int) { m.height = max(1, n); m.ensureVisible() }
func (m *Input) Focus() tea.Cmd  { return nil }
func (m *Input) Blur()           {}
func (m *Input) SetValue(text string) {
	m.text = []rune(text)
	m.cursor = len(m.text)
	m.newline = "\n"
	if strings.Contains(text, "\r\n") {
		m.newline = "\r\n"
	}
	m.ensureVisible()
}
func (m Input) Value() string { return string(m.text) }
func (m Input) lineStart() int {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}
func (m Input) lineEnd(start int) int {
	for i := start; i < len(m.text); i++ {
		if m.text[i] == '\n' {
			if i > start && m.text[i-1] == '\r' {
				return i - 1
			}
			return i
		}
	}
	return len(m.text)
}
func isWordRune(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 127
}

// ReplaceWordBeforeCursor deletes the word characters immediately before the
// cursor and inserts text in their place, leaving the cursor after it.
func (m *Input) ReplaceWordBeforeCursor(text string) {
	start := m.cursor
	for start > 0 && isWordRune(m.text[start-1]) {
		start--
	}
	m.text = append(append([]rune{}, m.text[:start]...), append([]rune(text), m.text[m.cursor:]...)...)
	m.cursor = start + len([]rune(text))
	m.ensureVisible()
}

func (m *Input) insert(text string) {
	if !utf8.ValidString(text) {
		return
	}
	for _, r := range text {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' || r == 127 || r >= 128 && r <= 159 {
			return
		}
	}
	if len(string(m.text))+len(text) > 1024*1024 {
		return
	}
	replacement := []rune(text)
	out := make([]rune, 0, len(m.text)+len(replacement))
	out = append(out, m.text[:m.cursor]...)
	out = append(out, replacement...)
	out = append(out, m.text[m.cursor:]...)
	m.text = out
	m.cursor += len(replacement)
}
func (m *Input) vertical(direction int) {
	start := m.lineStart()
	col := m.cursor - start
	if direction < 0 {
		if start == 0 {
			return
		}
		prevEnd := start - 1
		if prevEnd > 0 && m.text[prevEnd-1] == '\r' {
			prevEnd--
		}
		prev := start - 2
		for prev >= 0 && m.text[prev] != '\n' {
			prev--
		}
		prev++
		m.cursor = prev + min(col, prevEnd-prev)
	} else {
		next := start
		for next < len(m.text) && m.text[next] != '\n' {
			next++
		}
		if next == len(m.text) {
			return
		}
		next++
		m.cursor = next + min(col, m.lineEnd(next)-next)
	}
}
func (m Input) Update(message tea.Msg) (Input, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyRunes:
		text := strings.ReplaceAll(string(key.Runes), "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		m.insert(strings.ReplaceAll(text, "\n", m.newline))
	case tea.KeySpace:
		m.insert(" ")
	case tea.KeyTab:
		m.insert("\t")
	case tea.KeyEnter:
		m.insert(m.newline)
	case tea.KeyLeft:
		if m.cursor > 0 {
			m.cursor--
			if m.text[m.cursor] == '\n' && m.cursor > 0 && m.text[m.cursor-1] == '\r' {
				m.cursor--
			}
		}
	case tea.KeyRight:
		if m.cursor < len(m.text) {
			if m.text[m.cursor] == '\r' && m.cursor+1 < len(m.text) && m.text[m.cursor+1] == '\n' {
				m.cursor++
			}
			m.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		m.cursor = m.lineStart()
	case tea.KeyEnd, tea.KeyCtrlE:
		m.cursor = m.lineEnd(m.lineStart())
	case tea.KeyUp:
		m.vertical(-1)
	case tea.KeyDown:
		m.vertical(1)
	case tea.KeyPgUp:
		for i := 0; i < m.height; i++ {
			m.vertical(-1)
		}
	case tea.KeyPgDown:
		for i := 0; i < m.height; i++ {
			m.vertical(1)
		}
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.cursor > 0 {
			start := m.cursor - 1
			if m.text[start] == '\n' && start > 0 && m.text[start-1] == '\r' {
				start--
			}
			m.text = append(m.text[:start], m.text[m.cursor:]...)
			m.cursor = start
		}
	case tea.KeyDelete:
		if m.cursor < len(m.text) {
			end := m.cursor + 1
			if m.text[m.cursor] == '\r' && end < len(m.text) && m.text[end] == '\n' {
				end++
			}
			m.text = append(m.text[:m.cursor], m.text[end:]...)
		}
	}
	m.ensureVisible()
	return m, nil
}

// CursorLineCol returns the zero-based line and rune-offset column.
func (m Input) CursorLineCol() (int, int) { return m.position() }

func (m Input) position() (int, int) {
	row, col := 0, 0
	for _, r := range m.text[:m.cursor] {
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	return row, col
}
func displayText(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' {
			return -1
		}
		if r < 32 && r != '\t' || r == 127 || r >= 128 && r <= 159 {
			return ' '
		}
		return r
	}, strings.ReplaceAll(text, "\t", "    "))
}
func (m *Input) ensureVisible() {
	row, _ := m.position()
	if row < m.top {
		m.top = row
	}
	if row >= m.top+m.height {
		m.top = row - m.height + 1
	}
	cells := ansi.StringWidth(displayText(string(m.text[m.lineStart():m.cursor])))
	available := max(1, m.width-6)
	if cells < m.left {
		m.left = cells
	}
	if cells >= m.left+available {
		m.left = cells - available + 1
	}
}
func (m Input) View() string {
	lines := strings.Split(m.Value(), "\n")
	row, col := m.position()
	rendered := make([]string, 0, m.height)
	for i := m.top; i < len(lines) && len(rendered) < m.height; i++ {
		line := lines[i]
		if i == row {
			chars := []rune(line)
			at := min(col, len(chars))
			prefix := displayText(string(chars[:at]))
			cursor := " "
			suffix := ""
			if at < len(chars) {
				cursor = displayText(string(chars[at]))
				if cursor == "" {
					cursor = " "
				}
				suffix = displayText(string(chars[at+1:]))
			}
			line = prefix + "\x1b[7m" + cursor + "\x1b[0m" + suffix
		} else {
			line = displayText(line)
		}
		gutter := fmt.Sprintf("%4d ", i+1)
		available := max(1, m.width-ansi.StringWidth(gutter))
		rendered = append(rendered, ansi.Truncate(gutter+ansi.Cut(line, m.left, m.left+available), m.width, ""))
	}
	return strings.Join(rendered, "\n")
}
