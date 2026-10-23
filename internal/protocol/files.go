package protocol

import "termide/internal/domain"

const (
	MessageListFiles  MessageType = "files.list"
	MessageReadFile   MessageType = "files.read"
	MessageSaveFile   MessageType = "files.save"
	MessageFileResult MessageType = "files.result"
)

type FileRequest struct {
	Path    string `json:"path"`
	Text    string `json:"text"`
	Version string `json:"version,omitempty"`
}
type FileResponse struct {
	Path     string               `json:"path"`
	Document *domain.FileDocument `json:"document,omitempty"`
	Entries  []domain.FileEntry   `json:"entries,omitempty"`
}
