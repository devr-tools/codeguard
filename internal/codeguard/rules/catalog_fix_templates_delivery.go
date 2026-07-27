package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var deliveryFixTemplates = map[string]core.FixTemplate{
	"delivery.missing-rollback-strategy": {Kind: guided, Text: "Add rollback evidence next to the rollout or migration.\n\nBefore:\n# deploy production\nkubectl apply -f deploy/app.yaml\n\nAfter:\n# deploy production\nkubectl apply -f deploy/app.yaml\n# rollback: kubectl rollout undo deployment/app --namespace production\n# verify: curl -fsS https://service.example.com/health"},
	"delivery.unsafe-migration-order":    {Kind: guided, Text: "Convert the destructive migration into an expand/backfill/contract rollout.\n\nBefore:\nALTER TABLE users DROP COLUMN legacy_email;\n\nAfter:\n-- release 1: add replacement column and dual-write\n-- release 2: backfill and switch readers\n-- release 3: drop legacy_email after rollback window and backup verification"},
	"delivery.high-risk-change-without-kill-switch": {
		Kind: guided,
		Text: "Gate the high-risk behavior behind an operational flag or kill switch.\n\nBefore:\nchargeCustomer(order)\n\nAfter:\nif flags.Enabled(ctx, \"new_checkout_charge\") {\n\tchargeCustomer(order)\n} else {\n\tchargeCustomerLegacy(order)\n}\n// document how to disable the flag during rollout",
	},
	"delivery.missing-post-deploy-verification": {Kind: guided, Text: "Add a post-deploy verification step to the rollout workflow.\n\nBefore:\n- run: kubectl apply -f deploy/app.yaml\n\nAfter:\n- run: kubectl apply -f deploy/app.yaml\n- run: curl -fsS https://service.example.com/health\n# or call the service smoke/synthetic test used by your operations team"},
}
