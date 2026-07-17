package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var designPolicyFixTemplates = map[string]core.FixTemplate{
	"design.unreachable-module":  {Kind: guided, Text: "Remove the unreachable production module, connect it from an approved entrypoint, or add the missing entrypoint deliberately.\n\nBefore:\n// src/legacy.ts is reachable from no application or package entrypoint\n\nAfter:\n// delete legacy.ts if dead, or import its supported API from the configured entrypoint"},
	"design.stability-direction": {Kind: guided, Text: "Make the dependency point toward the more stable abstraction.\n\nBefore:\n// shared/core imports a volatile feature adapter\n\nAfter:\n// shared/core owns a small contract; the volatile adapter implements it at the composition root"},
}
