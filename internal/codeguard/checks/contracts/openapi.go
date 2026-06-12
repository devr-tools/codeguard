package contracts

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var openAPIMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

func openAPIBreakingFindings(env support.Context, target core.TargetConfig, changed []core.ChangedFile) []core.Finding {
	if !enabled(env.Config.Checks.ContractRules.OpenAPIBreaking) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range changed {
		if !isOpenAPIFile(file.Path) || file.Status == core.ChangedFileAdded {
			continue
		}
		base := parseOpenAPIDoc(readBase(env, target, file.Path))
		if base == nil {
			continue
		}
		head := map[string]any{}
		if file.Status != core.ChangedFileDeleted {
			if head = parseOpenAPIDoc(readHead(target, file.Path)); head == nil {
				continue
			}
		}
		findings = append(findings, openAPIDocFindings(env, file.Path, base, head)...)
	}
	return findings
}

func openAPIDocFindings(env support.Context, file string, base, head map[string]any) []core.Finding {
	findings := make([]core.Finding, 0)
	basePaths := mapValue(base, "paths")
	headPaths := mapValue(head, "paths")
	for _, path := range sortedKeys(basePaths) {
		headItem, ok := headPaths[path]
		if !ok {
			findings = append(findings, newOpenAPIFinding(env, file, fmt.Sprintf("path %s was removed", path)))
			continue
		}
		findings = append(findings, openAPIPathFindings(env, file, path, asMap(basePaths[path]), asMap(headItem))...)
	}
	return findings
}

func openAPIPathFindings(env support.Context, file string, path string, baseItem, headItem map[string]any) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, method := range openAPIMethods {
		baseOp := asMap(baseItem[method])
		if baseOp == nil {
			continue
		}
		operation := strings.ToUpper(method) + " " + path
		headOp := asMap(headItem[method])
		if headOp == nil {
			findings = append(findings, newOpenAPIFinding(env, file, fmt.Sprintf("operation %s was removed", operation)))
			continue
		}
		findings = append(findings, openAPIOperationFindings(env, file, operation, baseOp, headOp)...)
	}
	return findings
}

func openAPIOperationFindings(env support.Context, file string, operation string, baseOp, headOp map[string]any) []core.Finding {
	findings := make([]core.Finding, 0)
	headResponses := mapValue(headOp, "responses")
	for _, code := range sortedKeys(mapValue(baseOp, "responses")) {
		if _, ok := headResponses[code]; !ok {
			findings = append(findings, newOpenAPIFinding(env, file,
				fmt.Sprintf("response code %s was removed from %s", code, operation)))
		}
	}
	findings = append(findings, openAPIRequiredParamFindings(env, file, operation, baseOp, headOp)...)
	findings = append(findings, openAPIRequiredBodyFindings(env, file, operation, baseOp, headOp)...)
	return findings
}

func openAPIRequiredParamFindings(env support.Context, file string, operation string, baseOp, headOp map[string]any) []core.Finding {
	baseRequired := requiredParamSet(baseOp)
	findings := make([]core.Finding, 0)
	for _, raw := range listValue(headOp, "parameters") {
		param := asMap(raw)
		if param == nil || !asBool(param["required"]) || baseRequired[paramKey(param)] {
			continue
		}
		findings = append(findings, newOpenAPIFinding(env, file,
			fmt.Sprintf("parameter %v (%v) is newly required on %s", param["name"], param["in"], operation)))
	}
	return findings
}

func openAPIRequiredBodyFindings(env support.Context, file string, operation string, baseOp, headOp map[string]any) []core.Finding {
	baseFields := requiredBodyFields(baseOp)
	headFields := requiredBodyFields(headOp)
	findings := make([]core.Finding, 0)
	for _, contentType := range sortedKeys(headFields) {
		for _, field := range sortedKeys(headFields[contentType]) {
			if baseFields[contentType][field] {
				continue
			}
			findings = append(findings, newOpenAPIFinding(env, file,
				fmt.Sprintf("request field %q (%s) is newly required on %s", field, contentType, operation)))
		}
	}
	return findings
}

func newOpenAPIFinding(env support.Context, file string, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "contracts.openapi-breaking",
		Level:   "fail",
		Path:    file,
		Message: message,
	})
}
