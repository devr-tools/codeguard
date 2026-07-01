package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
)

func TestParseLCOVReadsLineHits(t *testing.T) {
	report := `TN:
SF:src/app.ts
FN:1,compute
FNDA:3,compute
DA:1,3
DA:2,0
DA:4,1,checksum-is-ignored
LF:3
LH:2
end_of_record
SF:src/other.ts
DA:10,0
end_of_record
`

	profile := quality.ParseLCOV(report)

	app, ok := profile["src/app.ts"]
	if !ok {
		t.Fatalf("expected src/app.ts in profile, got %v", profile)
	}
	if app[1] != 3 || app[2] != 0 || app[4] != 1 {
		t.Fatalf("unexpected hits for src/app.ts: %v", app)
	}
	if _, hasLine3 := app[3]; hasLine3 {
		t.Fatalf("line 3 has no DA record and must stay unmeasured: %v", app)
	}
	other, ok := profile["src/other.ts"]
	if !ok || other[10] != 0 {
		t.Fatalf("unexpected hits for src/other.ts: %v", other)
	}
}

func TestParseLCOVMergesRepeatedRecordsWithMaxHits(t *testing.T) {
	report := `SF:lib.js
DA:1,0
end_of_record
SF:lib.js
DA:1,2
DA:2,0
end_of_record
`

	profile := quality.ParseLCOV(report)

	lib := profile["lib.js"]
	if lib[1] != 2 {
		t.Fatalf("expected max hit count for repeated records, got %v", lib)
	}
	if lib[2] != 0 {
		t.Fatalf("expected line 2 uncovered, got %v", lib)
	}
}

func TestParseLCOVIgnoresMalformedInput(t *testing.T) {
	report := `DA:1,5
SF:ok.js
DA:not-a-number,1
DA:3
DA:-2,1
DA:2,1
end_of_record
DA:9,9
`

	profile := quality.ParseLCOV(report)

	if len(profile) != 1 {
		t.Fatalf("expected only ok.js, got %v", profile)
	}
	ok := profile["ok.js"]
	if len(ok) != 1 || ok[2] != 1 {
		t.Fatalf("expected only valid DA record retained, got %v", ok)
	}
}

func TestParseLCOVNormalizesWindowsPaths(t *testing.T) {
	profile := quality.ParseLCOV("SF:src\\nested\\file.js\nDA:1,1\nend_of_record\n")
	if _, ok := profile["src/nested/file.js"]; !ok {
		t.Fatalf("expected backslash path to normalize, got %v", profile)
	}
}
