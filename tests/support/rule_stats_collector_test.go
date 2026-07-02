package support_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

type ruleStatsOp struct {
	kind   string // "emit" or "suppress"
	ruleID string
	reason string
}

func TestRuleStatsCollectorSnapshot(t *testing.T) {
	cases := []struct {
		name string
		ops  []ruleStatsOp
		want []core.RuleStatsEntry
	}{
		{
			name: "empty_collector_yields_nil",
			ops:  nil,
			want: nil,
		},
		{
			name: "emitted_only",
			ops: []ruleStatsOp{
				{kind: "emit", ruleID: "quality.gofmt"},
				{kind: "emit", ruleID: "quality.gofmt"},
			},
			want: []core.RuleStatsEntry{
				{RuleID: "quality.gofmt", Emitted: 2, SuppressionRatio: 0},
			},
		},
		{
			name: "suppressed_only_reaches_ratio_one",
			ops: []ruleStatsOp{
				{kind: "suppress", ruleID: "security.secret", reason: runnersupport.SuppressionReasonBaseline},
				{kind: "suppress", ruleID: "security.secret", reason: runnersupport.SuppressionReasonWaiver},
			},
			want: []core.RuleStatsEntry{
				{RuleID: "security.secret", BaselineSuppressed: 1, WaiverSuppressed: 1, SuppressionRatio: 1},
			},
		},
		{
			name: "mixed_reasons_split_per_mechanism_and_rules_sorted",
			ops: []ruleStatsOp{
				{kind: "suppress", ruleID: "b.rule", reason: runnersupport.SuppressionReasonInline},
				{kind: "emit", ruleID: "a.rule"},
				{kind: "suppress", ruleID: "a.rule", reason: runnersupport.SuppressionReasonBaseline},
				{kind: "suppress", ruleID: "a.rule", reason: runnersupport.SuppressionReasonWaiver},
				{kind: "suppress", ruleID: "a.rule", reason: runnersupport.SuppressionReasonInline},
			},
			want: []core.RuleStatsEntry{
				{RuleID: "a.rule", Emitted: 1, BaselineSuppressed: 1, WaiverSuppressed: 1, InlineSuppressed: 1, SuppressionRatio: 0.75},
				{RuleID: "b.rule", InlineSuppressed: 1, SuppressionRatio: 1},
			},
		},
		{
			name: "unknown_reason_and_empty_rule_id_are_ignored",
			ops: []ruleStatsOp{
				{kind: "suppress", ruleID: "a.rule", reason: "bogus"},
				{kind: "emit", ruleID: ""},
				{kind: "suppress", ruleID: "", reason: runnersupport.SuppressionReasonWaiver},
			},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector := runnersupport.NewRuleStatsCollector()
			for _, op := range tc.ops {
				switch op.kind {
				case "emit":
					collector.RecordEmitted(op.ruleID)
				case "suppress":
					collector.RecordSuppressed(op.ruleID, op.reason)
				}
			}
			got := collector.Snapshot()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Snapshot() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestRuleStatsCollectorNilSafe locks in that a nil collector (e.g. a
// hand-built Context in tests) ignores records instead of panicking.
func TestRuleStatsCollectorNilSafe(t *testing.T) {
	var collector *runnersupport.RuleStatsCollector
	collector.RecordEmitted("a.rule")
	collector.RecordSuppressed("a.rule", runnersupport.SuppressionReasonWaiver)
	if got := collector.Snapshot(); got != nil {
		t.Fatalf("nil collector Snapshot() = %#v, want nil", got)
	}
}

// TestRuleStatsCollectorConcurrent hammers one collector from many goroutines
// (sections run in parallel and file scanning may parallelize within a
// section); run under -race it also proves the collector is data-race free.
func TestRuleStatsCollectorConcurrent(t *testing.T) {
	const workers = 16
	const perWorker = 200
	collector := runnersupport.NewRuleStatsCollector()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				collector.RecordEmitted("shared.rule")
				collector.RecordSuppressed("shared.rule", runnersupport.SuppressionReasonBaseline)
				collector.RecordSuppressed("shared.rule", runnersupport.SuppressionReasonWaiver)
				collector.RecordSuppressed("shared.rule", runnersupport.SuppressionReasonInline)
			}
		}()
	}
	wg.Wait()

	got := collector.Snapshot()
	want := []core.RuleStatsEntry{{
		RuleID:             "shared.rule",
		Emitted:            workers * perWorker,
		BaselineSuppressed: workers * perWorker,
		WaiverSuppressed:   workers * perWorker,
		InlineSuppressed:   workers * perWorker,
		SuppressionRatio:   0.75,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}
