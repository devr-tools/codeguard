package quality

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	namingBehaviorMismatchRuleID        = "naming.behavior-mismatch"
	namingBooleanNotPredicateRuleID     = "naming.boolean-not-predicate"
	namingDomainVocabularyDriftRuleID   = "naming.domain-vocabulary-drift"
	namingUnknownAbbreviationRuleID     = "naming.unknown-abbreviation"
	namingCardinalityMismatchRuleID     = "naming.cardinality-mismatch"
	namingImplementationLeakRuleID      = "naming.implementation-leak"
	namingMissingUnitRuleID             = "naming.missing-unit"
	namingRoleSuffixOveruseRuleID       = "naming.role-suffix-overuse"
	namingCrossLayerInconsistencyRuleID = "naming.cross-layer-inconsistency"

	functionHiddenMutationRuleID             = "function.hidden-mutation"
	functionInconsistentReturnContractRuleID = "function.inconsistent-return-contract"
	functionMultipleResponsibilitiesRuleID   = "function.multiple-responsibilities"
	functionOrchestrationDomainMixRuleID     = "function.orchestration-domain-mix"
	functionPartialResultRuleID              = "function.partial-result"
)

var (
	commandFunctionPrefixPattern = regexp.MustCompile(`^(add|append|assign|cancel|clear|close|create|delete|disable|emit|enable|insert|mutate|notify|open|persist|publish|record|remove|reset|save|send|set|store|submit|toggle|update|upsert|upload|write)`)
	readCallPattern              = regexp.MustCompile(`(?i)(^|[.>:\-_])(count|fetch|find|get|list|load|lookup|query|read|select|search)([A-Z_:\-.]|$)`)
	identifierTokenPattern       = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)
	infraNamePattern             = regexp.MustCompile(`(?i)(sql|http|redis|kafka|grpc|graphql|mongo|s3|dynamo|postgres|mysql|elastic|orm)`)
	roleSuffixPattern            = regexp.MustCompile(`(?i)(manager|helper|util|utils|service|processor)$`)
	durationNamePattern          = regexp.MustCompile(`(?i)(timeout|duration|delay|interval|ttl|latency|elapsed|expiry|expiration|retention)`)
	sizeNamePattern              = regexp.MustCompile(`(?i)(size|limit|length|capacity|bytes?|mb|kb|gb)`)
	moneyNamePattern             = regexp.MustCompile(`(?i)(amount|price|cost|fee|total|subtotal|balance|money)`)
	unitSuffixPattern            = regexp.MustCompile(`(?i)(nanos?|micros?|millis?|ms|seconds?|secs?|s|minutes?|mins?|hours?|hrs?|days?|bytes?|kb|mb|gb|cents?|pennies|usd|eur|gbp|aud|cad)$`)
	collectionTypePattern        = regexp.MustCompile(`(?i)(\[\]|\[\s*\]|array|list|slice|map|dict|record|set|vector|collection|iterable|sequence|promise<[^>]*\[\])`)
	scalarTypePattern            = regexp.MustCompile(`(?i)\b(bool|boolean|char|double|float|float64|int|int32|int64|number|string|str|uint|uint64)\b`)
	booleanExprPattern           = regexp.MustCompile(`(?i)^(true|false|nil|none|null|undefined|[A-Za-z_$][\w$]*\s*(===|!==|==|!=|<=|>=|<|>)|.*(\band\b|\bor\b|&&|\|\||\binstanceof\b|\bis\s+not\b|\bis\b).*)$`)
	paramMutationPattern         = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*(?:\.|->|\[)`)
	returnLinePattern            = regexp.MustCompile(`(?m)^\s*return(?:\s+([^;\n]+))?`)
	partialReturnPattern         = regexp.MustCompile(`(?i)\breturn\s+[^;\n,]+,\s*(err|error)\b|\breturn\s+\{[^}\n]*(data|result|value)[^}\n]*(err|error)[^}\n]*\}`)
	identifierWordSplitPattern   = regexp.MustCompile(`[_\-\s]+`)
)

func additionalPrecisionFunctionFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	findings := make([]core.Finding, 0, 8)
	if behaviorMismatch(file, fn) {
		findings = append(findings, precisionWarnFinding(env, namingBehaviorMismatchRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s name conflicts with observed query/command behavior", fn.Name), core.ConfidenceMedium))
	}
	if hiddenMutation(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionHiddenMutationRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mutates state without an explicit command-style name", fn.Name), core.ConfidenceMedium))
	}
	if inconsistentReturnContract(fn) {
		findings = append(findings, precisionWarnFinding(env, functionInconsistentReturnContractRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes empty and value return shapes; make the success/error contract explicit", fn.Name), core.ConfidenceMedium))
	}
	if partialResult(fn) {
		findings = append(findings, precisionWarnFinding(env, functionPartialResultRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s can return a value alongside an error without an explicit partial-result contract", fn.Name), core.ConfidenceMedium))
	}
	if count, labels := responsibilityCount(fn); count >= 4 {
		findings = append(findings, precisionWarnFinding(env, functionMultipleResponsibilitiesRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s combines %d responsibilities (%s); split orchestration from focused work", fn.Name, count, strings.Join(labels, ", ")), core.ConfidenceMedium))
	}
	if orchestrationDomainMix(fn) {
		findings = append(findings, precisionWarnFinding(env, functionOrchestrationDomainMixRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes request/job orchestration with domain decisions", fn.Name), core.ConfidenceMedium))
	}
	findings = append(findings, precisionNamingFindings(env, file, fn)...)
	return findings
}

func precisionNamingFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	findings := make([]core.Finding, 0, 6)
	allNames := make([]struct {
		name string
		typ  string
		expr string
		line int
	}, 0, 1+len(fn.Params)+len(fn.Assignments))
	allNames = append(allNames, struct {
		name string
		typ  string
		expr string
		line int
	}{name: fn.Name, line: fn.StartLine})
	for _, param := range fn.Params {
		allNames = append(allNames, struct {
			name string
			typ  string
			expr string
			line int
		}{name: param.Name, typ: param.Type, line: fn.StartLine})
	}
	for _, assignment := range fn.Assignments {
		allNames = append(allNames, struct {
			name string
			typ  string
			expr string
			line int
		}{name: assignment.Name, expr: assignment.Expr, line: assignment.Line})
	}
	for _, item := range allNames {
		if item.name == "" {
			continue
		}
		if item.name == fn.Name && isReactComponentOrHookBoundary(file, fn) {
			continue
		}
		if isBooleanNameCandidate(item.name, item.typ, item.expr, fn) && !isPredicateName(item.name) && !isAllowedBooleanUIName(file, fn, item.name) {
			findings = append(findings, precisionWarnFinding(env, namingBooleanNotPredicateRuleID, file, item.line,
				fmt.Sprintf("boolean name %q should read as a predicate such as is/has/can/should", item.name), core.ConfidenceMedium))
		}
		if cardinalityMismatch(item.name, item.typ) {
			findings = append(findings, precisionWarnFinding(env, namingCardinalityMismatchRuleID, file, item.line,
				fmt.Sprintf("identifier %q has plural/singular wording that conflicts with its value shape", item.name), core.ConfidenceMedium))
		}
		if implementationLeakName(item.name) {
			findings = append(findings, precisionWarnFinding(env, namingImplementationLeakRuleID, file, item.line,
				fmt.Sprintf("identifier %q exposes infrastructure vocabulary in domain-facing naming", item.name), core.ConfidenceMedium))
		}
		if missingUnit(item.name, item.typ, item.expr) {
			findings = append(findings, precisionWarnFinding(env, namingMissingUnitRuleID, file, item.line,
				fmt.Sprintf("numeric identifier %q names a duration, size, or money value without a unit suffix", item.name), core.ConfidenceMedium))
		}
		if abbr := unknownAbbreviation(env, item.name); abbr != "" {
			findings = append(findings, precisionWarnFinding(env, namingUnknownAbbreviationRuleID, file, item.line,
				fmt.Sprintf("identifier %q contains abbreviation %q that is not established in quality_rules.naming.allowed_abbreviations", item.name, abbr), core.ConfidenceLow))
		}
	}
	return findings
}

func sourceNamingFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	findings := make([]core.Finding, 0, 3)
	if finding, ok := glossaryDriftFinding(env, file, source); ok {
		findings = append(findings, finding)
	}
	if finding, ok := roleSuffixOveruseFinding(env, file, source); ok {
		findings = append(findings, finding)
	}
	if finding, ok := crossLayerInconsistencyFinding(env, file, source); ok {
		findings = append(findings, finding)
	}
	return findings
}

func behaviorMismatch(file string, fn precisionFunction) bool {
	if isFrameworkOrchestrationBoundary(file, fn) {
		return false
	}
	name := strings.ToLower(fn.Name)
	if hiddenSideEffect(file, fn) {
		return true
	}
	if !commandFunctionPrefixPattern.MatchString(name) || mutatingFunctionEvidence(fn) {
		return false
	}
	for _, call := range fn.Calls {
		if readCallPattern.MatchString(call.Callee) {
			return true
		}
	}
	return false
}

func hiddenMutation(file string, fn precisionFunction) bool {
	if explicitMutationName(fn.Name) || isDomainSideEffectBoundaryName(fn.Name) || isFrameworkOrchestrationBoundary(file, fn) || isScriptEntrypoint(file, fn.Name) {
		return false
	}
	if isReactComponentOrNamedHookBoundary(file, fn) {
		return false
	}
	mutatesParam := mutatesParameter(fn)
	mutatesState := mutatingFunctionEvidence(fn)
	if isReactLocalStateBoundary(file, fn) && mutatesState && !mutatesParam && onlyReactHookLocalStateMutation(fn) {
		return false
	}
	if isAccumulatorBuilderFunctionName(fn.Name) && !hasLikelyExternalMutationCall(fn) && !hasLikelyParameterAssignment(fn) {
		return false
	}
	return mutatesState || mutatesParam
}

func hasLikelyExternalMutationCall(fn precisionFunction) bool {
	localTargets := localMutationTargets(fn)
	params := paramNames(fn)
	for _, call := range directCalls(fn) {
		if !mutatingCallPattern.MatchString(call.Callee) {
			continue
		}
		if isLocalMutationCall(call.Callee, localTargets) || isBuilderAccumulatorMutationCall(fn, call) {
			continue
		}
		target := mutationCallTarget(call.Callee)
		if target == "" {
			continue
		}
		if _, isParam := params[target]; isParam {
			return true
		}
		if isAccumulatorLikeLocalName(target) {
			continue
		}
		return true
	}
	return false
}

func hasLikelyParameterAssignment(fn precisionFunction) bool {
	params := paramNames(fn)
	if len(params) == 0 {
		return false
	}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) {
			continue
		}
		for _, match := range paramMutationPattern.FindAllStringSubmatch(assignmentLeftHandSide(line), -1) {
			if _, ok := params[match[1]]; ok {
				return true
			}
		}
	}
	return false
}

func mutatingFunctionEvidence(fn precisionFunction) bool {
	localTargets := localMutationTargets(fn)
	for _, call := range directCalls(fn) {
		if mutatingCallPattern.MatchString(call.Callee) && !isLocalMutationCall(call.Callee, localTargets) && !isBuilderAccumulatorMutationCall(fn, call) {
			return true
		}
	}
	for _, assignment := range directAssignments(fn) {
		if assignment.Augmented && !isLocalMutationTarget(assignment.Name, localTargets) && !isBuilderAccumulatorAssignment(fn, assignment) {
			return true
		}
	}
	return false
}

func mutatesParameter(fn precisionFunction) bool {
	params := map[string]struct{}{}
	for _, param := range fn.Params {
		if param.Name != "" {
			params[param.Name] = struct{}{}
		}
	}
	if len(params) == 0 {
		return false
	}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) {
			continue
		}
		lhs := assignmentLeftHandSide(line)
		for _, match := range paramMutationPattern.FindAllStringSubmatch(lhs, -1) {
			if _, ok := params[match[1]]; ok {
				return true
			}
		}
	}
	return false
}

func assignmentLeftHandSide(line string) string {
	for idx := 0; idx < len(line); idx++ {
		if line[idx] != '=' {
			continue
		}
		prev := byte(0)
		next := byte(0)
		if idx > 0 {
			prev = line[idx-1]
		}
		if idx+1 < len(line) {
			next = line[idx+1]
		}
		if prev == '=' || prev == '!' || prev == '<' || prev == '>' || next == '=' || next == '>' {
			continue
		}
		return line[:idx]
	}
	return line
}

func lineHasAssignmentOperator(line string) bool {
	for idx := 0; idx < len(line); idx++ {
		if line[idx] != '=' {
			continue
		}
		prev := byte(0)
		next := byte(0)
		if idx > 0 {
			prev = line[idx-1]
		}
		if idx+1 < len(line) {
			next = line[idx+1]
		}
		if prev == '=' || prev == '!' || prev == '<' || prev == '>' || next == '=' || next == '>' {
			continue
		}
		return true
	}
	return false
}

func explicitMutationName(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	return commandFunctionPrefixPattern.MatchString(lowered) ||
		conventionalMutationBoundaryPattern.MatchString(lowered) ||
		isEventHandlerName(name) ||
		strings.Contains(lowered, "mutat") ||
		strings.Contains(lowered, "persist") ||
		strings.Contains(lowered, "write")
}

func inconsistentReturnContract(fn precisionFunction) bool {
	if nextResponseNullableGuardHelper(fn) || nullableParserLookupContract(fn) {
		return false
	}
	returns := returnCategories(fn.Body)
	if returns.total < 2 {
		return false
	}
	return returns.empty && returns.value
}

func nextResponseNullableGuardHelper(fn precisionFunction) bool {
	signature := strings.ToLower(fn.Signature)
	body := strings.ToLower(fn.Body)
	hasNullableSignature := strings.Contains(signature, "nextresponse") && strings.Contains(signature, "null")
	hasNextResponseBody := strings.Contains(body, "nextresponse.") || strings.Contains(body, "return new nextresponse")
	if !hasNullableSignature && !hasNextResponseBody {
		return false
	}
	return strings.Contains(body, "return null") && hasNextResponseBody
}

func nullableParserLookupContract(fn precisionFunction) bool {
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if !regexp.MustCompile(`^(as|exists|extract|find|get|lookup|parse|read|resolve|to)`).MatchString(loweredName) {
		return false
	}
	body := strings.ToLower(fn.Body)
	if loweredName == "exists" || strings.HasPrefix(loweredName, "exists") {
		return containsAny(body, []string{"return false", "return true"})
	}
	signature := strings.ToLower(fn.Signature)
	hasNullableReturnEvidence := containsAny(signature, []string{"| null", "|null", "null", "undefined", "optional"}) ||
		containsAny(body, []string{"return null", "return undefined", "return none", "return nil"})
	return hasNullableReturnEvidence && containsAny(body, []string{"return null", "return undefined", "return none", "return nil"})
}

type returnShapeCounts struct {
	total int
	empty bool
	value bool
}

func returnCategories(body string) returnShapeCounts {
	out := returnShapeCounts{}
	for _, match := range returnLinePattern.FindAllStringSubmatch(body, -1) {
		out.total++
		expr := ""
		if len(match) > 1 {
			expr = strings.TrimSpace(match[1])
		}
		if expr == "" || isEmptyReturnExpr(expr) {
			out.empty = true
			continue
		}
		if strings.Contains(expr, ",") {
			parts := strings.Split(expr, ",")
			first := strings.TrimSpace(parts[0])
			if isEmptyReturnExpr(first) {
				out.empty = true
			} else {
				out.value = true
			}
			continue
		}
		out.value = true
	}
	return out
}

func isEmptyReturnExpr(expr string) bool {
	expr = strings.TrimSpace(strings.TrimSuffix(expr, ";"))
	switch strings.ToLower(expr) {
	case "", "nil", "none", "null", "undefined":
		return true
	default:
		return strings.HasPrefix(expr, "nil,") || strings.HasPrefix(expr, "none,") || strings.HasPrefix(expr, "null,")
	}
}

func partialResult(fn precisionFunction) bool {
	loweredName := strings.ToLower(fn.Name)
	if strings.Contains(loweredName, "partial") || strings.Contains(loweredName, "try") {
		return false
	}
	return partialReturnPattern.MatchString(fn.Body)
}

func responsibilityCount(fn precisionFunction) (int, []string) {
	if isAdapterOrchestrationName(fn.Name) {
		return 0, nil
	}
	seen := map[string]struct{}{}
	record := func(label string) {
		seen[label] = struct{}{}
	}
	body := strings.ToLower(fn.Body)
	for _, call := range fn.Calls {
		classifyResponsibility(strings.ToLower(call.Callee), record)
	}
	for _, statement := range fn.Statements {
		classifyResponsibility(strings.ToLower(statement.Text), record)
	}
	if strings.Contains(body, " if ") || strings.Contains(body, "\tif ") || strings.Contains(body, "\nif ") {
		record("decision")
	}
	labels := make([]string, 0, len(seen))
	for label := range seen {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return len(labels), labels
}

func classifyResponsibility(text string, record func(string)) {
	switch {
	case strings.Contains(text, "validat") || strings.Contains(text, "sanitize"):
		record("validate")
	case strings.Contains(text, "auth") || strings.Contains(text, "permission") || strings.Contains(text, "allow"):
		record("authorize")
	case strings.Contains(text, "fetch") || strings.Contains(text, "find") || strings.Contains(text, "load") || strings.Contains(text, "query") || strings.Contains(text, "read") || strings.Contains(text, "select"):
		record("load")
	case strings.Contains(text, "save") || strings.Contains(text, "insert") || strings.Contains(text, "update") || strings.Contains(text, "delete") || strings.Contains(text, "persist") || strings.Contains(text, "write"):
		record("write")
	case strings.Contains(text, "send") || strings.Contains(text, "publish") || strings.Contains(text, "emit") || strings.Contains(text, "notify"):
		record("send")
	case strings.Contains(text, "cache") || strings.Contains(text, "redis"):
		record("cache")
	case strings.Contains(text, "format") || strings.Contains(text, "map") || strings.Contains(text, "transform") || strings.Contains(text, "serialize") || strings.Contains(text, "json"):
		record("transform")
	case strings.Contains(text, "log") || strings.Contains(text, "metric") || strings.Contains(text, "trace"):
		record("observe")
	}
}

func orchestrationDomainMix(fn precisionFunction) bool {
	name := strings.ToLower(fn.Name)
	body := strings.ToLower(fn.Body)
	orchestrator := strings.Contains(name, "handler") || strings.Contains(name, "controller") || strings.Contains(name, "job") ||
		strings.Contains(name, "worker") || strings.Contains(fn.Signature, "Request") || strings.Contains(fn.Signature, "Response") ||
		strings.Contains(body, "request") || strings.Contains(body, "response")
	if !orchestrator {
		return false
	}
	hasInfra := lowLevelOperationPattern.MatchString(fn.Body)
	for _, call := range fn.Calls {
		lowered := strings.ToLower(call.Callee)
		if strings.Contains(lowered, "fetch") || strings.Contains(lowered, "save") || strings.Contains(lowered, "send") ||
			strings.Contains(lowered, "cache") || strings.Contains(lowered, "publish") || strings.Contains(lowered, "query") {
			hasInfra = true
			break
		}
	}
	hasDomainDecision := strings.Contains(body, "if ") && (strings.Contains(body, "order") || strings.Contains(body, "user") ||
		strings.Contains(body, "account") || strings.Contains(body, "price") || strings.Contains(body, "eligib") ||
		strings.Contains(body, "valid"))
	return hasInfra && hasDomainDecision
}

func isBooleanNameCandidate(name string, typ string, expr string, fn precisionFunction) bool {
	if name == fn.Name {
		return functionLooksBoolean(fn)
	}
	if isBooleanType(typ) {
		return true
	}
	return expr != "" && booleanExprPattern.MatchString(strings.TrimSpace(expr))
}

func functionLooksBoolean(fn precisionFunction) bool {
	if isPredicateName(fn.Name) {
		return false
	}
	for _, statement := range fn.Statements {
		text := strings.TrimSpace(strings.TrimSuffix(statement.Text, ";"))
		if strings.HasPrefix(text, "return ") && booleanExprPattern.MatchString(strings.TrimSpace(strings.TrimPrefix(text, "return "))) {
			return true
		}
	}
	return false
}

func isBooleanType(typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	return typ == "bool" || typ == "boolean" || strings.Contains(typ, " bool") || strings.Contains(typ, ": boolean")
}

func isPredicateName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, prefix := range []string{"is", "are", "has", "have", "can", "could", "should", "must", "allow", "allows", "enable", "enabled", "disable", "disabled", "needs", "requires", "supports", "valid", "visible", "ready", "show", "matches"} {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
}

func cardinalityMismatch(name string, typ string) bool {
	base := strings.ToLower(strings.Trim(name, "_$"))
	if base == "" || conventionalCardinalityName(base) || configuredPluralDomainAbbreviation(base) || strings.HasSuffix(base, "status") || strings.HasSuffix(base, "class") {
		return false
	}
	plural := isPluralName(base)
	collection := collectionTypePattern.MatchString(typ)
	scalar := scalarTypePattern.MatchString(typ)
	if plural && scalar && !collection {
		return true
	}
	return !plural && collection && !strings.Contains(base, "map") && !strings.Contains(base, "list") && !strings.Contains(base, "set")
}

func isPluralName(name string) bool {
	if unitSuffixPattern.MatchString(name) {
		return false
	}
	return strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") && !strings.HasSuffix(name, "us")
}

func implementationLeakName(name string) bool {
	words := splitIdentifierWords(name)
	if len(words) <= 1 {
		return false
	}
	for _, word := range words {
		if infraNamePattern.MatchString(word) {
			return true
		}
	}
	return false
}

func missingUnit(name string, typ string, expr string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	if unitSuffixPattern.MatchString(lowered) || strings.HasSuffix(lowered, "count") || strings.HasSuffix(lowered, "total") {
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
	allowed := allowedAbbreviations(env)
	for _, word := range splitIdentifierWords(name) {
		candidate := strings.ToLower(strings.Trim(word, "_$"))
		if candidate == "" || len(candidate) < 2 {
			continue
		}
		if _, ok := allowed[candidate]; ok {
			continue
		}
		if isAllUpper(word) && len(candidate) <= 6 {
			return word
		}
		if isSuspiciousShortening(candidate) {
			return word
		}
	}
	return ""
}

func allowedAbbreviations(env support.Context) map[string]struct{} {
	defaults := []string{"api", "ast", "aws", "ci", "cli", "cpu", "cpp", "css", "db", "dns", "dto", "env", "grpc", "html", "http", "https", "id", "io", "ip", "js", "json", "mcp", "os", "pr", "rpc", "sdk", "sql", "ssh", "tcp", "tls", "ts", "tsx", "ui", "uri", "url", "uuid", "xml", "yaml", "yml"}
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
	case "acct", "addr", "amt", "cfg", "cust", "msg", "num", "qty", "usr":
		return true
	default:
		return false
	}
}

func glossaryDriftFinding(env support.Context, file string, source string) (core.Finding, bool) {
	for concept, entry := range env.Config.Checks.QualityRules.Naming.Glossary {
		preferred := strings.ToLower(strings.TrimSpace(concept))
		terms := append([]string{preferred}, entry.Avoid...)
		seen := map[string]int{}
		for _, ident := range identifiersInSource(source) {
			for _, term := range terms {
				normalized := strings.ToLower(strings.TrimSpace(term))
				if normalized == "" {
					continue
				}
				if identifierContainsWord(ident.name, normalized) {
					seen[normalized] = ident.line
				}
			}
		}
		if len(seen) >= 2 {
			line := firstSeenLine(seen)
			return precisionWarnFinding(env, namingDomainVocabularyDriftRuleID, file, line,
				fmt.Sprintf("domain concept %q appears under multiple terms (%s); prefer one glossary vocabulary", concept, sortedKeys(seen)), core.ConfidenceMedium), true
		}
	}
	return core.Finding{}, false
}

func roleSuffixOveruseFinding(env support.Context, file string, source string) (core.Finding, bool) {
	threshold := env.Config.Checks.QualityRules.Naming.RoleSuffixWarnThreshold
	if threshold <= 0 {
		threshold = 4
	}
	seen := map[string]int{}
	for _, ident := range identifiersInSource(source) {
		if roleSuffixPattern.MatchString(ident.name) {
			seen[ident.name] = ident.line
		}
	}
	if len(seen) < threshold {
		return core.Finding{}, false
	}
	return precisionWarnFinding(env, namingRoleSuffixOveruseRuleID, file, firstSeenLine(seen),
		fmt.Sprintf("file uses %d vague role suffix names such as Manager/Helper/Util/Service/Processor", len(seen)), core.ConfidenceMedium), true
}

func crossLayerInconsistencyFinding(env support.Context, file string, source string) (core.Finding, bool) {
	groups := [][]string{
		{"restaurant", "venue", "merchant", "establishment"},
		{"user", "customer", "account"},
		{"order", "purchase", "transaction"},
	}
	layerTerms := []string{"api", "request", "response", "dto", "entity", "record", "row", "model", "repository", "domain"}
	idents := identifiersInSource(source)
	for _, group := range groups {
		seen := map[string]int{}
		for _, ident := range idents {
			if !containsAnyIdentifierWord(ident.name, layerTerms) {
				continue
			}
			for _, term := range group {
				if identifierContainsWord(ident.name, term) {
					seen[term] = ident.line
				}
			}
		}
		if len(seen) >= 2 {
			return precisionWarnFinding(env, namingCrossLayerInconsistencyRuleID, file, firstSeenLine(seen),
				fmt.Sprintf("cross-layer names use multiple terms for one concept (%s)", sortedKeys(seen)), core.ConfidenceLow), true
		}
	}
	return core.Finding{}, false
}

type identifierAtLine struct {
	name string
	line int
}

func identifiersInSource(source string) []identifierAtLine {
	matches := identifierTokenPattern.FindAllStringIndex(source, -1)
	out := make([]identifierAtLine, 0, len(matches))
	for _, match := range matches {
		out = append(out, identifierAtLine{
			name: source[match[0]:match[1]],
			line: 1 + strings.Count(source[:match[0]], "\n"),
		})
	}
	return out
}

func splitIdentifierWords(name string) []string {
	trimmed := strings.Trim(name, "_$")
	if trimmed == "" {
		return nil
	}
	var words []string
	for _, part := range identifierWordSplitPattern.Split(trimmed, -1) {
		if part == "" {
			continue
		}
		words = append(words, splitCamelWords(part)...)
	}
	return words
}

func splitCamelWords(part string) []string {
	if part == "" {
		return nil
	}
	runes := []rune(part)
	start := 0
	out := make([]string, 0, 3)
	for idx := 1; idx < len(runes); idx++ {
		prev := runes[idx-1]
		cur := runes[idx]
		nextLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
		if unicode.IsLower(prev) && unicode.IsUpper(cur) || unicode.IsUpper(prev) && unicode.IsUpper(cur) && nextLower {
			out = append(out, string(runes[start:idx]))
			start = idx
		}
	}
	out = append(out, string(runes[start:]))
	return out
}

func identifierContainsWord(identifier string, word string) bool {
	for _, candidate := range splitIdentifierWords(identifier) {
		if strings.EqualFold(candidate, word) {
			return true
		}
	}
	return false
}

func containsAnyIdentifierWord(identifier string, words []string) bool {
	for _, word := range words {
		if identifierContainsWord(identifier, word) {
			return true
		}
	}
	return false
}

func isAllUpper(value string) bool {
	hasLetter := false
	for _, r := range value {
		if !unicode.IsLetter(r) {
			continue
		}
		hasLetter = true
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return hasLetter
}

func firstSeenLine(seen map[string]int) int {
	line := 0
	for _, candidate := range seen {
		if line == 0 || candidate < line {
			line = candidate
		}
	}
	if line == 0 {
		return 1
	}
	return line
}

func sortedKeys(seen map[string]int) string {
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
