package core

// CPPToolingConfig controls optional clang-backed validation for targets that
// explicitly declare a C++ language. Tool modes are off, auto, or required.
// Auto skips an unavailable tool/database, while required reports an
// actionable failure. Command overrides remain subject to the config-command
// trust gate.
type CPPToolingConfig struct {
	ClangFormatMode    string `json:"clang_format_mode,omitempty" yaml:"clang_format_mode,omitempty"`
	ClangFormatCommand string `json:"clang_format_command,omitempty" yaml:"clang_format_command,omitempty"`
	CompilerMode       string `json:"compiler_mode,omitempty" yaml:"compiler_mode,omitempty"`
	CompilerCommand    string `json:"compiler_command,omitempty" yaml:"compiler_command,omitempty"`
	CompileCommands    string `json:"compile_commands,omitempty" yaml:"compile_commands,omitempty"`
}

const (
	ExternalToolModeOff      = "off"
	ExternalToolModeAuto     = "auto"
	ExternalToolModeRequired = "required"
)
