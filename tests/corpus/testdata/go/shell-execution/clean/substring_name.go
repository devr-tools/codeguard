package fixtures

type runner struct{}

// Command returns the argument unchanged; it never launches a process.
func (runner) Command(name string) string { return name }

var safeexec runner

func listFilesLabel() string {
	return safeexec.Command("ls")
}
