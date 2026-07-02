package fixtures

// Deploy scripts used to call exec.Command("rsync", ...) before we moved to
// the artifact uploader.
func deployNote() string { return "uploader" }
