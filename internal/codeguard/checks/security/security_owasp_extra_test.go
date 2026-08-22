package security

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAppendOWASPExtraLineFindingsSuppressesCommentOnlyMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name   string
		raw    string
		masked string
	}{
		{name: "bind all", raw: "// 0.0.0.0 is mentioned in documentation"},
		{name: "cors wildcard", raw: "# Access-Control-Allow-Origin: * in an example", masked: "# Access-Control-Allow-Origin: * in an example"},
		{name: "weak hash", raw: "// crypto/md5 is discussed in a migration note"},
		{name: "weak cipher", raw: "/* cipher.getInstance(\"DES\") appears in prose */"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, "fixture.go", 1, tc.raw, tc.masked)
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func TestAppendOWASPExtraLineFindingsKeepsRealMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name   string
		file   string
		raw    string
		masked string
		rule   string
	}{
		{name: "bind all", file: "server.yaml", raw: `host: "0.0.0.0"`, masked: `host: "0.0.0.0"`, rule: "security.bind-all-interfaces"},
		{name: "cors wildcard", file: "headers.yaml", raw: `Access-Control-Allow-Origin: *`, masked: `Access-Control-Allow-Origin: *`, rule: "security.cors-wildcard"},
		{name: "weak hash", file: "digest.go", raw: `sum := md5.New()`, masked: `sum := md5.New()`, rule: "security.weak-hash"},
		{name: "weak cipher", file: "crypto.java", raw: `Cipher.getInstance("DES")`, masked: `Cipher.getInstance("DES")`, rule: "security.weak-cipher"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, tc.file, 7, tc.raw, tc.masked)
			if len(findings) != 1 {
				t.Fatalf("findings = %#v, want exactly one", findings)
			}
			if findings[0].RuleID != tc.rule || findings[0].Line != 7 {
				t.Fatalf("finding = %#v, want %s at line 7", findings[0], tc.rule)
			}
		})
	}
}

func TestAppendOWASPExtraLineFindingsSuppressesTrailingCommentMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "weak hash", raw: `return nil // md5.New() was removed`},
		{name: "weak cipher", raw: `return nil /* cipher.getInstance("DES") was removed */`},
		{name: "cors wildcard", raw: `return nil // Access-Control-Allow-Origin: * example`},
		{name: "bind all", raw: `return nil // 0.0.0.0 example`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, "fixture.go", 1, tc.raw, maskedSourceForFile("fixture.go", tc.raw))
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func TestFindingsForFileSuppressesConfigCommentOnlyMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name string
		file string
		data string
	}{
		{name: "yaml cors", file: "headers.yaml", data: "# Access-Control-Allow-Origin: * example\n"},
		{name: "env bind all", file: ".env", data: "# HOST=0.0.0.0\n"},
		{name: "docker weak hash", file: "Dockerfile", data: "# crypto/md5 was removed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := findingsForFile(env, tc.file, []byte(tc.data))
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func testSecurityContext() support.Context {
	return support.Context{NewFinding: func(input support.FindingInput) core.Finding {
		return core.Finding{
			RuleID:  input.RuleID,
			Level:   input.Level,
			Path:    input.Path,
			Line:    input.Line,
			Column:  input.Column,
			Message: input.Message,
		}
	}}
}
