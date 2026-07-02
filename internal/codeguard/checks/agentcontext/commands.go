package agentcontext

import "strings"

// commandLineReferences extracts references from one line inside a shell
// command fence. Heredocs and directory changes make the rest of the block
// unresolvable, so they flip the fence to unreliable instead of guessing.
func commandLineReferences(line string, fence *fenceState) []docReference {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}
	if strings.Contains(trimmed, "<<") {
		fence.unreliable = true
		return nil
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "$ "), "> ")
	refs, unreliable := commandsFromShellLine(trimmed)
	if unreliable {
		fence.unreliable = true
		return nil
	}
	return refs
}

// commandStringReferences parses a single inline-code command such as
// `make deploy` or `npm run build`. Only spans that start with a recognized
// command are treated as commands; anything else is left to the path rules.
func commandStringReferences(span string) []docReference {
	fields := strings.Fields(span)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "make", "npm", "pnpm", "yarn":
		refs, _ := commandsFromShellLine(span)
		return refs
	default:
		if strings.HasPrefix(fields[0], "./") {
			refs, _ := commandsFromShellLine(span)
			return refs
		}
		return nil
	}
}

// commandsFromShellLine splits a shell line on command separators and
// extracts references from each simple command. unreliable is true when the
// line changes the working directory, after which sibling and following
// commands can no longer be resolved against the repo root.
func commandsFromShellLine(line string) (refs []docReference, unreliable bool) {
	for _, command := range splitShellCommands(line) {
		fields := strings.Fields(command)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "cd" || fields[0] == "pushd" {
			return nil, true
		}
		refs = append(refs, simpleCommandReferences(fields)...)
	}
	return refs, false
}

// splitShellCommands breaks a line at &&, ||, ;, and | so each simple
// command is inspected on its own.
func splitShellCommands(line string) []string {
	for _, sep := range []string{"&&", "||", ";", "|"} {
		line = strings.ReplaceAll(line, sep, "\n")
	}
	return strings.Split(line, "\n")
}

// simpleCommandReferences extracts the resolvable references from one simple
// command's fields: make targets, npm/pnpm/yarn run scripts, and ./-prefixed
// repo-relative paths anywhere in the command.
func simpleCommandReferences(fields []string) []docReference {
	var refs []docReference
	switch fields[0] {
	case "make":
		refs = append(refs, makeTargetReferences(fields[1:])...)
	case "npm", "pnpm", "yarn":
		refs = append(refs, npmRunReference(fields[1:])...)
	}
	for _, field := range fields {
		if !strings.HasPrefix(field, "./") {
			continue
		}
		if value, ok := pathToken(field, false); ok {
			refs = append(refs, docReference{kind: refPath, value: value, display: field})
		}
	}
	return refs
}

// makeTargetReferences reads the targets of a make invocation. Flags that
// change the makefile or directory make resolution impossible, so they abort
// the whole invocation.
func makeTargetReferences(args []string) []docReference {
	var refs []docReference
	for _, arg := range args {
		switch {
		case arg == "-C" || arg == "-f" || strings.HasPrefix(arg, "--directory") || strings.HasPrefix(arg, "--file") || strings.HasPrefix(arg, "--makefile"):
			return nil
		case strings.HasPrefix(arg, "-") || strings.Contains(arg, "="):
			continue
		case isPlainCommandWord(arg):
			refs = append(refs, docReference{kind: refMake, value: arg, display: "make " + arg})
		}
	}
	return refs
}

// npmRunReference reads the script name from an `<npm|pnpm|yarn> run <name>`
// invocation. Shorthand forms (yarn build) are skipped because only the
// explicit run form is unambiguous across the three tools.
func npmRunReference(args []string) []docReference {
	if len(args) < 2 || (args[0] != "run" && args[0] != "run-script") {
		return nil
	}
	script := args[1]
	if !isPlainCommandWord(script) {
		return nil
	}
	return []docReference{{kind: refNpmScript, value: script, display: "run " + script}}
}

// isPlainCommandWord reports whether a make target or npm script name is a
// literal word this rule can resolve: no expansions, globs, or placeholders.
func isPlainCommandWord(word string) bool {
	if word == "" || strings.HasPrefix(word, "-") {
		return false
	}
	for _, r := range word {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == ':' || r == '/':
		default:
			return false
		}
	}
	return true
}
