package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	durationNamePattern    = regexp.MustCompile(`(?i)(timeout|duration|delay|interval|ttl|latency|elapsed|expiry|expiration|retention)`)
	sizeNamePattern        = regexp.MustCompile(`(?i)(size|limit|length|capacity|bytes?|mb|kb|gb)`)
	moneyNamePattern       = regexp.MustCompile(`(?i)(amount|price|cost|fee|total|subtotal|balance|money)`)
	unitSuffixPattern      = regexp.MustCompile(`(?i)(nanos?|micros?|millis?|ms|seconds?|secs?|s|minutes?|mins?|hours?|hrs?|days?|bytes?|kb|mb|gb|cents?|pennies|minor|bps|basispoints?|usd|eur|gbp|aud|cad)$`)
	collectionTypePattern  = regexp.MustCompile(`(?i)(\[\]|\[\s*\]|array|list|slice|map|dict|record|set|vector|collection|iterable|sequence|promise<[^>]*\[\])`)
	goSliceTypePattern     = regexp.MustCompile(`^\[[^\]]*\]\s*(.+)$`)
	scriptArrayTypePattern = regexp.MustCompile(`^(.+?)\s*\[\s*\]$`)
	// Ordered generic containers only; keyed containers are filtered first.
	genericCollectionTypePattern = regexp.MustCompile(`(?i)^(?:[\w.:]*\b)?(array|list|slice|vec|vector|set|collection|iterable|sequence|readonlyarray|deque|queue|stack|span|arraylist|immutablelist)\s*[<\[]\s*([^<>\[\]]+?)\s*[>\]]$`)
	keyedContainerTypePattern    = regexp.MustCompile(`(?i)(^|\b)(map\s*\[|record\s*<|dict\s*[<\[]|dictionary\s*<|hashmap\s*<|treemap\s*<|btreemap\s*<|unordered_map\s*<|multimap\s*<|counter\s*[<\[]|defaultdict\s*[<\[]|mapping\s*[<\[])`)
	unitPluralExclusionPattern   = regexp.MustCompile(`(?i)(nanos?|micros?|millis?|ms|seconds?|secs?|minutes?|mins?|hours?|hrs?|days?|bytes?|kb|mb|gb|cents?|pennies|minor|bps|basispoints?|usd|eur|gbp|aud|cad)$`)
	scalarTypePattern            = regexp.MustCompile(`(?i)\b(bool|boolean|char|double|float|float64|int|int32|int64|number|string|str|uint|uint64)\b`)
	numericScalarTypePattern     = regexp.MustCompile(`(?i)\b(double|float|float32|float64|int|int8|int16|int32|int64|number|uint|uint8|uint16|uint32|uint64|uintptr|bigint|decimal|long|short|size_t|ssize_t|byte|rune|duration)\b`)
)

// functionIdentifier is one name a function introduces: the function itself,
// a parameter, or a local assignment.
type functionIdentifier struct {
	name string
	typ  string
	expr string
	line int
	role string
}

// functionIdentifiers lists the function name, then its parameters, then its
// local assignments.
func functionIdentifiers(fn precisionFunction) []functionIdentifier {
	identifiers := make([]functionIdentifier, 0, 1+len(fn.Params)+len(fn.Assignments))
	identifiers = append(identifiers, functionIdentifier{name: fn.Name, line: fn.StartLine, role: "function"})
	for _, param := range fn.Params {
		identifiers = append(identifiers, functionIdentifier{name: param.Name, typ: param.Type, line: fn.StartLine, role: "parameter"})
	}
	for _, assignment := range fn.Assignments {
		identifiers = append(identifiers, functionIdentifier{name: assignment.Name, expr: assignment.Expr, line: assignment.Line, role: "local"})
	}
	return identifiers
}

func precisionNamingFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	findings := make([]core.Finding, 0, 6)
	for _, item := range functionIdentifiers(fn) {
		if skipsIdentifierNaming(file, fn, item) {
			continue
		}
		findings = append(findings, identifierNamingFindings(env, file, fn, item)...)
	}
	return findings
}

// skipsIdentifierNaming reports identifiers exempt from every naming rule:
// unnamed ones, React component/hook names, and booleans in scripts or UI
// helpers.
func skipsIdentifierNaming(file string, fn precisionFunction, item functionIdentifier) bool {
	if item.name == "" || (item.name == fn.Name && isReactComponentOrHookBoundary(file, fn)) {
		return true
	}
	return isBooleanNameCandidate(item.typ) && (isSeedOrScriptSourcePath(file) || isUIHelperOrMappingContext(file, fn))
}

func identifierNamingFindings(env support.Context, file string, fn precisionFunction, item functionIdentifier) []core.Finding {
	findings := make([]core.Finding, 0, 2)
	if booleanNameNotPredicate(file, fn, item) {
		findings = append(findings, precisionWarnFinding(env, namingBooleanNotPredicateRuleID, file, item.line,
			fmt.Sprintf("boolean name %q should read as a predicate such as is/has/can/should", item.name), core.ConfidenceMedium))
	}
	if cardinalityMismatch(file, fn, item.name, item.typ) {
		findings = append(findings, precisionWarnFinding(env, namingCardinalityMismatchRuleID, file, item.line,
			fmt.Sprintf("identifier %q has plural/singular wording that conflicts with its value shape", item.name), core.ConfidenceMedium))
	}
	if implementationLeakName(item.name) {
		findings = append(findings, precisionWarnFinding(env, namingImplementationLeakRuleID, file, item.line,
			fmt.Sprintf("identifier %q exposes infrastructure vocabulary in domain-facing naming", item.name), core.ConfidenceMedium))
	}
	if !anyPrecisionExemption(file, fn, uiOrScriptIdentifierExemptions) && missingUnit(item.name, item.typ, item.expr) {
		findings = append(findings, precisionWarnFinding(env, namingMissingUnitRuleID, file, item.line,
			fmt.Sprintf("numeric identifier %q names a duration, size, or money value without a unit suffix", item.name), core.ConfidenceMedium))
	}
	if abbr := unknownAbbreviation(env, item.name); abbr != "" {
		findings = append(findings, precisionWarnFinding(env, namingUnknownAbbreviationRuleID, file, item.line,
			fmt.Sprintf("identifier %q contains abbreviation %q that is not established in quality_rules.naming.allowed_abbreviations", item.name, abbr), core.ConfidenceLow))
	}
	return findings
}

// booleanNameNotPredicate reports a boolean identifier that does not read as
// a predicate and is not covered by a UI or imperative-parameter convention.
func booleanNameNotPredicate(file string, fn precisionFunction, item functionIdentifier) bool {
	if !isBooleanNameCandidate(item.typ) || isPredicateName(item.name) {
		return false
	}
	if item.role == "parameter" && isImperativeBooleanParameterName(item.name) {
		return false
	}
	return !isInferredUIBooleanAssignment(file, fn, item.typ, item.expr, item.line) &&
		!isAllowedBooleanUIName(file, fn, item.name)
}

func isBooleanNameCandidate(typ string) bool {
	return isBooleanType(typ)
}

func isImperativeBooleanParameterName(name string) bool {
	words := identifierWords(name)
	if len(words) < 2 {
		return false
	}
	for _, prefix := range []string{"include", "require", "allow", "skip", "enable", "disable", "force", "use"} {
		if words[0] == prefix {
			return true
		}
	}
	return false
}

func identifierWords(name string) []string {
	name = strings.Trim(name, "_$")
	var words []string
	start := 0
	for idx, r := range name {
		if idx > 0 && (r == '_' || (r >= 'A' && r <= 'Z')) {
			if word := strings.ToLower(strings.Trim(name[start:idx], "_")); word != "" {
				words = append(words, word)
			}
			start = idx
			if r == '_' {
				start++
			}
		}
	}
	if word := strings.ToLower(strings.Trim(name[start:], "_")); word != "" {
		words = append(words, word)
	}
	return words
}

func isInferredUIBooleanAssignment(file string, fn precisionFunction, typ string, expr string, line int) bool {
	if expr == "" || isBooleanType(typ) {
		return false
	}
	return isUIHelperOrMappingContext(file, fn) || isReactComponentOrHookBoundary(file, fn) || nearbyUICallbackStatement(fn, line)
}

func isBooleanType(typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if strings.Contains(typ, "{") || strings.Contains(typ, "}") {
		return false
	}
	return typ == "bool" || typ == "boolean" || strings.Contains(typ, " bool") || strings.Contains(typ, ": boolean")
}

func isPredicateName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, prefix := range []string{
		"allow", "allows", "are", "can", "changed", "compatible", "complete",
		"contains", "could", "deep", "disable", "disabled", "enable", "enabled",
		"exists", "forbidden", "has", "have", "included", "is", "looks",
		"matches", "must", "needs", "pass", "passes", "readable", "ready",
		"requires", "same", "should", "show", "supported", "supports", "valid",
		"verify", "visible",
	} {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	for _, suffix := range []string{
		"allowed", "changed", "compatible", "complete", "differs", "enabled",
		"equal", "equals", "exists", "forbidden", "included", "matches",
		"present", "readable", "supported", "valid",
	} {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

// cardinalityMismatch reports a name whose grammatical number contradicts the
// shape of its value. It recognises exactly two situations, both of which are
// wrong independently of any repository's vocabulary:
//
//   - a plural name on a scalar value ("names string"); and
//   - a singular name that is the element type of the collection it holds
//     ("user []User").
//
// It deliberately does NOT report every singular name on a collection type.
// Measured over four repositories, that formulation produced 586 findings and
// no true positives: byte strings ("payload []byte"), argv vectors ("command
// []string"), maps keyed by id ("index map[string]Ref"), comparison pairs
// ("left, right []Source"), Latin plurals ("criteria []string"), and mass
// nouns ("evidence map[string]any") are all correctly singular.
func cardinalityMismatch(file string, fn precisionFunction, name string, typ string) bool {
	base := strings.ToLower(strings.Trim(name, "_$"))
	if base == "" || conventionalCardinalityName(base) || configuredPluralDomainAbbreviation(base) || strings.HasSuffix(base, "status") || strings.HasSuffix(base, "class") {
		return false
	}
	if isUIHelperOrMappingContext(file, fn) {
		return false
	}
	if strings.Contains(typ, "{") || strings.Contains(typ, "}") {
		return false
	}
	if isPluralName(base) {
		return pluralNameOnScalar(typ)
	}
	return singularNameOfElementType(base, typ)
}

// pluralNameOnScalar reports a plural name bound to a single non-numeric,
// non-boolean scalar, e.g. "recipients string".
//
// Numeric scalars are excluded on purpose: a plural noun on a number names a
// count of things, which is correct English and pervasive ("maxLines int",
// "minFailures int", "samples int", "rawTokens int"). Booleans are excluded
// because a plural noun on a flag names a mode ("lineNumbers bool",
// "applyPatches bool", "allTargets bool"); whether such a flag should read as
// a predicate is naming.boolean-not-predicate's question, not this rule's.
// Enabling this direction without those two carve-outs added 481 counts and
// 13 flags across the measured repositories and no true positives. Variadic
// parameters are collections rather than scalars, so "needles ...string" is
// excluded too.
func pluralNameOnScalar(typ string) bool {
	trimmed := strings.TrimSpace(typ)
	if trimmed == "" || variadicParamType(trimmed) {
		return false
	}
	if collectionTypePattern.MatchString(trimmed) || strings.Contains(trimmed, "[") || strings.Contains(trimmed, "]") {
		return false
	}
	if numericScalarTypePattern.MatchString(trimmed) || isBooleanNameCandidate(trimmed) {
		return false
	}
	return scalarTypePattern.MatchString(trimmed)
}

// variadicParamType recognises variadic parameter spellings, which denote a
// collection without any bracket syntax: Go "...string", C# "params string",
// C++ "Args&&...".
func variadicParamType(typ string) bool {
	return strings.HasPrefix(typ, "...") || strings.HasSuffix(typ, "...") ||
		strings.HasPrefix(strings.ToLower(typ), "params ")
}

// singularNameOfElementType reports whether base is the (singular) name of the
// element type of the collection typ, e.g. "user" for "[]User" or "scenario"
// for "Array<Scenario>". That coincidence is the one high-precision signal
// that a collection has been named as though it held a single value.
//
// Keyed containers are excluded: naming a map after what it maps to
// ("index map[string]Ref", "cache map[string]*Entry") is idiomatic. So are
// primitive element types: "[]byte" and "[]rune" are byte and character
// strings rather than collections of bytes, and "[]string" is routinely a
// single composite value such as an argv vector.
func singularNameOfElementType(base string, typ string) bool {
	element, ok := collectionElementType(typ)
	if !ok {
		return false
	}
	if primitiveTypeName(element) {
		return false
	}
	return base == element || base+"s" == element || base+"es" == element
}

// collectionElementType extracts the element type of an ordered collection,
// lower-cased and stripped of package, namespace, and pointer decoration. It
// returns ok=false for keyed containers and for anything it cannot resolve.
func collectionElementType(typ string) (string, bool) {
	trimmed := strings.TrimSpace(strings.Trim(typ, "*&"))
	if trimmed == "" || keyedContainerTypePattern.MatchString(trimmed) {
		return "", false
	}
	var element string
	switch {
	case goSliceTypePattern.MatchString(trimmed):
		element = goSliceTypePattern.FindStringSubmatch(trimmed)[1]
	case scriptArrayTypePattern.MatchString(trimmed):
		element = scriptArrayTypePattern.FindStringSubmatch(trimmed)[1]
	case genericCollectionTypePattern.MatchString(trimmed):
		element = genericCollectionTypePattern.FindStringSubmatch(trimmed)[2]
	default:
		return "", false
	}
	element = strings.TrimSpace(strings.Trim(element, "*&"))
	if element == "" || strings.ContainsAny(element, "[]<>(),|") {
		return "", false
	}
	if idx := strings.LastIndexAny(element, ".:"); idx >= 0 {
		element = element[idx+1:]
	}
	element = strings.ToLower(strings.TrimSpace(element))
	if element == "" {
		return "", false
	}
	return element, true
}

func primitiveTypeName(element string) bool {
	switch element {
	case "byte", "rune", "bool", "boolean", "char", "string", "str", "int", "int8", "int16",
		"int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float", "float32", "float64", "double", "number", "bigint", "any", "unknown",
		"object", "error", "void":
		return true
	default:
		return false
	}
}

// isPluralName reports whether a name reads as grammatically plural.
//
// NOTE: this used to delegate the unit check to unitSuffixPattern, which lists
// a bare "s" (for "seconds") among its alternatives. Every name ending in "s"
// therefore looked like a unit-suffixed name, and every name not ending in "s"
// failed the plural test outright, so the function returned false for every
// input and the plural-name-on-scalar half of naming.cardinality-mismatch had
// never fired. It now uses a unit pattern without the bare second alias, and
// strips a trailing ordinal digit so that "outputs1" reads as plural.
func isPluralName(name string) bool {
	trimmed := strings.TrimRight(name, "0123456789")
	if trimmed == "" {
		trimmed = name
	}
	if unitPluralExclusionPattern.MatchString(trimmed) || singularSNameExclusion(trimmed) {
		return false
	}
	return strings.HasSuffix(trimmed, "s")
}

// singularSNameExclusion covers names that end in "s" while being singular.
// The suffix rules carry most of it (-ss, -us, -is, -as, -os: address, status,
// analysis, alias, macos); the short set holds the residue that no suffix rule
// separates from a real plural.
func singularSNameExclusion(name string) bool {
	for _, suffix := range []string{"ss", "us", "is", "as", "os"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	switch name {
	case "series", "species", "news", "lens", "dns", "tls", "cors", "https", "aws", "nats", "cms", "jws", "jwks":
		return true
	default:
		return false
	}
}

func implementationLeakName(name string) bool {
	if adapterOrUtilityImplementationName(name) {
		return false
	}
	words := splitIdentifierWords(name)
	if len(words) <= 1 {
		return false
	}
	infraWords := map[string]struct{}{
		"sql": {}, "http": {}, "redis": {}, "kafka": {}, "grpc": {}, "graphql": {},
		"mongo": {}, "s3": {}, "dynamo": {}, "postgres": {}, "mysql": {}, "elastic": {}, "orm": {},
	}
	for _, word := range words {
		if _, ok := infraWords[strings.ToLower(strings.Trim(word, "_$"))]; ok {
			return true
		}
	}
	return false
}

func adapterOrUtilityImplementationName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	return strings.HasPrefix(lowered, "map") ||
		strings.HasPrefix(lowered, "to") ||
		strings.HasPrefix(lowered, "from") ||
		strings.HasPrefix(lowered, "isvalid") ||
		strings.HasPrefix(lowered, "parse")
}

func missingUnit(name string, typ string, expr string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	if unitSuffixPattern.MatchString(lowered) || strings.HasSuffix(lowered, "count") || strings.HasSuffix(lowered, "total") ||
		strings.HasPrefix(lowered, "total") ||
		lowered == "limit" || lowered == "defaultlimit" || lowered == "pagesize" || lowered == "page_size" ||
		lowered == "size" || strings.Contains(lowered, "key") {
		return false
	}
	looksMeasured := durationNamePattern.MatchString(lowered) || sizeNamePattern.MatchString(lowered) || moneyNamePattern.MatchString(lowered)
	if !looksMeasured {
		return false
	}
	return scalarTypePattern.MatchString(typ) || numericExpr(expr)
}

func numericExpr(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	return expr[0] >= '0' && expr[0] <= '9'
}

func unknownAbbreviation(env support.Context, name string) string {
	for _, word := range splitIdentifierWords(name) {
		candidate := strings.ToLower(strings.Trim(word, "_$"))
		if candidate == "" || len(candidate) < 2 {
			continue
		}
		// PERF: allowedAbbreviations rebuilds a ~45-entry map from config on
		// every call, and this runs for every identifier in every function.
		// The two tests commute — a candidate is reported only when it is
		// suspicious AND not allowed — so apply the cheap one first and touch
		// the map only for the handful of candidates that could be reported.
		if !isSuspiciousShortening(candidate) {
			continue
		}
		if _, ok := allowedAbbreviations(env)[candidate]; ok {
			continue
		}
		return word
	}
	return ""
}

func allowedAbbreviations(env support.Context) map[string]struct{} {
	defaults := []string{"api", "ast", "aws", "ce", "ci", "cli", "cpu", "cpp", "css", "cfg", "db", "dns", "ds", "dto", "env", "grpc", "html", "http", "https", "id", "io", "ip", "js", "json", "mcp", "num", "os", "pr", "rpc", "sdk", "sql", "ssh", "tcp", "tls", "ts", "tsx", "ui", "uri", "url", "uuid", "xml", "yaml", "yml"}
	allowed := make(map[string]struct{}, len(defaults)+len(env.Config.Checks.QualityRules.Naming.AllowedAbbreviations))
	for _, value := range defaults {
		allowed[value] = struct{}{}
	}
	for _, value := range env.Config.Checks.QualityRules.Naming.AllowedAbbreviations {
		allowed[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	return allowed
}

func isSuspiciousShortening(word string) bool {
	switch word {
	case "acct", "addr", "cust", "qty", "usr":
		return true
	default:
		return false
	}
}
