package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var operationsFixTemplates = map[string]core.FixTemplate{
	"operations.missing-owner":   {Kind: guided, Text: "Add ownership metadata for production paths.\n\nExamples:\nCODEOWNERS:\n/service/payments/ @org/payments\n\nor service catalog metadata:\nowner: payments-platform"},
	"operations.missing-runbook": {Kind: guided, Text: "Add a runbook for critical systems.\n\nInclude: service overview, dashboards, alerts, deploy verification, common failures, escalation, and rollback steps."},
}
