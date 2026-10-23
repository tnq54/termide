package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func (h *Handler) openWorkspace(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	var w domain.Workspace
	if err := json.Unmarshal(message.Payload, &w); err != nil {
		return protocol.Message{}, err
	}
	if w.ID == "" || !filepath.IsAbs(w.Root) {
		return protocol.Message{}, fmt.Errorf("workspace ID and absolute root required")
	}
	root, err := filepath.EvalSymlinks(w.Root)
	if err != nil {
		return protocol.Message{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return protocol.Message{}, err
	}
	if !info.IsDir() {
		return protocol.Message{}, fmt.Errorf("workspace root must be a directory")
	}
	w.Root = filepath.Clean(root)
	existing, err := h.store.GetWorkspace(ctx, w.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := h.store.CreateWorkspace(ctx, w); err != nil {
			return protocol.Message{}, err
		}
	case err != nil:
		return protocol.Message{}, err
	default:
		if existing.Root != w.Root {
			return protocol.Message{}, fmt.Errorf("workspace ID already belongs to another root")
		}
		w = existing
	}
	payload, err := json.Marshal(w)
	return protocol.Message{Type: protocol.MessageWorkspaceReady, WorkspaceID: w.ID, Payload: payload}, err
}
