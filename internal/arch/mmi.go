package arch

import "math"

// Criterion kinds.
const (
	// Measured is a criterion computed exactly from the program and model.
	Measured CriterionKind = "measured"
	// Unmeasured is a criterion the inputs cannot determine, such as a
	// layering criterion with no layers declared. Reason says why.
	Unmeasured CriterionKind = "unmeasured"
	// Judgement is a reviewer criterion: never scored here, filled only by a
	// supplied, approved judgement (mmi.md §7.6).
	Judgement CriterionKind = "judgement"
)

// Who the MMI chapter says determines each criterion (Table 4-1).
const (
	byMetrics      = "metrics tools"
	byArchitecture = "architecture analysis tools"
	byReviewer     = "reviewers"
)

// CriterionKind says how a criterion's value was obtained.
type CriterionKind string

// Criterion is one MMI criterion (Table 4-1) with its raw value and its score
// from Table 4-2.
type Criterion struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Group string        `json:"group"`
	Kind  CriterionKind `json:"kind"`
	// DeterminedBy is the chapter's own classification of the criterion.
	DeterminedBy string `json:"determinedBy"`
	// Value is the raw measurement, in Unit; absent unless Kind is Measured.
	Value *float64 `json:"value,omitempty"`
	Unit  string   `json:"unit,omitempty"`
	// Score is the 0–10 score; absent for an unmeasured criterion and for a
	// judgement no one has supplied.
	Score *int `json:"score,omitempty"`
	// Reason explains an unmeasured criterion, or how a judgement was filled.
	Reason string `json:"reason,omitempty"`
}

// MMI is the Modularity Maturity Index reported as an interval (mmi.md §2,
// mmi-plan.md §2.4). Low counts every criterion without a score as 0 and High
// as 10, so the deterministic part is never mistaken for a judgement. The two
// are equal only when every criterion is scored.
type MMI struct {
	Low      float64     `json:"low"`
	High     float64     `json:"high"`
	Exact    bool        `json:"exact"`
	Criteria []Criterion `json:"criteria"`
}

// band is one row of Table 4-2: the ten boundaries between scores 0 and 10.
// For a higher-is-better criterion a value scores the number of boundaries it
// exceeds; for a lower-is-better one, the number it stays below.
type band struct {
	higherIsBetter bool
	bounds         [10]float64
}

// groupWeights returns the weights of Table 4-1's six groups, in Table 4-1's
// order; they sum to 1.
func groupWeights() []struct {
	group  string
	weight float64
} {
	return []struct {
		group  string
		weight float64
	}{{"1.1", 0.25}, {"1.2", 0.10}, {"1.3", 0.10}, {"2.1", 0.15}, {"2.2", 0.15}, {"3", 0.25}}
}

// table42 returns Table 4-2 of the MMI chapter. It is a function, not a
// variable, because Go has no constant maps and the table must not change.
func table42() map[string]band {
	return map[string]band{
		"1.1.1": {true, [10]float64{54, 58, 62, 66, 70, 74, 78, 82, 86, 90}},
		"1.1.2": {true, [10]float64{75, 77.5, 80, 82.5, 85, 87.5, 90, 92.5, 95, 97.5}},
		"1.1.3": {false, [10]float64{7.5, 5, 3.5, 2.5, 2, 1.5, 1.1, 0.85, 0.65, 0.5}},
		"1.1.4": {false, [10]float64{16.5, 11, 7.5, 5, 3.5, 2.5, 2, 1.5, 1.1, 0.85}},
		"1.2.1": {false, [10]float64{6.5, 4, 2.5, 1.5, 1, 0.65, 0.4, 0.25, 0.15, 0.1}},
		"1.3.1": {false, [10]float64{23, 18, 13.5, 10.5, 8, 6, 4.75, 3.5, 2.75, 2}},
		"1.3.2": {false, [10]float64{23, 18, 13.5, 10.5, 8, 6, 4.75, 3.5, 2.75, 2}},
		"1.3.3": {false, [10]float64{23, 18, 13.5, 10.5, 8, 6, 4.75, 3.5, 2.75, 2}},
		"1.3.4": {false, [10]float64{3.6, 2.6, 1.9, 1.4, 1, 0.75, 0.5, 0.4, 0.3, 0.2}},
		"2.1.1": {false, [10]float64{6.5, 4, 2.5, 1.5, 1, 0.65, 0.4, 0.25, 0.15, 0.1}},
		"2.1.2": {false, [10]float64{14, 9.6, 6.5, 4.5, 3.2, 2.25, 1.5, 1.1, 0.75, 0.5}},
		"2.2.1": {false, [10]float64{25, 22.5, 20, 17.5, 15, 12.5, 10, 7.5, 5, 2.5}},
		"2.2.2": {false, [10]float64{50, 45, 40, 35, 30, 25, 20, 15, 10, 5}},
		"2.2.3": {false, [10]float64{106, 82, 62, 48, 37, 29, 22, 17, 13, 10}},
		"2.2.4": {false, [10]float64{37, 30, 24, 19, 15, 12, 10, 8, 6, 5}},
		"3.1":   {true, [10]float64{54.5, 59, 63.5, 68, 72.5, 77, 81.5, 86, 90.5, 95}},
		"3.2":   {false, [10]float64{7.5, 5, 3.5, 2.5, 2, 1.5, 1.1, 0.85, 0.65, 0.5}},
	}
}

// ScoreOf returns the Table 4-2 score of a raw value for a scored criterion,
// and false for a criterion Table 4-2 does not score numerically.
func ScoreOf(criterion string, value float64) (int, bool) {
	b, ok := table42()[criterion]
	if !ok {
		return 0, false
	}
	score := 0
	for _, bound := range b.bounds {
		if (b.higherIsBetter && value > bound) || (!b.higherIsBetter && value < bound) {
			score++
		}
	}
	return score, true
}

// newMMI aggregates criteria into the interval: per group, the mean score
// with unscored criteria at 0 (Low) or 10 (High), weighted by the group's
// share (mmi.md §2, this article's reading of the chapter's procedure).
func newMMI(criteria []Criterion) MMI {
	type sums struct{ low, high, n float64 }
	groups := map[string]*sums{}
	for i := range criteria {
		c := &criteria[i]
		s := groups[c.Group]
		if s == nil {
			s = &sums{}
			groups[c.Group] = s
		}
		s.n++
		if c.Score != nil {
			s.low += float64(*c.Score)
			s.high += float64(*c.Score)
			continue
		}
		s.high += 10
	}
	m := MMI{Criteria: criteria}
	for _, gw := range groupWeights() {
		if s := groups[gw.group]; s != nil {
			// The explicit conversions forbid fusing the multiply and add into
			// one FMA instruction, which arm64 would round differently from
			// amd64; the same inputs must give the same MMI everywhere.
			m.Low += float64(gw.weight * (s.low / s.n))
			m.High += float64(gw.weight * (s.high / s.n))
		}
	}
	m.Low, m.High = round2(m.Low), round2(m.High)
	m.Exact = m.Low == m.High
	return m
}

// measured returns a measured criterion with its score.
func measured(id, name, by string, value float64, unit string) Criterion {
	v := round6(value)
	c := Criterion{
		ID:           id,
		Name:         name,
		Group:        groupOf(id),
		Kind:         Measured,
		DeterminedBy: by,
		Value:        &v,
		Unit:         unit,
	}
	if s, ok := ScoreOf(id, v); ok {
		c.Score = &s
	}
	return c
}

// unmeasured returns a criterion the inputs cannot determine.
func unmeasured(id, name, by, reason string) Criterion {
	return Criterion{
		ID:           id,
		Name:         name,
		Group:        groupOf(id),
		Kind:         Unmeasured,
		DeterminedBy: by,
		Reason:       reason,
	}
}

// percent returns 100 × part ÷ whole, and 0 for an empty whole: no candidates
// means no violations.
func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// groupOf returns the Table 4-1 group of a criterion: "1.1" for "1.1.3", and
// "3" for the ungrouped pattern-consistency criteria.
func groupOf(id string) string {
	if id[0] == '3' {
		return "3"
	}
	return id[:3]
}

// round2 rounds x to two decimal places, the precision an MMI is read at.
func round2(x float64) float64 {
	scaled := x * 100
	return math.Round(scaled) / 100
}
