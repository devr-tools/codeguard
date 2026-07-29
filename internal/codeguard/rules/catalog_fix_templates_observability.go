package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var observabilityFixTemplates = map[string]core.FixTemplate{
	"observability.unstructured-log":             {Kind: guided, Text: "Replace raw print/console logging with structured logging.\n\nBefore:\nconsole.log(\"failed\", err)\n\nAfter:\nlogger.error(\"checkout failed\", { operation: \"checkout\", err })"},
	"observability.error-without-context":        {Kind: guided, Text: "Add safe operation/request context to error logs.\n\nBefore:\nlogger.error(err)\n\nAfter:\nlogger.error(\"load order failed\", { operation: \"load_order\", order_id: safeOrderID, err })"},
	"observability.sensitive-log-data":           {Kind: guided, Text: "Remove, redact, or hash sensitive values before logging.\n\nBefore:\nlogger.info(\"login\", { token })\n\nAfter:\nlogger.info(\"login\", { token_present: token != \"\" })"},
	"observability.high-cardinality-label":       {Kind: guided, Text: "Use bounded metric labels.\n\nBefore:\nrequests_total.WithLabelValues(userID, rawPath)\n\nAfter:\nrequests_total.WithLabelValues(routeTemplate, statusClass)"},
	"observability.critical-path-uninstrumented": {Kind: guided, Text: "Instrument critical entrypoints with tracing, metrics, or structured logs.\n\nAdd a span/metric/log at the handler, job, consumer, or payment boundary and record failures with safe context."},
	"observability.log-and-ignore":               {Kind: guided, Text: "Do not log and silently continue unless the failure is explicitly safe.\n\nBefore:\nif err != nil { logger.error(err); return nil }\n\nAfter:\nif err != nil { return fmt.Errorf(\"send receipt: %w\", err) }"},
	"observability.shallow-health-check":         {Kind: guided, Text: "Split liveness from readiness and make readiness verify critical dependencies.\n\nBefore:\n/health returns 200 OK unconditionally\n\nAfter:\n/live returns process liveness; /ready checks database, queue, and required downstream readiness with bounded timeouts."},
}
