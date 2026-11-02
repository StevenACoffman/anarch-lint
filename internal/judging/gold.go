package judging

import (
	"cmp"
	"slices"
)

// Agreement is one configuration's agreement with the gold labels of one
// family, in one half of the gold set, with an interval for it and the
// calibration of the confidence the judge stated. These are reports, never
// thresholds: promoting a family to automatic judging needs a bar set
// before measuring, which no family has yet (mmi-plan.md §9.3 F).
type Agreement struct {
	Family  string `json:"family"`
	Config  string `json:"config"`
	HeldOut bool   `json:"heldOut"`
	Correct int    `json:"correct"`
	Total   int    `json:"total"`
	// Low and High bound the agreement's 95% interval.
	Low  float64 `json:"low"`
	High float64 `json:"high"`
	// ECE and Brier measure how well the stated confidence matched the
	// outcomes; lower is better.
	ECE   float64 `json:"ece"`
	Brier float64 `json:"brier"`
}

// Statistics computes what an agreement reports beyond its counts: an
// interval for k agreements out of n, and the calibration of the outcomes.
// The shell supplies them, so the core imports no statistics library.
type Statistics struct {
	Interval  func(k, n int) (lo, hi float64)
	Calibrate func(outcomes []GoldOutcome) (ece, brier float64)
}

// Agreements groups the gold outcomes by family, configuration and half,
// sorted, and measures each group.
func (r *Result) Agreements(stats Statistics) []Agreement {
	type group struct {
		family, config string
		heldOut        bool
	}
	groups := map[group][]GoldOutcome{}
	for _, g := range r.Gold {
		k := group{g.Family, g.Config, g.HeldOut}
		groups[k] = append(groups[k], g)
	}
	out := make([]Agreement, 0, len(groups))
	for k, outcomes := range groups {
		a := Agreement{Family: k.family, Config: k.config, HeldOut: k.heldOut, Total: len(outcomes)}
		for _, o := range outcomes {
			if o.Correct {
				a.Correct++
			}
		}
		a.Low, a.High = stats.Interval(a.Correct, a.Total)
		a.ECE, a.Brier = stats.Calibrate(outcomes)
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b Agreement) int {
		return cmp.Or(cmp.Compare(a.Family, b.Family), cmp.Compare(a.Config, b.Config),
			boolOrder(a.HeldOut)-boolOrder(b.HeldOut))
	})
	return out
}

// boolOrder puts false before true.
func boolOrder(b bool) int {
	if b {
		return 1
	}
	return 0
}
