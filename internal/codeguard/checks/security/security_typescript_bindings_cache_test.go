package security

import "testing"

func TestTypeScriptModulePatternCachesOnlyFixedScannerModules(t *testing.T) {
	t.Parallel()

	trustedFirst := typeScriptModulePattern(tsNamedImportPattern, "child_process")
	trustedSecond := typeScriptModulePattern(tsNamedImportPattern, "child_process")
	if trustedFirst != trustedSecond {
		t.Fatal("fixed scanner module pattern was recompiled")
	}

	untrustedFirst := typeScriptModulePattern(tsNamedImportPattern, "attacker-controlled")
	untrustedSecond := typeScriptModulePattern(tsNamedImportPattern, "attacker-controlled")
	if untrustedFirst == untrustedSecond {
		t.Fatal("runtime-derived module pattern was retained globally")
	}
}

func TestTypeScriptModulePatternQuotesRuntimeModuleName(t *testing.T) {
	t.Parallel()

	pattern := typeScriptModulePattern(tsNamedImportPattern, `unsafe.*module`)
	if !pattern.MatchString(`import { exec } from "unsafe.*module"`) {
		t.Fatal("literal runtime module name did not match")
	}
	if pattern.MatchString(`import { exec } from "unsafe-xyz-module"`) {
		t.Fatal("runtime module name was interpreted as a regular expression")
	}
}
