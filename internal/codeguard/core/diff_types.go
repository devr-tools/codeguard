package core

type ChangedFileStatus string

const (
	ChangedFileAdded    ChangedFileStatus = "A"
	ChangedFileModified ChangedFileStatus = "M"
	ChangedFileDeleted  ChangedFileStatus = "D"
)

// ChangedFile describes a file that differs between the diff base ref and the
// working tree, as reported by git diff --name-status.
type ChangedFile struct {
	Path   string            `json:"path"`
	Status ChangedFileStatus `json:"status"`
}
