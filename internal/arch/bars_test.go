package arch_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestLoosened names each way a change loosens the bars, and nothing for a
// change that tightens them.
func TestLoosened(t *testing.T) {
	t.Parallel()
	base := func() arch.Bars {
		return arch.Bars{
			Model: arch.Model{
				Version:    1,
				Thresholds: arch.Thresholds{LargeFuncLines: 40},
				Gates:      arch.Gates{MinCoverage: 90},
				Balance: arch.BalanceSettings{
					History: arch.HistoryWindow{Commits: 100, MaxHotspot: 50},
				},
				ForbiddenImports: []arch.ForbiddenImport{
					{From: []string{"x"}, Imports: []string{"y"}, Reason: "r"},
				},
				Fitness: arch.Fitness{Calls: []arch.CallRule{{
					RuleOptions: arch.RuleOptions{Name: "clock"}, From: []string{"x"},
					Callees: []string{"time.Now"},
				}}},
			},
			Baseline: &arch.Baseline{
				Findings: []arch.BaselineFinding{{Fingerprint: "aaaa"}},
				Metrics:  []arch.Metric{{Name: "types.sdi", Value: 2}},
			},
			Waivers: []arch.Waiver{{Fingerprint: "bbbb", Expires: "2026-12-31"}},
		}
	}
	cases := map[string]struct {
		change func(b *arch.Bars)
		want   []string
	}{
		"nothing": {change: func(*arch.Bars) {}},
		"tightened": {change: func(b *arch.Bars) {
			b.Model.Balance.History = arch.HistoryWindow{Commits: 200, MaxHotspot: 40}
			b.Model.Thresholds.LargeFuncLines = 30
			b.Model.Gates.MinCoverage = 95
			b.Baseline.Findings = nil
			b.Baseline.Metrics[0].Value = 1
			b.Waivers = nil
		}},
		"the history window narrowed": {
			change: func(b *arch.Bars) {
				b.Model.Balance.History = arch.HistoryWindow{Commits: 10, MaxHotspot: 80}
			},
			want: []string{
				"balance.history.commits lowered from 100 to 10",
				"balance.history.maxHotspot raised or removed from 50",
			},
		},
		"the history window removed": {
			change: func(b *arch.Bars) { b.Model.Balance.History = arch.HistoryWindow{} },
			want:   []string{"balance.history removed"},
		},
		"a threshold raised": {
			change: func(b *arch.Bars) { b.Model.Thresholds.LargeFuncLines = 80 },
			want:   []string{"thresholds.largeFuncLines raised from 40 to 80"},
		},
		"a floor lowered": {
			change: func(b *arch.Bars) { b.Model.Gates.MinCoverage = 50 },
			want:   []string{"gates.minCoverage lowered from 90 to 50"},
		},
		"a rule removed": {
			change: func(b *arch.Bars) {
				b.Model.ForbiddenImports = nil
				b.Model.Fitness.Calls = nil
			},
			want: []string{"forbiddenImports shrank from 1 to 0", "fitness.calls clock removed"},
		},
		"a gate weakened": {
			change: func(b *arch.Bars) { b.Model.Fitness.Calls[0].Gate = arch.GateWarn },
			want:   []string{"fitness.calls clock gate weakened from fail to warn"},
		},
		"the baseline grown": {
			change: func(b *arch.Bars) {
				b.Baseline.Findings = append(
					b.Baseline.Findings,
					arch.BaselineFinding{Fingerprint: "cccc"},
				)
				b.Baseline.Metrics[0].Value = 3
			},
			want: []string{
				"the baseline accepts 1 finding it did not",
				"the baseline's types.sdi loosened from 2 to 3",
			},
		},
		"a waiver added and one extended": {
			change: func(b *arch.Bars) {
				b.Waivers = []arch.Waiver{
					{Fingerprint: "bbbb", Expires: "2027-06-30"},
					{Fingerprint: "dddd", Expires: "2026-12-31"},
				}
			},
			want: []string{
				"the waiver of bbbb extended from 2026-12-31 to 2027-06-30",
				"a waiver of dddd added",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			prev, next := base(), base()
			tc.change(&next)
			equals(t, tc.want, arch.Loosened(&prev, &next))
		})
	}
}
