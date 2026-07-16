package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceFrameworksCatalog covers the framework-aware performance rules
// (performance_rules.detect_framework_patterns, on by default within the
// opt-in performance section). Every rule is additionally gated on file-level
// framework evidence — imports or obvious idioms — so code that does not use
// the framework never matches.
var performanceFrameworksCatalog = map[string]core.RuleMetadata{
	"performance.python.django-nplusone-relation": {
		ID:             "performance.python.django-nplusone-relation",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Django relation access in queryset loop",
		Description:    "Warns when a loop over a Django queryset accesses a related object (item.relation.attr or item.relation_set.*) in a file with Django evidence and no select_related/prefetch_related anywhere, issuing one extra query per row (performance_rules.detect_framework_patterns).",
		HowToFix:       "Load the relation with the initial query: queryset.select_related(\"relation\") for foreign keys and one-to-ones, queryset.prefetch_related(\"relation_set\") for reverse and many-to-many relations.",
	},
	"performance.python.orm-query-in-loop": {
		ID:             "performance.python.orm-query-in-loop",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "ORM point query in loop",
		Description:    "Warns when a Django .objects.get/.objects.filter call or a SQLAlchemy session.get call runs inside a loop body, issuing one query per iteration. Covers only the ORM call shapes the generic performance.n-plus-one-query pattern misses, so a line never reports under both rules (performance_rules.detect_framework_patterns).",
		HowToFix:       "Batch the lookups before the loop: Model.objects.in_bulk(ids) or .filter(pk__in=ids) in Django, one query with an in_() filter in SQLAlchemy.",
	},
	"performance.typescript.react-expensive-render": {
		ID:             "performance.typescript.react-expensive-render",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "TypeScript expensive work in React render",
		Description:    "Warns when a React component body (file imports react; function named with a capital letter or use* prefix) chains two or more array methods (.sort/.filter/.map) or constructs via new Array/JSON.parse outside a useMemo/useCallback/useEffect wrapper, redoing the work on every render (performance_rules.detect_framework_patterns).",
		HowToFix:       "Wrap the computation in useMemo with the right dependency array, or hoist it out of the component so it runs once.",
	},
	"performance.javascript.react-expensive-render": {
		ID:             "performance.javascript.react-expensive-render",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "JavaScript expensive work in React render",
		Description:    "Warns when a React component body (file imports react; function named with a capital letter or use* prefix) chains two or more array methods (.sort/.filter/.map) or constructs via new Array/JSON.parse outside a useMemo/useCallback/useEffect wrapper, redoing the work on every render (performance_rules.detect_framework_patterns).",
		HowToFix:       "Wrap the computation in useMemo with the right dependency array, or hoist it out of the component so it runs once.",
	},
	"performance.typescript.express-sync-middleware": {
		ID:             "performance.typescript.express-sync-middleware",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "TypeScript CPU-heavy sync call in Express middleware",
		Description:    "Warns when a known CPU-heavy synchronous API (bcrypt.hashSync/compareSync, crypto.pbkdf2Sync/scryptSync, zlib *Sync, child_process.execSync) runs inside an app.use/router.use middleware region in a file with express evidence, blocking the event loop on every request. Takes precedence over performance.typescript.sync-io-in-handler on the same line (performance_rules.detect_framework_patterns).",
		HowToFix:       "Use the asynchronous variant (bcrypt.hash, crypto.pbkdf2, promisified zlib, exec) or move the CPU-heavy work off the request path (queue or worker thread).",
	},
	"performance.javascript.express-sync-middleware": {
		ID:             "performance.javascript.express-sync-middleware",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "JavaScript CPU-heavy sync call in Express middleware",
		Description:    "Warns when a known CPU-heavy synchronous API (bcrypt.hashSync/compareSync, crypto.pbkdf2Sync/scryptSync, zlib *Sync, child_process.execSync) runs inside an app.use/router.use middleware region in a file with express evidence, blocking the event loop on every request. Takes precedence over performance.javascript.sync-io-in-handler on the same line (performance_rules.detect_framework_patterns).",
		HowToFix:       "Use the asynchronous variant (bcrypt.hash, crypto.pbkdf2, promisified zlib, exec) or move the CPU-heavy work off the request path (queue or worker thread).",
	},
}
