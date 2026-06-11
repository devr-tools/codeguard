package core

type ScanMode string

const (
	ScanModeFull ScanMode = "full"
	ScanModeDiff ScanMode = "diff"
)

type ScanOptions struct {
	Mode    ScanMode
	BaseRef string
}

type ScanScope struct {
	Mode         ScanMode
	BaseRef      string
	ChangedFiles map[string]struct{}
}
