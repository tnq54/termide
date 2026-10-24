package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/protocol"
)

type hoverResult struct {
	text string
	err  error
}

func (m Model) hoverCommand(path string, line, character int) tea.Cmd {
	payload, err := json.Marshal(protocol.LspHoverRequest{Path: path, Line: line, Character: character})
	if err != nil {
		return func() tea.Msg { return hoverResult{err: err} }
	}
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageLspHover, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return hoverResult{err: err}
		}
		if response.Type != protocol.MessageLspHoverResult || response.WorkspaceID != m.workspace.ID {
			return hoverResult{err: fmt.Errorf("unexpected hover response")}
		}
		var result protocol.LspHoverResult
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			return hoverResult{err: err}
		}
		return hoverResult{text: result.Contents}
	}
}

func (m Model) receiveHover(result hoverResult) (tea.Model, tea.Cmd) {
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	if strings.TrimSpace(result.text) == "" {
		m.Status = "no hover available"
		return m, nil
	}
	m.hover = result.text
	return m, nil
}

// renderHover appends a popup box after the first body line.
func renderHover(lines []string, text string, width int) []string {
	if strings.TrimSpace(text) == "" || len(lines) == 0 {
		return lines
	}
	popup := []string{"--- hover (Esc) ---"}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(safeLabel(line))
		if line == "" {
			continue
		}
		for len(line) > 0 {
			cut := line
			if len(cut) > width-2 {
				cut = cut[:width-2]
			}
			popup = append(popup, "> "+cut)
			line = line[len(cut):]
			if len(popup) > 12 {
				popup = append(popup, "> ...")
				break
			}
		}
		if len(popup) > 12 {
			break
		}
	}
	out := make([]string, 0, len(lines)+len(popup))
	out = append(out, lines[0])
	out = append(out, popup...)
	return append(out, lines[1:]...)
}
