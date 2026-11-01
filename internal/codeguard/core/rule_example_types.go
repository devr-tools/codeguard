package core

// RuleExample is a minimal pair of snippets for one rule: Bad triggers the
// rule and Good is the corrected shape that does not.
type RuleExample struct {
	// Language names the snippet's syntax (go, python, typescript, yaml,
	// json, markdown, ...), for rendering and for picking a comment style.
	Language string `json:"language"`
	// Path is where the snippet lives relative to the repository root. Rules
	// that only inspect particular files (workflows, agent settings, MCP
	// configs) need it; for source rules it is a representative file name.
	Path string `json:"path"`
	Bad  string `json:"bad"`
	Good string `json:"good"`
	// Overrides are `key=value` pairs in the `scan -set` syntax that the bad
	// snippet needs to trigger, such as a threshold lowered so the example
	// stays short. Empty means the defaults trigger it.
	Overrides []string `json:"overrides,omitempty"`
}

// RuleConfigKeyRole says how a config key relates to a rule.
type RuleConfigKeyRole string

const (
	// RuleConfigKeyRoleEnable marks a boolean key that must be true for the
	// rule to run; setting it to false disables the rule.
	RuleConfigKeyRoleEnable RuleConfigKeyRole = "enable"
	// RuleConfigKeyRoleTune marks a key that adjusts what the rule reports
	// (a threshold, pattern list, or mode) without being an on/off switch.
	RuleConfigKeyRoleTune RuleConfigKeyRole = "tune"
)

// RuleConfigKey is one config path that controls a rule. Key is the full
// dotted path from the config root, for example
// checks.quality_rules.max_parameters.
type RuleConfigKey struct {
	Key  string            `json:"key"`
	Role RuleConfigKeyRole `json:"role"`
}
