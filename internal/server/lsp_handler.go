package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/protocol"
)

func (h *Handler) handleLsp(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.Type != protocol.MessageLspHover {
		return protocol.Message{}, fmt.Errorf("unsupported message type %q", message.Type)
	}
	if h.lsp == nil {
		return protocol.Message{}, fmt.Errorf("language server is not configured")
	}
	workspace, err := h.store.GetWorkspace(ctx, message.WorkspaceID)
	if err != nil {
		return protocol.Message{}, err
	}
	var request protocol.LspHoverRequest
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid lsp request: %w", err)
	}
	contents, err := h.lsp.Hover(ctx, message.WorkspaceID, workspace.Root, request.Path, request.Line, request.Character)
	if err != nil {
		return protocol.Message{}, err
	}
	payload, err := json.Marshal(protocol.LspHoverResult{Contents: contents})
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: protocol.MessageLspHoverResult, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Payload: payload}, nil
}
