package ci

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ruleWorkflowArtipacked is named after zizmor's artipacked audit.
const ruleWorkflowArtipacked = "ci.workflow-credential-persistence"

// checkoutSafeMajor is the first actions/checkout major that keeps the
// persisted token under $RUNNER_TEMP instead of the workspace's .git/config,
// so uploading the workspace no longer uploads the token.
const checkoutSafeMajor = 6

// workspacePrefixes are spellings of the checkout root in an upload path.
var workspacePrefixes = []string{"${{ github.workspace }}", "${{github.workspace}}", "$github_workspace", "${github_workspace}"}

// credentialPersistenceFindings reports zizmor's artipacked leak, narrowed to
// the case where the token actually reaches an artifact: a checkout that
// persists credentials into .git/config, followed in the same job by an
// upload-artifact step whose path covers the checkout and that uploads
// hidden files. A persisted token with no such upload is not reported; it
// stays on a runner that is discarded after the job.
func credentialPersistenceFindings(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		for idx, step := range job.steps {
			if !checkoutPersistsToWorkspace(doc, step) {
				continue
			}
			checkoutPath, _ := stepInput(step, "path")
			upload, ok := leakingUpload(doc, job.steps[idx+1:], checkoutPath)
			if !ok {
				continue
			}
			line, column := usesLocation(step)
			uploadLine, _ := usesLocation(upload)
			findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowArtipacked, "fail", line, column,
				fmt.Sprintf("actions/checkout persists the job's GITHUB_TOKEN in .git/config, and the upload-artifact step on line %d uploads the checkout including hidden files, so anyone who can download the artifact gets the token; set persist-credentials: false on the checkout (or narrow the upload path)", uploadLine)))
		}
	}
	return findings
}

// checkoutPersistsToWorkspace reports an actions/checkout step that leaves
// the token in .git/config: persist-credentials is not literally false, and
// the action is not known to be v6 or later. An expression-valued input is
// treated as unknown and not reported.
func checkoutPersistsToWorkspace(doc *workflowDoc, step workflowStep) bool {
	if !step.usesAction("actions/checkout") {
		return false
	}
	value, state := stepInput(step, "persist-credentials")
	if state == inputDynamic || (state == inputLiteral && !isTruthyLiteral(value)) {
		return false
	}
	major, known := actionMajorVersion(doc, step)
	return !known || major < checkoutSafeMajor
}

// leakingUpload returns the first later upload-artifact step that would
// include .git/config: its path covers the checkout root (or names .git), and
// it uploads hidden files, which upload-artifact v4 and later skip unless
// include-hidden-files: true. A path may use ${{ github.workspace }}; any
// other expression never normalizes to the root, so it is not reported.
func leakingUpload(doc *workflowDoc, steps []workflowStep, checkoutPath string) (workflowStep, bool) {
	for _, step := range steps {
		if !step.usesAction("actions/upload-artifact") {
			continue
		}
		paths, state := stepInput(step, "path")
		if state == inputAbsent || !uploadCoversCheckout(paths, checkoutPath) {
			continue
		}
		if uploadIncludesHiddenFiles(doc, step) {
			return step, true
		}
	}
	return workflowStep{}, false
}

func uploadIncludesHiddenFiles(doc *workflowDoc, step workflowStep) bool {
	if value, state := stepInput(step, "include-hidden-files"); state == inputLiteral && isTruthyLiteral(value) {
		return true
	}
	major, known := actionMajorVersion(doc, step)
	return known && major < 4
}

// uploadCoversCheckout reports whether any (non-exclusion) line of an upload
// path is the checkout root, a glob over it, or a path through .git.
func uploadCoversCheckout(paths, checkoutPath string) bool {
	root := normalizeUploadPath(checkoutPath)
	for _, raw := range strings.Split(paths, "\n") {
		entry := strings.TrimSpace(raw)
		if entry == "" || strings.HasPrefix(entry, "!") {
			continue
		}
		normalized := normalizeUploadPath(entry)
		if normalized == root || hasGitSegment(normalized) {
			return true
		}
	}
	return false
}

// normalizeUploadPath reduces a path to its checkout-relative directory: the
// workspace prefix, ./, trailing globs and slashes are removed, so ".",
// "./**", "*" and "${{ github.workspace }}/" all normalize to "".
func normalizeUploadPath(entry string) string {
	entry = strings.Trim(strings.TrimSpace(entry), `"'`)
	lower := strings.ToLower(entry)
	for _, prefix := range workspacePrefixes {
		if strings.HasPrefix(lower, prefix) {
			entry = entry[len(prefix):]
			break
		}
	}
	for {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(entry, "/"), "./")
		trimmed = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(trimmed, "/"), "*"), "*")
		if trimmed == "." {
			trimmed = ""
		}
		if trimmed == entry {
			return entry
		}
		entry = trimmed
	}
}

func hasGitSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".git" {
			return true
		}
	}
	return false
}
