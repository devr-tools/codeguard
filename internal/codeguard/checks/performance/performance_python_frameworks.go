package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Framework-aware Python rules (performance_rules.detect_framework_patterns).
// Every rule is gated on file-level framework evidence — a Django import or
// `.objects.` manager usage for the Django rules, a SQLAlchemy import for the
// session rule — so plain-Python code never matches.
var (
	pythonDjangoEvidence     = regexp.MustCompile(`(?m)^\s*(?:from\s+django[.\s]|import\s+django\b)|\.objects\.`)
	pythonSQLAlchemyEvidence = regexp.MustCompile(`(?m)^\s*(?:from\s+sqlalchemy[.\s]|import\s+sqlalchemy\b)`)
	pythonRelatedPrefetch    = regexp.MustCompile(`\bselect_related\s*\(|\bprefetch_related\s*\(`)
	pythonForLoopHeader      = regexp.MustCompile(`^\s*for\s+(\w+)\s+in\s+(.+?):`)
	pythonQuerysetAssign     = regexp.MustCompile(`^\s*(\w+)\s*=\s*\S.*\.objects\.`)
	// pythonAttrChain matches <base>.<relation>.<attr>; the scan then requires
	// <base> to be an active queryset-loop variable, drops chains whose middle
	// segment is `objects` (that is a manager call, covered by the ORM rule),
	// and drops chains whose final segment is immediately called (scalar method
	// calls like item.name.strip() are usually not relation loads).
	pythonAttrChain = regexp.MustCompile(`\b(\w+)\.(\w+)\.(\w+)\b`)
	// pythonReverseRelation matches <base>.<relation>_set. — the Django reverse
	// relation manager — which is a relation traversal even when called.
	pythonReverseRelation = regexp.MustCompile(`\b(\w+)\.(\w+)_set\.`)
	// pythonORMQueryCall and pythonSessionGetCall are deliberately disjoint
	// from pythonQueryCallPattern (the generic n-plus-one rule): they cover
	// only the ORM point-query shapes the generic pattern misses, so one line
	// never reports under both rules.
	pythonORMQueryCall   = regexp.MustCompile(`\.objects\.(?:get|filter)\s*\(`)
	pythonSessionGetCall = regexp.MustCompile(`\bsession\.get\s*\(`)
)

// pythonFrameworkScan carries the framework-aware state for one file scan. A
// nil pointer (toggle disabled, or no framework evidence in the file) turns
// every hook into a no-op.
type pythonFrameworkScan struct {
	django     bool
	sqlalchemy bool
	// prefetched is true when select_related/prefetch_related appears anywhere
	// in the file; the relation rule then stays quiet for the whole file.
	prefetched   bool
	querysetVars map[string]struct{}
	loopVars     []pythonQuerysetLoopVar
}

// pythonQuerysetLoopVar is an indent-delimited region in which name iterates a
// Django queryset.
type pythonQuerysetLoopVar struct {
	indent int
	name   string
}

func newPythonFrameworkScan(rules core.PerformanceRulesConfig, source string) *pythonFrameworkScan {
	if !toggleEnabled(rules.DetectFrameworkPatterns) {
		return nil
	}
	django := pythonDjangoEvidence.MatchString(source)
	sqlalchemy := pythonSQLAlchemyEvidence.MatchString(source)
	if !django && !sqlalchemy {
		return nil
	}
	return &pythonFrameworkScan{
		django:     django,
		sqlalchemy: sqlalchemy,
		prefetched: pythonRelatedPrefetch.MatchString(source),
	}
}

// observe is called once per non-blank line, after the generic checks, with
// the loop state the generic scan computed for the same line.
func (f *pythonFrameworkScan) observe(s *pythonPerformanceScan, lineNo int, line string, indent int, inLoop bool) {
	if f == nil {
		return
	}
	for len(f.loopVars) > 0 && indent <= f.loopVars[len(f.loopVars)-1].indent {
		f.loopVars = f.loopVars[:len(f.loopVars)-1]
	}
	if m := pythonQuerysetAssign.FindStringSubmatch(line); m != nil {
		if f.querysetVars == nil {
			f.querysetVars = map[string]struct{}{}
		}
		f.querysetVars[m[1]] = struct{}{}
	}
	isLoopHeader := pythonLoopStartPattern.MatchString(line)
	f.checkDjangoRelation(s, lineNo, line)
	f.checkORMQueryInLoop(s, lineNo, line, inLoop, isLoopHeader)
	if header := pythonForLoopHeader.FindStringSubmatch(line); header != nil && f.iterableIsQueryset(header[2]) {
		f.loopVars = append(f.loopVars, pythonQuerysetLoopVar{indent: indent, name: header[1]})
	}
}

// iterableIsQueryset reports whether a for-loop iterable expression looks like
// a Django queryset: an inline manager expression, or a variable previously
// assigned from one.
func (f *pythonFrameworkScan) iterableIsQueryset(expr string) bool {
	if strings.Contains(expr, ".objects.") {
		return true
	}
	_, tracked := f.querysetVars[strings.TrimSpace(expr)]
	return tracked
}

func (f *pythonFrameworkScan) isActiveLoopVar(name string) bool {
	for _, loopVar := range f.loopVars {
		if loopVar.name == name {
			return true
		}
	}
	return false
}

// checkDjangoRelation implements performance.python.django-nplusone-relation:
// relation traversal on a queryset-loop variable in a file with Django
// evidence and no select_related/prefetch_related anywhere.
func (f *pythonFrameworkScan) checkDjangoRelation(s *pythonPerformanceScan, lineNo int, line string) {
	if !f.django || f.prefetched || len(f.loopVars) == 0 {
		return
	}
	for _, m := range pythonReverseRelation.FindAllStringSubmatch(line, -1) {
		if f.isActiveLoopVar(m[1]) {
			s.addFinding("performance.python.django-nplusone-relation", lineNo,
				"reverse relation access on a queryset row inside the loop issues one query per row; load it up front with prefetch_related")
			return
		}
	}
	for _, m := range pythonAttrChain.FindAllStringSubmatchIndex(line, -1) {
		base := line[m[2]:m[3]]
		middle := line[m[4]:m[5]]
		if !f.isActiveLoopVar(base) || middle == "objects" {
			continue
		}
		if rest := strings.TrimLeft(line[m[1]:], " \t"); strings.HasPrefix(rest, "(") {
			continue
		}
		s.addFinding("performance.python.django-nplusone-relation", lineNo,
			"attribute access through a related object inside a queryset loop issues one query per row; load the relation up front with select_related or prefetch_related")
		return
	}
}

// checkORMQueryInLoop implements performance.python.orm-query-in-loop: Django
// .objects.get/.objects.filter or SQLAlchemy session.get inside a loop body.
// Loop headers are exempt (the iterable there runs once, not per iteration),
// and any line the generic n-plus-one pattern already covers is skipped so
// the two rules stay disjoint.
func (f *pythonFrameworkScan) checkORMQueryInLoop(s *pythonPerformanceScan, lineNo int, line string, inLoop bool, isLoopHeader bool) {
	if !inLoop || isLoopHeader || pythonQueryCallPattern.MatchString(line) {
		return
	}
	if f.django && pythonORMQueryCall.MatchString(line) {
		s.addFinding("performance.python.orm-query-in-loop", lineNo,
			"Django ORM query inside a loop issues one query per iteration; batch it before the loop with in_bulk(), filter(pk__in=...), or values_list")
		return
	}
	if f.sqlalchemy && pythonSessionGetCall.MatchString(line) {
		s.addFinding("performance.python.orm-query-in-loop", lineNo,
			"SQLAlchemy session.get inside a loop issues one query per iteration; fetch the rows in one query with an in_() filter before the loop")
	}
}
