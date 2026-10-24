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

type diagnosticsResult struct {
	path  string
	items []protocol.LspDiagnostic
	err   error
}

func (m Model) diagnosticsCommand(path string) tea.Cmd {
	payload, err := json.Marshal(protocol.LspDiagnosticsRequest{Path: path})
	if err != nil {
		return func() tea.Msg { return diagnosticsResult{path: path, err: err} }
	}
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageLspDiagnostics, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return diagnosticsResult{path: path, err: err}
		}
		if response.Type != protocol.MessageLspDiagnosticsResult || response.WorkspaceID != m.workspace.ID {
			return diagnosticsResult{path: path, err: fmt.Errorf("unexpected diagnostics response")}
		}
		var result protocol.LspDiagnosticsResult
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			return diagnosticsResult{path: path, err: err}
		}
		return diagnosticsResult{path: path, items: result.Items}
	}
}

func (m Model) receiveDiagnostics(result diagnosticsResult) (tea.Model, tea.Cmd) {
	if m.files.document == nil || m.files.document.Path != result.path {
		return m, nil
	}
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	m.diags = m.diags[:0]
	for _, item := range result.items {
		m.diags = append(m.diags, formatDiagnostic(item))
	}
	if len(m.diags) == 0 {
		m.Status = "no problems"
	} else {
		m.Status = fmt.Sprintf("%d problem(s)", len(m.diags))
	}
	return m, nil
}

func formatDiagnostic(item protocol.LspDiagnostic) string {
	severity := "hint"
	switch item.Severity {
	case 1:
		severity = "error"
	case 2:
		severity = "warning"
	case 3:
		severity = "info"
	}
	return fmt.Sprintf("line %d: [%s] %s", item.Line+1, severity, strings.TrimSpace(item.Message))
}

// renderProblems appends a capped problems section after the first body line.
func renderProblems(lines []string, diags []string, width int) []string {
	if len(diags) == 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines)+len(diags)+1)
	out = append(out, lines[0])
	out = append(out, "--- problems ---")
	for i, diag := range diags {
		if i >= 8 {
			out = append(out, "> ...")
			break
		}
		text := strings.TrimSpace(safeLabel(diag))
		if len(text) > width-2 {
			text = text[:width-2]
		}
		out = append(out, "> "+text)
	}
	return append(out, lines[1:]...)
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
