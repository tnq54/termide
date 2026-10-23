package server

import (
	"context"
	"encoding/json"
	"fmt"
	"termide/internal/fs"
	"termide/internal/protocol"
)

func (h *Handler) handleFiles(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	workspace, err := h.store.GetWorkspace(ctx, message.WorkspaceID)
	if err != nil {
		return protocol.Message{}, err
	}
	var request protocol.FileRequest
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		return protocol.Message{}, err
	}
	if message.Type == protocol.MessageSaveFile {
		var fields struct {
			Text *string `json:"text"`
		}
		if err := json.Unmarshal(message.Payload, &fields); err != nil {
			return protocol.Message{}, err
		}
		if fields.Text == nil {
			return protocol.Message{}, fmt.Errorf("save requires explicit string text")
		}
	}
	files := fs.NewDocuments(workspace.Root)
	response := protocol.FileResponse{Path: request.Path}
	switch message.Type {
	case protocol.MessageListFiles:
		response.Entries, err = files.List(request.Path)
	case protocol.MessageReadFile:
		doc, readErr := files.Read(request.Path)
		err = readErr
		response.Document = &doc
	case protocol.MessageSaveFile:
		doc, saveErr := files.Save(request.Path, request.Version, request.Text)
		err = saveErr
		response.Document = &doc
	}
	if err != nil {
		return protocol.Message{}, err
	}
	payload, err := json.Marshal(response)
	return protocol.Message{Type: protocol.MessageFileResult, WorkspaceID: workspace.ID, Payload: payload}, err
}
