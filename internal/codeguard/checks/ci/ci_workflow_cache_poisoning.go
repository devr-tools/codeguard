package ci

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const ruleWorkflowCachePoisoning = "ci.workflow-cache-poisoning"

// cacheToggle is how an action's control input switches its cache.
type cacheToggle int

const (
	cacheOptInBool   cacheToggle = iota // input: true enables the cache
	cacheOptInString                    // any non-empty value (a package manager name) enables it
	cacheOptOut                         // input: true disables it (lookup-only, no-cache)
	cacheAlways                         // the action always restores a cache
)

type cacheControl struct {
	input            string
	toggle           cacheToggle
	enabledByDefault bool
	fix              string
}

// cacheAwareActions follows zizmor's cache-poisoning table, narrowed to the
// actions whose default is documented. setup-node's package-manager-cache
// auto-detection is deliberately not modelled: it depends on package.json,
// which a per-file scan cannot see, so only an explicit cache: counts.
var cacheAwareActions = map[string]cacheControl{
	"actions/cache":                             {input: "lookup-only", toggle: cacheOptOut, enabledByDefault: true, fix: "drop the cache step from the release job"},
	"actions/cache/restore":                     {toggle: cacheAlways, fix: "drop the cache restore step from the release job"},
	"actions/setup-go":                          {input: "cache", toggle: cacheOptInBool, enabledByDefault: true, fix: "set cache: false"},
	"actions/setup-node":                        {input: "cache", toggle: cacheOptInString, fix: "remove the cache: input"},
	"actions/setup-python":                      {input: "cache", toggle: cacheOptInString, fix: "remove the cache: input"},
	"actions/setup-java":                        {input: "cache", toggle: cacheOptInString, fix: "remove the cache: input"},
	"actions/setup-dotnet":                      {input: "cache", toggle: cacheOptInBool, fix: "set cache: false"},
	"astral-sh/setup-uv":                        {input: "enable-cache", toggle: cacheOptInBool, fix: "set enable-cache: false"},
	"ruby/setup-ruby":                           {input: "bundler-cache", toggle: cacheOptInBool, fix: "set bundler-cache: false"},
	"swatinem/rust-cache":                       {input: "lookup-only", toggle: cacheOptOut, enabledByDefault: true, fix: "drop the rust-cache step from the release job"},
	"oven-sh/setup-bun":                         {input: "no-cache", toggle: cacheOptOut, enabledByDefault: true, fix: "set no-cache: true"},
	"gradle/actions/setup-gradle":               {input: "cache-disabled", toggle: cacheOptOut, enabledByDefault: true, fix: "set cache-disabled: true"},
	"actions-rust-lang/setup-rust-toolchain":    {input: "cache", toggle: cacheOptInBool, enabledByDefault: true, fix: "set cache: false"},
	"jdx/mise-action":                           {input: "cache", toggle: cacheOptInBool, enabledByDefault: true, fix: "set cache: false"},
	"mlugg/setup-zig":                           {input: "use-cache", toggle: cacheOptInBool, enabledByDefault: true, fix: "set use-cache: false"},
	"graalvm/setup-graalvm":                     {input: "cache", toggle: cacheOptInString, fix: "remove the cache: input"},
	"pyo3/maturin-action":                       {input: "sccache", toggle: cacheOptInBool, fix: "set sccache: false"},
	"determinatesystems/magic-nix-cache-action": {input: "use-gha-cache", toggle: cacheOptInBool, enabledByDefault: true, fix: "set use-gha-cache: false"},
}

// releasePublisherActions are zizmor's known publishing actions: a job that
// runs one ships what it builds, whatever triggered the workflow.
var releasePublisherActions = map[string]bool{
	"pypa/gh-action-pypi-publish":                   true,
	"rubygems/release-gem":                          true,
	"jreleaser/release-action":                      true,
	"goreleaser/goreleaser-action":                  true,
	"softprops/action-gh-release":                   true,
	"release-drafter/release-drafter":               true,
	"googleapis/release-please-action":              true,
	"docker/build-push-action":                      true, // only with a literal push: true
	"redhat-actions/push-to-registry":               true,
	"aws-actions/amazon-ecs-deploy-task-definition": true,
	"aws-actions/aws-cloudformation-github-deploy":  true,
	"azure/aci-deploy":                              true,
	"azure/container-apps-deploy-action":            true,
	"azure/functions-action":                        true,
	"azure/sql-action":                              true,
	"cloudflare/wrangler-action":                    true,
	"google-github-actions/deploy-appengine":        true,
	"google-github-actions/deploy-cloudrun":         true,
	"google-github-actions/deploy-cloud-functions":  true,
}

// releaseScenario describes why a job counts as a release job: the
// workflow's trigger, or a publishing step in the job itself.
func releaseScenario(doc *workflowDoc, job workflowJob) string {
	if trigger := releaseTrigger(doc); trigger != "" {
		return trigger
	}
	for _, step := range job.steps {
		if isPublisherStep(step) {
			return "publishes with " + step.actionName()
		}
	}
	return ""
}

func isPublisherStep(step workflowStep) bool {
	name := step.actionName()
	if !releasePublisherActions[name] {
		return false
	}
	if name == "docker/build-push-action" {
		value, state := stepInput(step, "push")
		return state == inputLiteral && isTruthyLiteral(value)
	}
	return true
}

// releaseTrigger narrows zizmor's release triggers to workflows that run
// for nothing but releases: a release event or a tags-only push, optionally
// alongside workflow_dispatch. zizmor also counts pushes that list tags next
// to branches, and pushes to release-named branches, but on real repositories
// those are mostly CI test workflows whose output is never shipped; a job in
// them is still covered when it runs a publishing action.
func releaseTrigger(doc *workflowDoc) string {
	_, on := mappingEntry(doc.root, "on")
	_, push := mappingEntry(on, "push")
	_, tags := mappingEntry(push, "tags")
	_, branches := mappingEntry(push, "branches")
	_, branchesIgnore := mappingEntry(push, "branches-ignore")
	tagPush := tags != nil && branches == nil && branchesIgnore == nil
	for name := range doc.triggers {
		switch {
		case name == "release", name == "workflow_dispatch":
		case name == "push" && tagPush:
		default:
			return ""
		}
	}
	switch {
	case doc.triggers["release"]:
		return "runs only on release events"
	case tagPush:
		return "runs only on tag pushes"
	}
	return ""
}

func (control cacheControl) restores(step workflowStep) bool {
	if control.toggle == cacheAlways {
		return true
	}
	value, state := stepInput(step, control.input)
	switch state {
	case inputDynamic:
		return false
	case inputAbsent:
		return control.enabledByDefault
	}
	switch control.toggle {
	case cacheOptOut:
		return !isTruthyLiteral(value)
	case cacheOptInString:
		return value != "" && !strings.EqualFold(value, "false")
	default:
		return isTruthyLiteral(value)
	}
}

func cachePoisoningFindings(env support.Context, doc *workflowDoc) []core.Finding {
	if doc.composite {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		scenario := releaseScenario(doc, job)
		if scenario == "" {
			continue
		}
		for _, step := range job.steps {
			control, ok := cacheAwareActions[step.actionName()]
			if !ok || !control.restores(step) {
				continue
			}
			line, column := usesLocation(step)
			finding := newWorkflowFinding(env, doc, ruleWorkflowCachePoisoning, "warn", line, column,
				fmt.Sprintf("job %q %s but restores a build cache through %s; any default-branch run that writes that cache (including pull_request_target, workflow_run or issue_comment runs handling untrusted input) can plant files in the release build. Build releases without a cache: %s", job.id, scenario, step.actionName(), control.fix))
			finding.Confidence = core.ConfidenceMedium
			findings = append(findings, finding)
		}
	}
	return findings
}
