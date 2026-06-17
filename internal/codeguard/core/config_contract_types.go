package core

type ContractRulesConfig struct {
	GoExportedBreaking   *bool    `json:"go_exported_breaking,omitempty" yaml:"go_exported_breaking,omitempty"`
	OpenAPIBreaking      *bool    `json:"openapi_breaking,omitempty" yaml:"openapi_breaking,omitempty"`
	ProtoBreaking        *bool    `json:"proto_breaking,omitempty" yaml:"proto_breaking,omitempty"`
	MigrationDestructive *bool    `json:"migration_destructive,omitempty" yaml:"migration_destructive,omitempty"`
	MigrationPaths       []string `json:"migration_paths,omitempty" yaml:"migration_paths,omitempty"`
}
