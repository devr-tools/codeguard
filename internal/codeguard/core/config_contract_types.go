package core

type ContractRulesConfig struct {
	GoExportedBreaking   *bool    `json:"go_exported_breaking,omitempty"`
	OpenAPIBreaking      *bool    `json:"openapi_breaking,omitempty"`
	ProtoBreaking        *bool    `json:"proto_breaking,omitempty"`
	MigrationDestructive *bool    `json:"migration_destructive,omitempty"`
	MigrationPaths       []string `json:"migration_paths,omitempty"`
}
