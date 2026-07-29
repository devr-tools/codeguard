package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	defensiveUnvalidatedBoundaryInputRuleID  = "defensive.unvalidated-boundary-input"
	defensiveInvalidStateRepresentableRuleID = "defensive.invalid-state-representable"
	defensiveNullAssumptionRuleID            = "defensive.null-assumption"
	defensiveIntegerOverflowRuleID           = "defensive.integer-overflow"
	defensiveSequenceCollisionRiskRuleID     = "defensive.sequence-collision-risk"
	defensiveBoundsAssumptionRuleID          = "defensive.bounds-assumption"
	defensiveUnsafeDefaultRuleID             = "defensive.unsafe-default"
	defensiveNonExhaustiveBranchRuleID       = "defensive.non-exhaustive-branch"
	defensiveUncheckedExternalResponseRuleID = "defensive.unchecked-external-response"
	defensiveMissingSchemaValidationRuleID   = "defensive.missing-schema-validation"
	defensiveMissingResourceLimitRuleID      = "defensive.missing-resource-limit"
	defensiveInvalidStateTransitionRuleID    = "defensive.invalid-state-transition"
	defensiveFailOpenAuthorizationRuleID     = "defensive.fail-open-authorization"
)

var (
	indexAccessPattern       = regexp.MustCompile(`\b([A-Za-z_][\w$]*(?:\.[A-Za-z_][\w$]*)?)\s*\[\s*([^\]\n]+)\s*\]`)
	jsonDecodePattern        = regexp.MustCompile(`(?i)(json\.Unmarshal|json\.NewDecoder|JSON\.parse|json\.loads|nlohmann::json::parse|decode_json|parseJson)`)
	externalCallPattern      = regexp.MustCompile(`(?i)(http\.Get|client\.Do|fetch\s*\(|axios\.|requests\.(get|post|put|delete)|curl_easy_perform|httplib::|http_client)`)
	resourceReadPattern      = regexp.MustCompile(`(?i)(io\.ReadAll|ReadAll|read_to_string|read_to_end|\.read\s*\(|\.text\s*\(|arrayBuffer\s*\(|bodyParser|multer|formData\s*\(|request\.body|r\.Body)`)
	ormCollectionReadPattern = regexp.MustCompile(`(?i)\bfindMany\s*\(`)
	unsafeDefaultPattern     = regexp.MustCompile(`(?i)(getenv|process\.env|os\.environ|std::getenv|config).*?(default|fallback|\|\||!=|,\s*['"]).*?(true|false|allow|disable|skip|insecure)`)
	switchLikePattern        = regexp.MustCompile(`(?i)\b(switch|match)\b[^{:\n]*(status|state|kind|type)`)
	stateAssignmentPattern   = regexp.MustCompile(`(?i)(status|state)\s*(?:=|:=|=>)\s*["']?(paid|active|complete|completed|shipped|deleted|approved)["']?`)
	authFailOpenPattern      = regexp.MustCompile(`(?is)(except|catch)\b[^{:\n]*(?:\{|:)[^}\n]*return\s+(true|allow|nil|none)`)
	structStartPattern       = regexp.MustCompile(`(?i)\b(type\s+\w+\s+struct|interface\s+\w+|class\s+\w+|struct\s+\w+)`)
	boolFieldPattern         = regexp.MustCompile(`(?i)\b(bool|boolean)\b`)
	stringStateFieldPattern  = regexp.MustCompile(`(?i)\b(status|state|kind)\b.*\b(string|str|std::string|String)\b|\b(string|str|std::string|String)\b.*\b(status|state|kind)\b`)
	resourceCountGuard       = regexp.MustCompile(`(?i)\b(?:count|size|length|len|bytes)\s*(?:<=|<|>|>=)\s*(?:max|limit|quota|cap|[0-9])`)
	resourceNamedCountLimit  = regexp.MustCompile(`(?i)\b(?:max|limit|quota|cap)[A-Za-z0-9_]*(?:count|size|length|len|bytes)\b`)
	sequenceAllocationLine   = regexp.MustCompile(`(?i)\b(?:external[_]?id|next[_]?id|sequence|slug|number)\b.*(?:count|max)\s*\+\s*1|(?:count|max)\s*\+\s*1.*\b(?:external[_]?id|next[_]?id|sequence|slug|number)\b`)
	jsonReaderSchemaCall     = regexp.MustCompile(`(?i)\b(?:read|parse|decode)Json[A-Za-z0-9_]*\s*\([^)\n,]+,\s*[A-Za-z_$][\w$]*(?:Schema|Validator|Codec|Parser)\b`)
	prismaTakePattern        = regexp.MustCompile(`(?is)\b(?:findMany|findFirst|findUnique|query|search)\s*\([^)]*\btake\s*:`)
	sequenceIndexKeyPattern  = regexp.MustCompile(`(?i)^(?:i|j|n|idx|index|offset|position|pos|[A-Za-z_$][\w$]*(?:Index|Idx|Offset|Position|Pos))$`)
)

func defensiveBoundaryFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	body := functionRawBody(fn)
	loweredBody := strings.ToLower(body)
	findings := make([]core.Finding, 0)

	if line, ok := unvalidatedBoundaryInputLine(file, fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveUnvalidatedBoundaryInputRuleID, file, line,
			"boundary input is consumed without validation or schema checks", core.ConfidenceMedium))
	}
	if line, ok := nullAssumptionLine(file, fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveNullAssumptionRuleID, file, line,
			"nullable boundary value is dereferenced without a nil/null guard", core.ConfidenceMedium))
	}
	if line, message, confidence, ok := sequenceCollisionRiskLine(file, fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveSequenceCollisionRiskRuleID, file, line,
			message, confidence))
	}
	if line, ok := integerOverflowLine(file, fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveIntegerOverflowRuleID, file, line,
			"arithmetic on count, size, or length input lacks an overflow bound check", core.ConfidenceMedium))
	}
	if line, ok := boundsAssumptionLine(file, fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveBoundsAssumptionRuleID, file, line,
			"indexed access assumes collection bounds without a nearby length check", core.ConfidenceMedium))
	}
	if line, ok := unsafeDefaultLine(fn.Statements); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveUnsafeDefaultRuleID, file, line,
			"configuration default can fail open or disable a safety control", core.ConfidenceHigh))
	}
	if !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) {
		if line, ok := nonExhaustiveBranchLine(fn, loweredBody); ok {
			findings = append(findings, precisionWarnFinding(env, defensiveNonExhaustiveBranchRuleID, file, line,
				"enum-like branch over state, status, kind, or type lacks default/exhaustive handling", core.ConfidenceMedium))
		}
	}
	if line, ok := uncheckedExternalResponseLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveUncheckedExternalResponseRuleID, file, line,
			"external response is consumed without checking status, ok, or transport error", core.ConfidenceMedium))
	}
	if !isUIHelperOrMappingContext(file, fn) && !isReactComponentOrHookBoundary(file, fn) {
		if line, ok := missingSchemaValidationLine(fn, loweredBody); ok {
			findings = append(findings, precisionWarnFinding(env, defensiveMissingSchemaValidationRuleID, file, line,
				"decoded JSON or event payload is used without schema or invariant validation", core.ConfidenceMedium))
		}
	}
	if !isUIHelperOrMappingContext(file, fn) && !isReactComponentOrHookBoundary(file, fn) {
		if line, ok := missingResourceLimitLine(fn, loweredBody); ok {
			findings = append(findings, precisionWarnFinding(env, defensiveMissingResourceLimitRuleID, file, line,
				"boundary read or upload lacks an explicit size/count/time resource limit", core.ConfidenceMedium))
		}
	}
	if line, ok := invalidStateTransitionLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveInvalidStateTransitionRuleID, file, line,
			"state transition writes a terminal state without checking the allowed prior state", core.ConfidenceMedium))
	}
	if line, ok := failOpenAuthorizationLine(fn, loweredBody); ok {
		findings = append(findings, precisionWarnFinding(env, defensiveFailOpenAuthorizationRuleID, file, line,
			"authorization failure path defaults to allow/success", core.ConfidenceHigh))
	}
	return findings
}

func sourceDefensiveInvariantFindings(env support.Context, file string, source string) []core.Finding {
	if isQualityFixturePath(file) {
		return nil
	}
	if isScriptLikeSourcePath(file) && isLikelyUIFile(file) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	for idx := 0; idx < len(lines); idx++ {
		if !structStartPattern.MatchString(lines[idx]) || !structuralStateContainerLine(lines[idx]) {
			continue
		}
		if structuralDataTransferContainerLine(lines[idx]) {
			continue
		}
		boolFields := 0
		hasStringState := false
		for lookahead := idx; lookahead < len(lines) && lookahead <= idx+12; lookahead++ {
			line := lines[lookahead]
			boolFields += len(boolFieldPattern.FindAllString(line, -1))
			if stringStateFieldPattern.MatchString(line) {
				hasStringState = true
			}
			if strings.Contains(line, "}") {
				break
			}
		}
		if boolFields >= 2 || hasStringState {
			return []core.Finding{precisionWarnFinding(env, defensiveInvalidStateRepresentableRuleID, file, idx+1,
				"state shape uses booleans or raw status strings that can represent impossible combinations", core.ConfidenceMedium)}
		}
	}
	return nil
}

func structuralStateContainerLine(line string) bool {
	lowered := strings.ToLower(line)
	return strings.Contains(lowered, "struct") || strings.Contains(lowered, "interface") || strings.Contains(lowered, "class")
}

func structuralDataTransferContainerLine(line string) bool {
	fields := strings.Fields(strings.NewReplacer("{", " ", "<", " ", "(", " ").Replace(line))
	for idx, field := range fields {
		lowered := strings.ToLower(strings.Trim(field, "_$"))
		if lowered != "interface" && lowered != "type" && lowered != "class" && lowered != "struct" {
			continue
		}
		if idx+1 >= len(fields) {
			return false
		}
		name := strings.ToLower(strings.Trim(fields[idx+1], "_$"))
		return strings.HasSuffix(name, "args") ||
			strings.HasSuffix(name, "input") ||
			strings.HasSuffix(name, "options") ||
			strings.HasSuffix(name, "opts") ||
			strings.HasSuffix(name, "row") ||
			strings.Contains(name, "rowpart") ||
			strings.HasSuffix(name, "part") ||
			regexp.MustCompile(`part\d+$`).MatchString(name) ||
			strings.HasSuffix(name, "context") ||
			strings.HasSuffix(name, "ctx") ||
			strings.Contains(name, "dto") ||
			strings.Contains(name, "seed")
	}
	return false
}

func unvalidatedBoundaryInputLine(file string, fn precisionFunction, loweredBody string) (int, bool) {
	if isUIHelperOrMappingContext(file, fn) || isReactComponentOrHookBoundary(file, fn) || isLikelyUIFile(file) || isFrontendLibraryPath(file) {
		return 0, false
	}
	if !boundaryFunctionName(fn.Name) && !hasBoundaryParam(fn.Params) {
		return 0, false
	}
	if isValidationOrExtractionHelperName(fn.Name) {
		return 0, false
	}
	if containsAny(strings.ToLower(fn.Name), []string{"oauth", "origin", "url"}) {
		return 0, false
	}
	if validatedBoundaryInputPattern(fn, loweredBody) {
		return 0, false
	}
	if formDataHasContentLengthPreflight(loweredBody) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"request", "req.", "event", "payload", "body", "json", "params", "query"}) {
		return fn.StartLine, true
	}
	return 0, false
}

func formDataHasContentLengthPreflight(loweredBody string) bool {
	return strings.Contains(loweredBody, "formdata") &&
		containsAny(loweredBody, []string{"content-length", "contentlength"}) &&
		containsAny(loweredBody, []string{"> max", "> limit", "max_upload", "upload too large"})
}

func validatedBoundaryInputPattern(fn precisionFunction, loweredBody string) bool {
	if containsAny(loweredBody, []string{"validate", "schema", "sanitize", "bind", "decodevalid", "safeparse", "z.safeparse", "zod.", "yup.", "pydantic", "jsonschema"}) {
		return true
	}
	if jsonReaderSchemaCall.MatchString(functionRawBody(fn)) {
		return true
	}
	if strings.Contains(loweredBody, "nextresponse.") && containsAny(loweredBody, []string{"return nextresponse", ".json(", "redirect("}) && containsAny(loweredBody, []string{"if (!", "if (!", "if(", "if "}) {
		return true
	}
	if regexp.MustCompile(`(?i)\b(parse|assert|guard|ensure|decode)[A-Z_][A-Za-z0-9_]*(?:Input|Payload|Body|Params|Query|Record|Request|Event|Config)?\s*\(`).MatchString(functionRawBody(fn)) {
		return true
	}
	return false
}

func isValidationOrExtractionHelperName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	if strings.HasPrefix(lowered, "parse") || strings.HasPrefix(lowered, "assert") ||
		strings.HasPrefix(lowered, "guard") || strings.HasPrefix(lowered, "ensure") ||
		strings.HasPrefix(lowered, "decode") || strings.HasPrefix(lowered, "validate") {
		return true
	}
	return containsAny(lowered, []string{"bearertokenfrom", "tokenfrom", "headerfrom", "requestbodyfrom"})
}

func hasBoundaryParam(params []support.ParsedParam) bool {
	for _, param := range params {
		name := strings.ToLower(strings.Trim(param.Name, "_$"))
		typ := strings.ToLower(param.Type)
		switch name {
		case "req", "event", "payload", "body", "params", "query":
			return true
		case "request":
			if typ == "" || isTransportRequestType(typ) {
				return true
			}
		case "input":
			if typ == "" || containsAny(typ, []string{"unknown", "any", "record", "json", "request", "payload", "body", "params", "query"}) {
				return true
			}
		default:
			if strings.HasSuffix(name, "payload") || strings.HasSuffix(name, "body") || strings.HasSuffix(name, "params") || strings.HasSuffix(name, "query") {
				return true
			}
		}
	}
	return false
}

func isTransportRequestType(typ string) bool {
	typ = strings.TrimSpace(strings.ToLower(typ))
	return typ == "request" ||
		strings.Contains(typ, "nextrequest") ||
		strings.Contains(typ, "httprequest") ||
		strings.Contains(typ, "express.request") ||
		strings.Contains(typ, "incomingmessage")
}

func nullAssumptionLine(file string, fn precisionFunction, loweredBody string) (int, bool) {
	if isUIHelperOrMappingContext(file, fn) || isReactComponentOrHookBoundary(file, fn) {
		return 0, false
	}
	for _, param := range fn.Params {
		name := strings.ToLower(strings.Trim(param.Name, "*& "))
		if name == "" || !nullableParam(param) {
			continue
		}
		if nullableParamGuarded(loweredBody, name) {
			continue
		}
		if nullableUseLine(fn, name) > 0 {
			return firstUseLine(fn, name), true
		}
	}
	return 0, false
}

func nullableParam(param support.ParsedParam) bool {
	typ := strings.ToLower(strings.TrimSpace(param.Type))
	if typ == "" {
		return false
	}
	if strings.Contains(typ, "*") || strings.Contains(typ, "optional") || strings.Contains(typ, "maybe") {
		return true
	}
	if strings.Contains(typ, "{") && strings.Contains(typ, "}") &&
		!containsAny(typ, []string{"} | null", "}|null", "} | undefined", "}|undefined", "} | none", "}|none"}) {
		return false
	}
	if strings.HasSuffix(strings.TrimSpace(typ), "[]") &&
		!regexp.MustCompile(`\]\s*\|\s*(null|undefined|none)\b`).MatchString(typ) {
		return false
	}
	return regexp.MustCompile(`(^|\|)\s*(null|undefined|none)\b|\b(null|undefined|none)\s*\|`).MatchString(typ) ||
		strings.Contains(typ, "?")
}

func nullableParamGuarded(loweredBody string, name string) bool {
	guards := []string{
		name + " == nil",
		name + " != nil",
		name + " is none",
		name + " is not none",
		name + " === null",
		name + " !== null",
		name + " == null",
		name + " != null",
		name + " == nullptr",
		name + " != nullptr",
		"typeof " + name + " === ",
		"typeof " + name + " == ",
	}
	if containsAny(loweredBody, guards) {
		return true
	}
	quotedName := regexp.QuoteMeta(name)
	return regexp.MustCompile(`if\s*\(\s*!\s*`+quotedName+`\s*\)\s*(?:return|throw|continue|break)\b`).MatchString(loweredBody) ||
		nullableParamHasBlockExitGuard(loweredBody, quotedName)
}

func nullableParamHasBlockExitGuard(loweredBody string, quotedName string) bool {
	guardStart := regexp.MustCompile(`if\s*\(\s*!\s*` + quotedName + `\s*\)\s*\{`).FindStringIndex(loweredBody)
	if guardStart == nil {
		return false
	}
	windowStart := guardStart[1]
	windowEnd := windowStart + 3000
	if windowEnd > len(loweredBody) {
		windowEnd = len(loweredBody)
	}
	return regexp.MustCompile(`\b(?:return|throw|continue|break)\b`).MatchString(loweredBody[windowStart:windowEnd])
}

func nullableUseLine(fn precisionFunction, name string) int {
	for _, statement := range fn.Statements {
		lowered := strings.ToLower(firstNonEmptyString(statement.Raw, statement.Text))
		if nullableStatementUsesOnlyNullSafeOperators(lowered, name) {
			continue
		}
		if containsAny(lowered, []string{name + ".", name + "->", name + "[", "*" + name}) {
			return statement.Line
		}
	}
	return 0
}

func nullableStatementUsesOnlyNullSafeOperators(statement string, name string) bool {
	return containsAny(statement, []string{name + "?.", name + "?.[", name + " ??"})
}

func firstUseLine(fn precisionFunction, name string) int {
	if line := nullableUseLine(fn, name); line > 0 {
		return line
	}
	return fn.StartLine
}

func integerOverflowLine(file string, fn precisionFunction, loweredBody string) (int, bool) {
	if isUIRenderArithmeticContext(file, fn, loweredBody) || isSeedOrScriptSourcePath(file) {
		return 0, false
	}
	if sequenceAllocationArithmetic(loweredBody) || metricStatArithmeticContext(fn, loweredBody) || dateCountFormattingContext(fn, loweredBody) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"maxint", "math.max", "checked", "saturating", "overflow", "limits<", "safeint"}) {
		return 0, false
	}
	if !regexp.MustCompile(`(?i)\b(count|size|length|len|capacity|offset|total|bytes)\b`).MatchString(loweredBody) {
		return 0, false
	}
	if !resourceAllocationArithmeticContext(loweredBody) {
		return 0, false
	}
	arithmetic := regexp.MustCompile(`[A-Za-z_][\w$]*\s*(\*|\+|<<)\s*[A-Za-z0-9_]`)
	for _, statement := range fn.Statements {
		if arithmetic.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return statement.Line, true
		}
	}
	return 0, false
}

func resourceAllocationArithmeticContext(loweredBody string) bool {
	return containsAny(loweredBody, []string{
		"buffer.alloc", "allocunsafe", "new uint8array", "new arraybuffer", "new array(",
		"make([]", "bytearray(", "vector<", "reserve(", "resize(", "setlength(", "content-length", "contentlength",
		"readall", "read_all", "readtoend", "read_to_end",
	})
}

func sequenceCollisionRiskLine(file string, fn precisionFunction, loweredBody string) (int, string, string, bool) {
	if isSeedOrScriptSourcePath(file) || isLikelyUIFile(file) || !sequenceAllocationArithmetic(loweredBody) {
		return 0, "", core.ConfidenceLow, false
	}
	line := firstSequenceAllocationLine(fn)
	if guardedSequenceCollisionRetry(loweredBody) {
		return line,
			"external ID allocation is protected by bounded unique-collision retry, but count-derived IDs remain architectural debt; prefer a database sequence or transactional allocator",
			core.ConfidenceLow,
			true
	}
	return line,
		"external ID allocation derives the next value from current count; use a database sequence, UUID, or transactional allocator instead of count-based generation",
		core.ConfidenceMedium,
		true
}

func sequenceAllocationArithmetic(loweredBody string) bool {
	if !containsAny(loweredBody, []string{"count + 1", "count+1", "max + 1", "max+1"}) {
		return false
	}
	return containsAny(loweredBody, []string{"externalid", "external_id", "nextid", "next_id", "sequence", "slug", "number"})
}

func firstSequenceAllocationLine(fn precisionFunction) int {
	for _, statement := range fn.Statements {
		raw := firstNonEmptyString(statement.Raw, statement.Text)
		if sequenceAllocationLine.MatchString(raw) {
			return statement.Line
		}
	}
	return fn.StartLine
}

func guardedSequenceCollisionRetry(loweredBody string) bool {
	if containsAny(loweredBody, []string{"withexternalidretry", "with_external_id_retry", "externalidretry", "external_id_retry"}) &&
		containsAny(loweredBody, []string{"p2002", "unique", "collision", "externalid", "external_id"}) {
		return true
	}
	if !containsAny(loweredBody, []string{"p2002", "unique", "collision", "prisma"}) {
		return false
	}
	if !containsAny(loweredBody, []string{"retry", "attempt", "for "}) {
		return false
	}
	return containsAny(loweredBody, []string{"count + 1", "count+1", "externalid", "external_id", "nextid", "next_id"})
}

func metricStatArithmeticContext(fn precisionFunction, loweredBody string) bool {
	loweredName := strings.ToLower(fn.Name)
	if containsAny(loweredName, []string{"metric", "metrics", "stat", "stats", "counter", "histogram", "telemetry"}) {
		return true
	}
	return containsAny(loweredBody, []string{"metric.", "metrics.", "counter.", "histogram", "stat.", "stats.", "telemetry", "prometheus", "datadog"})
}

func dateCountFormattingContext(fn precisionFunction, loweredBody string) bool {
	loweredName := strings.ToLower(fn.Name)
	if !containsAny(loweredName, []string{"format", "display", "label", "render", "summary", "calendar", "date", "time", "bucket", "group"}) {
		return false
	}
	return containsAny(loweredBody, []string{
		"date", "time", "calendar", "duration", "intl.", "datetimeformat", "formatdistance",
		"formatrelative", "plural", "label", "title", "subtitle", "bucket", "startof", "endof",
		"adddays", "subdays", "dayjs", "date-fns", "`${", " + \"", " + '",
	})
}

func isUIRenderArithmeticContext(file string, fn precisionFunction, loweredBody string) bool {
	if isUIHelperOrMappingContext(file, fn) {
		return true
	}
	if !isScriptLikeSourcePath(file) || !isLikelyUIFile(file) {
		return false
	}
	if isUIRenderHelperName(fn.Name) || isUIRenderMappingBody(fn.Body) {
		return true
	}
	return containsAny(loweredBody, []string{
		"stylesheet.", "dimensions.", "pixelratio.", "fontscale", "spacing",
		"padding", "margin", "width", "height", "opacity", "zindex",
	})
}

func isSeedOrScriptSourcePath(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	base := normalized
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	return strings.Contains(normalized, "/scripts/") ||
		strings.Contains(normalized, "/script/") ||
		strings.Contains(normalized, "/prisma/") && strings.HasSuffix(normalized, ".ts") ||
		strings.Contains(normalized, "/seed") ||
		strings.Contains(normalized, "/seeds/") ||
		strings.Contains(normalized, "/backfill") ||
		strings.Contains(normalized, "/import") ||
		strings.HasPrefix(base, "seed") ||
		strings.Contains(base, "seed") ||
		strings.HasPrefix(base, "backfill") ||
		strings.Contains(base, "backfill") ||
		strings.HasPrefix(base, "import") ||
		strings.Contains(base, "import") ||
		strings.HasPrefix(base, "cleanup")
}

func boundsAssumptionLine(file string, fn precisionFunction, loweredBody string) (int, bool) {
	if isUIHelperOrMappingContext(file, fn) || isReactComponentOrHookBoundary(file, fn) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"len(", ".length", ".size()", "empty()", "bounds", "range", "count >"}) {
		return 0, false
	}
	for idx, statement := range fn.Statements {
		raw := firstNonEmptyString(statement.Raw, statement.Text)
		if strings.Contains(raw, "map[") {
			continue
		}
		match := indexAccessPattern.FindStringSubmatch(raw)
		if match == nil {
			continue
		}
		if !indexExpressionLooksSequenceAccess(match[1], match[2], raw) {
			continue
		}
		if nearbyBoundsGuard(fn.Statements, idx, match[1]) {
			continue
		}
		return statement.Line, true
	}
	return 0, false
}

func indexExpressionLooksSequenceAccess(target string, key string, raw string) bool {
	loweredTarget := strings.ToLower(strings.TrimSpace(target))
	loweredKey := strings.ToLower(strings.TrimSpace(strings.Trim(key, `"'`)))
	loweredRaw := strings.ToLower(raw)
	if strings.Contains(loweredTarget, "process.env") || strings.Contains(loweredTarget, "env") ||
		strings.Contains(loweredTarget, "map") || strings.Contains(loweredTarget, "dict") || strings.Contains(loweredTarget, "lookup") {
		return false
	}
	if containsAny(loweredRaw, []string{"record<", "map<", "dictionary", "dict", "object.", "hasown", " in ", ".has("}) {
		return false
	}
	if regexp.MustCompile(`^\d+$`).MatchString(loweredKey) {
		return true
	}
	if sequenceIndexKeyPattern.MatchString(strings.TrimSpace(strings.Trim(key, `"'`))) {
		return true
	}
	return containsAny(loweredTarget, []string{"array", "list", "slice", "items", "rows", "columns", "chars", "parts", "tokens", "segments", "lines", "values"}) &&
		!containsAny(loweredKey, []string{"name", "key", "id", "type", "status", "field"})
}

func nearbyBoundsGuard(statements []support.ParsedStatement, idx int, target string) bool {
	target = strings.ToLower(strings.TrimSpace(strings.Split(target, ".")[0]))
	if target == "" {
		return false
	}
	start := idx - 3
	if start < 0 {
		start = 0
	}
	for lookback := start; lookback <= idx && lookback < len(statements); lookback++ {
		line := strings.ToLower(firstNonEmptyString(statements[lookback].Raw, statements[lookback].Text))
		if strings.Contains(line, "len("+target+")") ||
			strings.Contains(line, target+".length") ||
			strings.Contains(line, target+".size()") ||
			strings.Contains(line, target+".empty()") {
			return true
		}
	}
	return false
}

func unsafeDefaultLine(statements []support.ParsedStatement) (int, bool) {
	for _, statement := range statements {
		if unsafeDefaultPattern.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return statement.Line, true
		}
	}
	return 0, false
}

func nonExhaustiveBranchLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !switchLikePattern.MatchString(functionRawBody(fn)) || containsAny(loweredBody, []string{"default", "else", "unreachable", "assert_never", "exhaustive"}) {
		return 0, false
	}
	return fn.StartLine, true
}

func uncheckedExternalResponseLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !externalCallPattern.MatchString(functionRawBody(fn)) {
		return 0, false
	}
	if urlProtocolAllowlistPattern(loweredBody) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"status", ".ok", "err != nil", "if err", "error", "catch", "raise_for_status", "response_code"}) {
		return 0, false
	}
	if containsAny(loweredBody, []string{".json", "readall", ".text", ".body", "json()"}) {
		return firstPatternLine(fn, externalCallPattern), true
	}
	return 0, false
}

func urlProtocolAllowlistPattern(loweredBody string) bool {
	if !containsAny(loweredBody, []string{"new url(", ".protocol"}) {
		return false
	}
	return containsAny(loweredBody, []string{"https:", "http:", "allowedprotocol", "allowed_protocol", "protocols.includes", "includes(url.protocol)", "protocol !==", "protocol !="})
}

func missingSchemaValidationLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !jsonDecodePattern.MatchString(functionRawBody(fn)) {
		return 0, false
	}
	if validatedBoundaryInputPattern(fn, loweredBody) || jsonReaderSchemaCall.MatchString(functionRawBody(fn)) || containsAny(loweredBody, []string{"jsonschema", "isvalid", "required"}) {
		return 0, false
	}
	return firstPatternLine(fn, jsonDecodePattern), true
}

func missingResourceLimitLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !resourceReadPattern.MatchString(functionRawBody(fn)) {
		if line, ok := missingORMCollectionLimitLine(fn, loweredBody); ok {
			return line, true
		}
		return 0, false
	}
	if uploadValidationHelperPattern(loweredBody) {
		return 0, false
	}
	if resourceLimitProofPattern(loweredBody) {
		return 0, false
	}
	if boundedReadByteLengthCheck(loweredBody) {
		return 0, false
	}
	if preFormDataContentLengthHelperGuard(loweredBody) {
		return 0, false
	}
	return firstPatternLine(fn, resourceReadPattern), true
}

func missingORMCollectionLimitLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !ormCollectionReadPattern.MatchString(functionRawBody(fn)) {
		return 0, false
	}
	if resourceLimitProofPattern(loweredBody) {
		return 0, false
	}
	if !ormCollectionReadRequiresExplicitLimit(fn, loweredBody) {
		return 0, false
	}
	return firstPatternLine(fn, ormCollectionReadPattern), true
}

func ormCollectionReadRequiresExplicitLimit(fn precisionFunction, loweredBody string) bool {
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if containsAny(loweredName, []string{"search", "list", "page", "feed", "autocomplete"}) {
		return true
	}
	if boundaryFunctionName(fn.Name) && containsAny(loweredBody, []string{"request", "query", "params", "cursor", "page", "search"}) {
		return true
	}
	return containsAny(loweredBody, []string{"cursor", "skip:", "offset:", "searchparams", "query."})
}

func resourceLimitProofPattern(loweredBody string) bool {
	if containsAny(loweredBody, []string{
		"limitreader", "maxbytes", "max_bytes", "content-length", "contentlength",
		"limit(", "take(", "take:", ".take", "slice(", ".slice(", "buffer_size", "quota",
		"maxresults", "max_results", "page_size", "pagesize",
	}) {
		return true
	}
	return resourceCountGuard.MatchString(loweredBody) || resourceNamedCountLimit.MatchString(loweredBody) || prismaTakePattern.MatchString(loweredBody)
}

func uploadValidationHelperPattern(loweredBody string) bool {
	return containsAny(loweredBody, []string{"validateinternaluploadfile", "validateuploadfile", "validatefileupload", "validateupload"}) ||
		containsAny(loweredBody, []string{"internal_upload_max_bytes", "upload_max_bytes", "max_upload_bytes", "max_file_bytes"})
}

func boundedReadByteLengthCheck(loweredBody string) bool {
	if !containsAny(loweredBody, []string{"arraybuffer", ".text", "readall", ".read"}) {
		return false
	}
	return containsAny(loweredBody, []string{"bytelength", "byte_length", ".length > max", ".length > limit", "buffer.length", "bytes.length"})
}

func preFormDataContentLengthHelperGuard(loweredBody string) bool {
	if !strings.Contains(loweredBody, "formdata") {
		return false
	}
	return containsAny(loweredBody, []string{
		"assertcontentlength", "ensurecontentlength", "validatecontentlength", "guardcontentlength",
		"assertrequestsize", "ensurerequestsize", "validaterequestsize", "guardrequestsize",
		"assertuploadsize", "ensureuploadsize", "validateuploadsize", "guarduploadsize",
	})
}

func invalidStateTransitionLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !stateAssignmentPattern.MatchString(functionRawBody(fn)) {
		return 0, false
	}
	if containsAny(loweredBody, []string{"cantransition", "allowed", "from", "previous", "current", "validtransition", "state machine"}) {
		return 0, false
	}
	return firstPatternLine(fn, stateAssignmentPattern), true
}

func failOpenAuthorizationLine(fn precisionFunction, loweredBody string) (int, bool) {
	if !containsAny(strings.ToLower(fn.Name), []string{"auth", "allow", "permission", "policy"}) && !containsAny(loweredBody, []string{"authorize", "permission", "authz", "policy"}) {
		return 0, false
	}
	if authFailOpenPattern.MatchString(loweredBody) || regexp.MustCompile(`(?is)(?:if\s+)?(?:\w+\s*:=\s*)?authorize\([^)]*\)\s*;\s*err\s*!=\s*nil\s*\{[^}]*return\s+(true|nil)`).MatchString(loweredBody) || regexp.MustCompile(`(?is)err\s*!=\s*nil\s*\{[^}]*return\s+(true|nil)`).MatchString(loweredBody) {
		return fn.StartLine, true
	}
	return 0, false
}

func firstPatternLine(fn precisionFunction, pattern *regexp.Regexp) int {
	for _, statement := range fn.Statements {
		if pattern.MatchString(firstNonEmptyString(statement.Raw, statement.Text)) {
			return statement.Line
		}
	}
	return fn.StartLine
}
