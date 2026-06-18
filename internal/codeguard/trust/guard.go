package trust

import "fmt"

// ErrConfigCommandsDisabled is returned when codeguard is asked to run a
// command supplied by repository configuration while command execution from
// config is disabled (the default).
type ErrConfigCommandsDisabled struct {
	// Context describes where the command came from (e.g. the check name).
	Context string
	// Command is the command codeguard refused to run.
	Command string
}

func (e ErrConfigCommandsDisabled) Error() string {
	prefix := ""
	if e.Context != "" {
		prefix = e.Context + ": "
	}
	cmd := e.Command
	if cmd == "" {
		cmd = "<command>"
	}
	return fmt.Sprintf(
		"%srefusing to run config-supplied command %q: command execution from repository "+
			"configuration is disabled by default because codeguard may run against untrusted "+
			"pull requests. Enable it for trusted repositories by setting %s=1 or passing "+
			"--allow-config-commands.",
		prefix, cmd, AllowConfigCommandsEnv)
}

// GuardConfigCommand returns a non-nil error when execution of a config-supplied
// command is not permitted by the active trust policy. context describes the
// origin of the command (used in the error message); it may be empty.
func GuardConfigCommand(context, command string) error {
	if AllowConfigCommands() {
		return nil
	}
	return ErrConfigCommandsDisabled{Context: context, Command: command}
}
