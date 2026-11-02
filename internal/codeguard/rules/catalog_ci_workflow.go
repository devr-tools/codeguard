package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// ciWorkflowCatalog covers the GitHub Actions attack surface (the classes
// zizmor, actionlint and OpenSSF Scorecard report) for workflow files under
// .github/workflows and composite action.yml files.
var ciWorkflowCatalog = map[string]core.RuleMetadata{
	"ci.workflow-script-injection": {
		ID:               "ci.workflow-script-injection",
		Section:          "CI/CD",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Workflow script injection",
		Description:      "Fails when an attacker-controlled ${{ }} expression (issue, pull request, comment or review text, commit messages, github.head_ref, or a composite action's inputs.*) is interpolated directly into a run: script or an actions/github-script script.",
		HowToFix:         "Pass the value through the step's env: block and reference it as a quoted shell variable (\"$TITLE\") or process.env in github-script, so it is data rather than code.",
	},
	"ci.workflow-dangerous-trigger": {
		ID:               "ci.workflow-dangerous-trigger",
		Section:          "CI/CD",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Privileged trigger checks out pull request code",
		Description:      "Fails when a pull_request_target or workflow_run workflow checks out the pull request's own head (github.event.pull_request.head.*, github.head_ref, workflow_run head refs, refs/pull/*, or gh pr checkout), running untrusted code with the base repository's secrets and write token.",
		HowToFix:         "Build and test pull request code in an unprivileged pull_request workflow; keep pull_request_target/workflow_run jobs on the base ref and pass results between them as artifacts treated as untrusted data.",
	},
	"ci.workflow-write-all-permissions": {
		ID:               "ci.workflow-write-all-permissions",
		Section:          "CI/CD",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Workflow grants write-all permissions",
		Description:      "Fails when a workflow or job sets permissions: write-all, giving GITHUB_TOKEN write access to every scope.",
		HowToFix:         "Replace write-all with an explicit map of the scopes the job needs, for example contents: read plus pull-requests: write.",
	},
	"ci.workflow-missing-permissions": {
		ID:               "ci.workflow-missing-permissions",
		Section:          "CI/CD",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Workflow token permissions not declared",
		Description:      "Warns when a workflow has no top-level permissions block and at least one job sets none, so GITHUB_TOKEN falls back to the repository or organization default (read-write on all scopes for older or permissive repositories). Workflows triggered only by workflow_call are skipped because the caller caps their token.",
		HowToFix:         "Add permissions: contents: read at the top of the workflow and grant additional scopes on the individual jobs that need them.",
	},
	"ci.workflow-remote-script-pipe": {
		ID:               "ci.workflow-remote-script-pipe",
		Section:          "CI/CD",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Remote script piped into a shell",
		Description:      "Warns when a workflow or composite action step pipes curl or wget output straight into sh/bash (including bash <(curl ...) and sh -c \"$(curl ...)\"), executing whatever the remote host serves.",
		HowToFix:         "Download the script or binary to a file, verify a pinned checksum or signature, then execute it; or use a pinned action or package manager instead.",
	},
	"ci.workflow-fork-trigger-secrets": {
		ID:               "ci.workflow-fork-trigger-secrets",
		Section:          "CI/CD",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Secrets in a fork-triggered job",
		Description:      "Warns when a job in a pull_request_target workflow reads secrets.* (other than GITHUB_TOKEN) or passes secrets: inherit, because any fork can trigger the run.",
		HowToFix:         "Move secret-using steps to a workflow that fork pull requests cannot trigger, or protect the job with an environment that requires reviewer approval.",
	},
}
