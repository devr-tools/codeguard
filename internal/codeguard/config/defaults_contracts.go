package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func applyContractDefaults(dst *core.ContractRulesConfig, def core.ContractRulesConfig) {
	if dst.GoExportedBreaking == nil {
		dst.GoExportedBreaking = boolPtr(true)
	}
	if dst.OpenAPIBreaking == nil {
		dst.OpenAPIBreaking = boolPtr(true)
	}
	if dst.ProtoBreaking == nil {
		dst.ProtoBreaking = boolPtr(true)
	}
	if dst.MigrationDestructive == nil {
		dst.MigrationDestructive = boolPtr(true)
	}
	if dst.MigrationPaths == nil && len(def.MigrationPaths) > 0 {
		dst.MigrationPaths = append([]string(nil), def.MigrationPaths...)
	}
}
