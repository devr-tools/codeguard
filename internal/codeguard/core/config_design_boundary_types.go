package core

// DesignLayerConfig assigns source paths to an architectural layer and
// constrains the local and external dependencies that layer may use.
type DesignLayerConfig struct {
	Name           string   `json:"name" yaml:"name"`
	Paths          []string `json:"paths" yaml:"paths"`
	MayDependOn    []string `json:"may_depend_on,omitempty" yaml:"may_depend_on,omitempty"`
	DenyDependOn   []string `json:"deny_depend_on,omitempty" yaml:"deny_depend_on,omitempty"`
	DeniedExternal []string `json:"denied_external,omitempty" yaml:"denied_external,omitempty"`
}

// DesignDomainConfig describes a bounded context. Imports from another domain
// must target a public path, while data paths remain private to their owner.
type DesignDomainConfig struct {
	Name        string   `json:"name" yaml:"name"`
	Paths       []string `json:"paths" yaml:"paths"`
	PublicPaths []string `json:"public_paths,omitempty" yaml:"public_paths,omitempty"`
	DataPaths   []string `json:"data_paths,omitempty" yaml:"data_paths,omitempty"`
	MayDependOn []string `json:"may_depend_on,omitempty" yaml:"may_depend_on,omitempty"`
}

// DesignCapabilityConfig limits imports that grant a technical capability
// (database, network, filesystem, cloud SDK, and similar) to approved paths.
type DesignCapabilityConfig struct {
	Name         string   `json:"name" yaml:"name"`
	Imports      []string `json:"imports" yaml:"imports"`
	AllowedPaths []string `json:"allowed_paths" yaml:"allowed_paths"`
}

// DesignPublicSurfaceConfig prevents consumers outside a package or component
// from deep-importing implementation modules instead of its public entrypoints.
type DesignPublicSurfaceConfig struct {
	Name        string   `json:"name" yaml:"name"`
	Paths       []string `json:"paths" yaml:"paths"`
	Entrypoints []string `json:"entrypoints" yaml:"entrypoints"`
}

// DesignProductionTestConfig keeps test-only helpers and dependencies out of
// production source paths.
type DesignProductionTestConfig struct {
	Enabled         *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ProductionPaths []string `json:"production_paths" yaml:"production_paths"`
	TestPaths       []string `json:"test_paths" yaml:"test_paths"`
}

// DesignReachabilityConfig reports production modules that cannot be reached
// from an approved application or package entrypoint.
type DesignReachabilityConfig struct {
	Enabled     *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Entrypoints []string `json:"entrypoints,omitempty" yaml:"entrypoints,omitempty"`
	IgnorePaths []string `json:"ignore_paths,omitempty" yaml:"ignore_paths,omitempty"`
}

// DesignStabilityConfig warns when a stable, widely depended-on module points
// toward a substantially less stable module.
type DesignStabilityConfig struct {
	Enabled             *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MinimumFanIn        int      `json:"minimum_fan_in,omitempty" yaml:"minimum_fan_in,omitempty"`
	MaxInstabilityDelta float64  `json:"max_instability_delta,omitempty" yaml:"max_instability_delta,omitempty"`
	IgnorePaths         []string `json:"ignore_paths,omitempty" yaml:"ignore_paths,omitempty"`
}
