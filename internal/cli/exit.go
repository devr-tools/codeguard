package cli

import (
	"flag"
	"io"
)

// Process exit codes returned by command handlers.
const (
	// exitOK indicates the command completed successfully.
	exitOK = 0
	// exitError indicates the command failed, or a scan/patch reported one or
	// more failed sections.
	exitError = 1
)

// parseFlags parses args into fs, returning ok=false together with the exit
// code a handler should return when parsing fails. Flag errors are reported by
// the FlagSet's own output (set to stderr by callers), so no additional message
// is written here.
func parseFlags(fs *flag.FlagSet, args []string, _ io.Writer) (ok bool, code int) {
	if err := fs.Parse(args); err != nil {
		return false, exitError
	}
	return true, exitOK
}
