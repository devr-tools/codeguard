package agentcontext

import "sort"

// CommandSurface is the root Makefile and package.json command surface exactly
// as the agent-context rules read it. Generators use it so a doc written from
// the surface satisfies context.undocumented-commands by construction instead
// of by a second, drifting parser.
type CommandSurface struct {
	// MakeTargets lists the root Makefile's targets in first-seen file order.
	MakeTargets []string
	// MakeReliable is false when the Makefile has includes or pattern rules,
	// in which case the context rules never judge make targets.
	MakeReliable bool
	// NpmScripts lists the root package.json scripts, sorted by name.
	NpmScripts []string
	// NpmReliable is false for workspace roots, whose scripts the rules
	// never judge.
	NpmReliable bool
}

// ReadCommandSurface parses the command surface of the repository at root.
func ReadCommandSurface(root string) CommandSurface {
	resolver := newRepoResolver(root)
	scripts := make([]string, 0, len(resolver.npmScripts))
	for name := range resolver.npmScripts {
		scripts = append(scripts, name)
	}
	sort.Strings(scripts)
	return CommandSurface{
		MakeTargets:  append([]string(nil), resolver.makeOrder...),
		MakeReliable: resolver.makeReliable,
		NpmScripts:   scripts,
		NpmReliable:  resolver.npmReliable,
	}
}

// CanonicalCommandNames returns the high-signal command names
// context.undocumented-commands requires the agent docs to mention.
func CanonicalCommandNames() []string {
	return append([]string(nil), highSignalCommandNames...)
}

// IsResolvableCommandWord reports whether a make target or npm script name is
// a literal the context rules can resolve (no expansions or placeholders).
func IsResolvableCommandWord(word string) bool {
	return isPlainCommandWord(word)
}
