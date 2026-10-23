package domain

type FileDocument struct {
	Path    string `json:"path"`
	Text    string `json:"text"`
	Version string `json:"version"`
}

type FileEntry struct {
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
}
