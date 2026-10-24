package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/protocol"
)

func ptyPayload[T any](message protocol.Message, request *T) error {
	if err := json.Unmarshal(message.Payload, request); err != nil {
		return fmt.Errorf("invalid pty request: %w", err)
	}
	return nil
}

func ptyResponse(workspaceID string, msgType protocol.MessageType, payload any) (protocol.Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: msgType, WorkspaceID: workspaceID, Payload: raw}, nil
}

func (h *Handler) ptyWorkspaceRoot(ctx context.Context, message protocol.Message) (string, error) {
	if message.WorkspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}
	workspace, err := h.store.GetWorkspace(ctx, message.WorkspaceID)
	if err != nil {
		return "", err
	}
	return workspace.Root, nil
}

func (h *Handler) ptyOpen(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	root, err := h.ptyWorkspaceRoot(ctx, message)
	if err != nil {
		return protocol.Message{}, err
	}
	var request protocol.PtyOpenRequest
	if err := ptyPayload(message, &request); err != nil {
		return protocol.Message{}, err
	}
	id, err := h.pty.Open(ctx, message.WorkspaceID, root, request.Rows, request.Cols)
	if err != nil {
		return protocol.Message{}, err
	}
	return ptyResponse(message.WorkspaceID, protocol.MessagePtyOpened, protocol.PtyOpened{ID: id})
}

func (h *Handler) ptyWrite(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if _, err := h.ptyWorkspaceRoot(ctx, message); err != nil {
		return protocol.Message{}, err
	}
	var request protocol.PtyTarget
	if err := ptyPayload(message, &request); err != nil {
		return protocol.Message{}, err
	}
	if request.ID == "" {
		return protocol.Message{}, fmt.Errorf("pty session ID is required")
	}
	if err := h.pty.Write(ctx, message.WorkspaceID, request.ID, request.Data); err != nil {
		return protocol.Message{}, err
	}
	return ptyResponse(message.WorkspaceID, protocol.MessagePtyOk, struct{}{})
}

func (h *Handler) ptyRead(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if _, err := h.ptyWorkspaceRoot(ctx, message); err != nil {
		return protocol.Message{}, err
	}
	var request protocol.PtyTarget
	if err := ptyPayload(message, &request); err != nil {
		return protocol.Message{}, err
	}
	if request.ID == "" {
		return protocol.Message{}, fmt.Errorf("pty session ID is required")
	}
	data, next, err := h.pty.Read(ctx, message.WorkspaceID, request.ID, request.After)
	if err != nil {
		return protocol.Message{}, err
	}
	return ptyResponse(message.WorkspaceID, protocol.MessagePtyOutput, protocol.PtyOutput{ID: request.ID, Data: data, Next: next})
}

func (h *Handler) ptyResize(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if _, err := h.ptyWorkspaceRoot(ctx, message); err != nil {
		return protocol.Message{}, err
	}
	var request protocol.PtyTarget
	if err := ptyPayload(message, &request); err != nil {
		return protocol.Message{}, err
	}
	if request.ID == "" {
		return protocol.Message{}, fmt.Errorf("pty session ID is required")
	}
	if err := h.pty.Resize(ctx, message.WorkspaceID, request.ID, request.Rows, request.Cols); err != nil {
		return protocol.Message{}, err
	}
	return ptyResponse(message.WorkspaceID, protocol.MessagePtyOk, struct{}{})
}

func (h *Handler) ptyClose(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if _, err := h.ptyWorkspaceRoot(ctx, message); err != nil {
		return protocol.Message{}, err
	}
	var request protocol.PtyTarget
	if err := ptyPayload(message, &request); err != nil {
		return protocol.Message{}, err
	}
	if request.ID == "" {
		return protocol.Message{}, fmt.Errorf("pty session ID is required")
	}
	if err := h.pty.Close(message.WorkspaceID, request.ID); err != nil {
		return protocol.Message{}, err
	}
	return ptyResponse(message.WorkspaceID, protocol.MessagePtyOk, struct{}{})
}

func (h *Handler) handlePty(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	switch message.Type {
	case protocol.MessagePtyOpen:
		return h.ptyOpen(ctx, message)
	case protocol.MessagePtyWrite:
		return h.ptyWrite(ctx, message)
	case protocol.MessagePtyRead:
		return h.ptyRead(ctx, message)
	case protocol.MessagePtyResize:
		return h.ptyResize(ctx, message)
	case protocol.MessagePtyClose:
		return h.ptyClose(ctx, message)
	default:
		return protocol.Message{}, fmt.Errorf("unsupported message type %q", message.Type)
	}
}
