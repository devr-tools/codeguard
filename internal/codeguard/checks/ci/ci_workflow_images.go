package ci

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

const ruleWorkflowUnpinnedImage = "ci.workflow-unpinned-image"

// imageReference is one container image a workflow runs: a job container,
// a service container, or a docker:// step.
type imageReference struct {
	node  *yaml.Node
	ref   string
	label string
}

func jobImageReferences(job workflowJob) []imageReference {
	refs := make([]imageReference, 0)
	_, container := mappingEntry(job.node, "container")
	if container != nil && container.Kind == yaml.ScalarNode {
		refs = append(refs, imageReference{node: container, ref: container.Value, label: fmt.Sprintf("job %q container", job.id)})
	} else if _, image := mappingEntry(container, "image"); image != nil && image.Kind == yaml.ScalarNode {
		refs = append(refs, imageReference{node: image, ref: image.Value, label: fmt.Sprintf("job %q container", job.id)})
	}
	_, services := mappingEntry(job.node, "services")
	if services != nil && services.Kind == yaml.MappingNode {
		for idx := 0; idx+1 < len(services.Content); idx += 2 {
			if _, image := mappingEntry(services.Content[idx+1], "image"); image != nil && image.Kind == yaml.ScalarNode {
				refs = append(refs, imageReference{node: image, ref: image.Value, label: fmt.Sprintf("job %q service %q", job.id, services.Content[idx].Value)})
			}
		}
	}
	for _, step := range job.steps {
		if image, ok := strings.CutPrefix(strings.TrimSpace(step.uses), "docker://"); ok && step.usesVal != nil {
			refs = append(refs, imageReference{node: step.usesVal, ref: image, label: "docker:// step"})
		}
	}
	return refs
}

// unpinnedImageFindings reports container images not pinned by digest. A
// tag is mutable (the registry owner, or anyone who takes over the account,
// can repoint it), so a tag warns and a missing or latest tag fails, the
// way ci.mutable-deployment-reference fails any action not pinned to a
// commit SHA. Lines that rule already reports for :latest are skipped so the
// same reference is not reported twice; expression-built references cannot
// be resolved offline and are skipped too.
func unpinnedImageFindings(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		for _, image := range jobImageReferences(job) {
			ref := strings.TrimSpace(image.ref)
			if ref == "" || strings.Contains(ref, "${{") || strings.Contains(ref, "@sha256:") {
				continue
			}
			tag := imageTag(ref)
			level, problem := "warn", fmt.Sprintf("uses the mutable tag %q", tag)
			switch {
			case tag == "":
				level, problem = "fail", "has no tag, so it resolves to whatever latest points to"
			case strings.EqualFold(tag, "latest"):
				level, problem = "fail", "uses the mutable latest tag"
			}
			if level == "fail" && mutableReferenceReportsLine(doc, image.node.Line) {
				continue
			}
			findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowUnpinnedImage, level, image.node.Line, image.node.Column,
				fmt.Sprintf("%s image %q %s; pin it by digest (%s@sha256:<digest>, keeping the tag in a comment) so every run executes the image that was reviewed", image.label, ref, problem, imageRepository(ref))))
		}
	}
	return findings
}

// imageTag returns the tag of an image reference: the text after the last
// ':' in its final path segment (a ':' earlier is a registry port).
func imageTag(ref string) string {
	last := ref[strings.LastIndex(ref, "/")+1:]
	if _, tag, ok := strings.Cut(last, ":"); ok {
		return tag
	}
	return ""
}

func imageRepository(ref string) string {
	if tag := imageTag(ref); tag != "" {
		return strings.TrimSuffix(ref, ":"+tag)
	}
	return ref
}

func mutableReferenceReportsLine(doc *workflowDoc, line int) bool {
	if line < 1 || line > len(doc.lines) {
		return false
	}
	text := doc.lines[line-1]
	return latestImagePattern.MatchString(text) || deployImageRunPattern.MatchString(text)
}
