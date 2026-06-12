package contracts

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func protoBreakingFindings(env support.Context, target core.TargetConfig, changed []core.ChangedFile) []core.Finding {
	if !enabled(env.Config.Checks.ContractRules.ProtoBreaking) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range changed {
		if !strings.HasSuffix(file.Path, ".proto") || file.Status == core.ChangedFileAdded {
			continue
		}
		baseData := readBase(env, target, file.Path)
		if len(baseData) == 0 {
			continue
		}
		base := parseProto(baseData)
		head := parseProto(readHead(target, file.Path))
		findings = append(findings, protoMessageFindings(env, file.Path, base, head)...)
		findings = append(findings, protoServiceFindings(env, file.Path, base, head)...)
	}
	return findings
}

func protoMessageFindings(env support.Context, file string, base, head protoDefs) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, message := range sortedKeys(base.messages) {
		headFields, ok := head.messages[message]
		if !ok {
			findings = append(findings, newProtoFinding(env, file, fmt.Sprintf("message %s was removed", message)))
			continue
		}
		findings = append(findings, protoFieldFindings(env, file, message, base.messages[message], headFields)...)
	}
	return findings
}

func protoFieldFindings(env support.Context, file string, message string, baseFields, headFields map[string]protoField) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, name := range sortedKeys(baseFields) {
		baseField := baseFields[name]
		headField, ok := headFields[name]
		if !ok {
			findings = append(findings, newProtoFinding(env, file,
				fmt.Sprintf("field %s.%s was removed or renamed", message, name)))
			continue
		}
		if headField.number != baseField.number {
			findings = append(findings, newProtoFinding(env, file,
				fmt.Sprintf("field %s.%s was renumbered from %s to %s", message, name, baseField.number, headField.number)))
		}
		if headField.typ != baseField.typ {
			findings = append(findings, newProtoFinding(env, file,
				fmt.Sprintf("field %s.%s changed type from %s to %s", message, name, baseField.typ, headField.typ)))
		}
	}
	return findings
}

func protoServiceFindings(env support.Context, file string, base, head protoDefs) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, service := range sortedKeys(base.services) {
		headRPCs, ok := head.services[service]
		if !ok {
			findings = append(findings, newProtoFinding(env, file, fmt.Sprintf("service %s was removed", service)))
			continue
		}
		for _, rpc := range sortedKeys(base.services[service]) {
			if !headRPCs[rpc] {
				findings = append(findings, newProtoFinding(env, file,
					fmt.Sprintf("rpc %s was removed from service %s", rpc, service)))
			}
		}
	}
	return findings
}

func newProtoFinding(env support.Context, file string, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "contracts.proto-breaking",
		Level:   "fail",
		Path:    file,
		Message: message,
	})
}
