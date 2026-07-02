package fixtures

// The exec package is imported only to reference an exported sentinel value;
// nothing in this file launches a process.
import "os/exec"

var errProbe = exec.ErrNotFound
