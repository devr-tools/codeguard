package design

import "regexp"

var (
	pythonClassDeclPattern    = regexp.MustCompile(`^class\s+([A-Za-z_]\w*)\s*(?:\((.*)\))?\s*:`)
	pythonMethodDeclPattern   = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pythonProtocolAttrPattern = regexp.MustCompile(`^([A-Za-z_]\w*)\s*:`)
)
