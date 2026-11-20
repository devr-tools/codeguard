package ci

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	actionRefVersionPattern     = regexp.MustCompile(`(?i)^v?(\d+)(?:\.|$)`)
	actionCommentVersionPattern = regexp.MustCompile(`(?i)#\s*(?:tag[=:]\s*)?v?(\d+)(?:[.\s]|$)`)
)

// inputState is what a step's with: input says about a setting: absent, set
// to a literal value, or computed by an expression that cannot be evaluated
// offline (which the precision-first rules treat as "do not report").
type inputState int

const (
	inputAbsent inputState = iota
	inputLiteral
	inputDynamic
)

// stepInput returns a with: input's literal value and how it is set.
func stepInput(step workflowStep, name string) (string, inputState) {
	_, value := mappingEntry(step.with, name)
	if value == nil || value.Kind != yaml.ScalarNode {
		return "", inputAbsent
	}
	text := strings.TrimSpace(value.Value)
	if strings.Contains(text, "${{") {
		return text, inputDynamic
	}
	return text, inputLiteral
}

// isTruthyLiteral mirrors how the runner and @actions/core read a boolean:
// only "true" (in any case) is true; "yes", "1" and "on" are not.
func isTruthyLiteral(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

// actionMajorVersion reads the major version of a step's action from its
// ref (@v4, @v4.1.2) or, for a commit-SHA pin, from the version comment
// pinning tools leave on the uses: line (@<sha> # v4.1.2).
func actionMajorVersion(doc *workflowDoc, step workflowStep) (int, bool) {
	_, ref, _ := strings.Cut(step.uses, "@")
	if match := actionRefVersionPattern.FindStringSubmatch(strings.TrimSpace(ref)); match != nil {
		return atoiVersion(match[1])
	}
	if step.usesVal == nil || step.usesVal.Line < 1 || step.usesVal.Line > len(doc.lines) {
		return 0, false
	}
	if match := actionCommentVersionPattern.FindStringSubmatch(doc.lines[step.usesVal.Line-1]); match != nil {
		return atoiVersion(match[1])
	}
	return 0, false
}

func atoiVersion(text string) (int, bool) {
	major, err := strconv.Atoi(text)
	return major, err == nil
}

// firstTrigger returns the first of names (in order) that triggers the
// workflow, or "".
func firstTrigger(doc *workflowDoc, names []string) string {
	for _, name := range names {
		if doc.triggers[name] {
			return name
		}
	}
	return ""
}

// usesLocation is the line and column of a step's uses: value, or of the
// step itself when it has none.
func usesLocation(step workflowStep) (int, int) {
	if step.usesVal != nil {
		return step.usesVal.Line, step.usesVal.Column
	}
	return step.node.Line, step.node.Column
}

// actionName is the owner/repo[/path] part of a uses: reference, lowercased.
func (step workflowStep) actionName() string {
	action, _, _ := strings.Cut(strings.ToLower(step.uses), "@")
	return action
}
