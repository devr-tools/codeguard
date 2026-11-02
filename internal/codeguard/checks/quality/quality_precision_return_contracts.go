package quality

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	returnLinePattern    = regexp.MustCompile(`(?m)^\s*return(?:\s+([^;\n]+))?`)
	partialReturnPattern = regexp.MustCompile(`(?i)\breturn\s+[^;\n,]+,\s*(err|error)\b|\breturn\s+\{[^}\n]*(data|result|value)[^}\n]*(err|error)[^}\n]*\}`)
)

func inconsistentReturnContract(fn precisionFunction) bool {
	if nextResponseNullableGuardHelper(fn) || nullableParserLookupContract(fn) {
		return false
	}
	if explicitResultObjectContract(fn) {
		return false
	}
	if explicitNullableReturnContract(fn) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(fn.Signature), "error") {
		return false
	}
	if standardGoResultErrorContract(fn) && !returnsNonZeroValueWithError(fn) {
		return false
	}
	returns := returnCategories(fn)
	if returns.total < 2 {
		return false
	}
	return returns.empty && returns.value
}

func explicitNullableReturnContract(fn precisionFunction) bool {
	signature := strings.ToLower(fn.Signature)
	if signature == "" {
		return false
	}
	return containsAny(signature, []string{
		"| null", "|null", "| undefined", "|undefined",
		"null>", "undefined>", "optional<", "option<",
		"promise<", "result<",
	}) && containsAny(fn.Body, []string{
		"return null", "return undefined", "return none", "return nil",
	})
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

// PERF: compiled once; this detector runs per function.
var lookupFunctionNamePattern = regexp.MustCompile(`^(as|exists|extract|find|get|lookup|parse|read|resolve|to)`)

func nullableParserLookupContract(fn precisionFunction) bool {
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if !lookupFunctionNamePattern.MatchString(loweredName) {
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

func returnCategories(fn precisionFunction) returnShapeCounts {
	out := returnShapeCounts{}
	for _, match := range returnLinePattern.FindAllStringSubmatchIndex(fn.Body, -1) {
		if len(match) < 4 || returnLineIsNested(fn, bodyOffsetLine(fn, match[0])) {
			continue
		}
		out.total++
		expr := ""
		if match[2] >= 0 {
			expr = strings.TrimSpace(fn.Body[match[2]:match[3]])
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

func partialResult(file string, fn precisionFunction) bool {
	loweredName := strings.ToLower(fn.Name)
	if strings.Contains(loweredName, "partial") || strings.Contains(loweredName, "try") || explicitResultObjectContract(fn) ||
		explicitRepositoryCommandResultContract(file, fn) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(fn.Signature), "error") {
		return false
	}
	if standardGoResultErrorContract(fn) && !returnsNonZeroValueWithError(fn) {
		return false
	}
	for _, match := range partialReturnPattern.FindAllStringIndex(fn.Body, -1) {
		if !returnLineIsNested(fn, bodyOffsetLine(fn, match[0])) {
			return true
		}
	}
	return false
}

func standardGoResultErrorContract(fn precisionFunction) bool {
	first := standardGoResultErrorFirstType(fn)
	return first != ""
}

func standardGoResultErrorFirstType(fn precisionFunction) string {
	signature := strings.ToLower(strings.ReplaceAll(fn.Signature, " ", ""))
	if !strings.HasPrefix(signature, "(") || !strings.HasSuffix(signature, ",error)") {
		return ""
	}
	first := strings.TrimPrefix(strings.TrimSuffix(signature, ",error)"), "(")
	if first == "" || strings.Contains(first, ",") {
		return ""
	}
	return first
}

func explicitRepositoryCommandResultContract(file string, fn precisionFunction) bool {
	if standardGoResultErrorFirstType(fn) != "bool" {
		return false
	}
	if !hasRepositoryCommandPrefix(fn.Name) || !hasRepositoryCommandContext(file, fn) {
		return false
	}
	return goBoolErrorPathsReturnFalse(fn)
}

func hasRepositoryCommandPrefix(name string) bool {
	for _, prefix := range []string{
		"Create", "Update", "Delete", "Insert", "Upsert", "Save", "Unsave",
		"Like", "Unlike", "Record", "Mark", "Set", "Enable", "Disable",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasRepositoryCommandContext(file string, fn precisionFunction) bool {
	path := "/" + strings.ToLower(filepath.ToSlash(file))
	if strings.Contains(path, "/repository") || strings.Contains(path, "/repositories") ||
		strings.Contains(path, "/storage/") || strings.Contains(path, "/postgres/") {
		return true
	}
	return strings.Contains(strings.ToLower(fn.Receiver), "repository")
}

func goBoolErrorPathsReturnFalse(fn precisionFunction) bool {
	for _, match := range returnLinePattern.FindAllStringSubmatchIndex(fn.Body, -1) {
		if len(match) < 4 || match[2] < 0 || returnLineIsNested(fn, bodyOffsetLine(fn, match[0])) {
			continue
		}
		expr := strings.TrimSpace(fn.Body[match[2]:match[3]])
		parts := strings.Split(expr, ",")
		if len(parts) < 2 {
			continue
		}
		errExpr := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.Join(parts[1:], ","), ";")))
		if errExpr == "nil" || errExpr == "" {
			continue
		}
		if !strings.Contains(errExpr, "err") && !strings.Contains(errExpr, "error") {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[0])) != "false" {
			return false
		}
	}
	return true
}

func returnsNonZeroValueWithError(fn precisionFunction) bool {
	firstType := standardGoResultErrorFirstType(fn)
	for _, match := range returnLinePattern.FindAllStringSubmatchIndex(fn.Body, -1) {
		if len(match) < 4 || match[2] < 0 || returnLineIsNested(fn, bodyOffsetLine(fn, match[0])) {
			continue
		}
		expr := strings.TrimSpace(fn.Body[match[2]:match[3]])
		parts := strings.Split(expr, ",")
		if len(parts) < 2 {
			continue
		}
		first := strings.TrimSpace(parts[0])
		second := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(parts[1], ";")))
		if second != "err" && second != "error" {
			continue
		}
		if !isEmptyReturnExpr(first) && !isGoZeroReturnExpr(firstType, first) {
			return true
		}
	}
	return false
}

func returnLineIsNested(fn precisionFunction, line int) bool {
	return callInNestedFunction(fn, line)
}

func bodyOffsetLine(fn precisionFunction, offset int) int {
	if offset < 0 {
		return fn.StartLine
	}
	if offset > len(fn.Body) {
		offset = len(fn.Body)
	}
	return fn.StartLine + strings.Count(fn.Body[:offset], "\n")
}

func isGoZeroReturnExpr(resultType string, expr string) bool {
	resultType = strings.TrimSpace(strings.ToLower(resultType))
	expr = strings.TrimSpace(strings.TrimSuffix(expr, ";"))
	loweredExpr := strings.ToLower(expr)
	if resultType == "bool" {
		return loweredExpr == "false"
	}
	if resultType == "string" {
		return expr == `""` || expr == "``"
	}
	if scalarTypePattern.MatchString(resultType) || strings.HasPrefix(resultType, "uint") {
		return loweredExpr == "0"
	}
	if strings.HasPrefix(resultType, "*") || strings.HasPrefix(resultType, "[]") ||
		strings.HasPrefix(resultType, "map[") || strings.HasPrefix(resultType, "chan ") ||
		strings.HasPrefix(resultType, "func(") || resultType == "any" || resultType == "interface{}" {
		return loweredExpr == "nil"
	}
	if strings.HasSuffix(loweredExpr, "{}") {
		zeroType := strings.TrimSpace(strings.TrimSuffix(loweredExpr, "{}"))
		return zeroType == resultType
	}
	return false
}

func explicitResultObjectContract(fn precisionFunction) bool {
	loweredSignature := strings.ToLower(fn.Signature)
	loweredBody := strings.ToLower(fn.Body)
	return containsAny(loweredSignature, []string{"result", "parseresult", "parsed", "response"}) &&
		strings.Contains(loweredBody, "ok:") &&
		containsAny(loweredBody, []string{"value:", "error:", "data:"})
}
