package arch

import (
	"cmp"
	"maps"
	"math"
	"regexp"
	"slices"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

// validationSchema names the validation report format.
const validationSchema = "anarch-lint.validation.v1"

// Verdicts on whether a package metric predicts fixes.
const (
	// metricPredictive is a metric whose ranking of the packages agrees with
	// their later fixes, size held constant, beyond chance at the 5% level.
	metricPredictive = "predictive"
	// metricNotPredictive is one whose agreement chance could explain.
	metricNotPredictive = "not-predictive"
	// metricInsufficientData is one the history cannot test: too few
	// packages or fixes, or no variation to rank.
	metricInsufficientData = "insufficient-data"
)

// Bounds below which a validation says insufficient data, and the
// significance level of its test; listed among the assumptions.
const (
	minValidationPackages = 10
	minFixCommits         = 5
	significance          = 0.05
)

// fixSubject matches the subject of a commit that fixes something, the
// heuristic of SZZ's first step (Śliwerski, Zimmermann and Zeller, MSR
// 2005), which reads commit messages.
var fixSubject = regexp.MustCompile(
	`(?i)\b(fix|fixes|fixed|fixing|bugfix|hotfix|bug|bugs|regression|revert)\b`,
)

// FixHistory is what a validation reads of the commits after a revision:
// how many of them there were, and the files each fix among them changed.
type FixHistory struct {
	// Since is the revision measured; Until is the last commit read.
	Since string `json:"since"`
	Until string `json:"until"`
	// Commits counts the first-parent commits read, and FixCommits those
	// whose subject reads as a fix.
	Commits    int `json:"commits"`
	FixCommits int `json:"fixCommits"`
	// Fixes maps each file, relative to the analysis root, to the fix
	// commits that changed it.
	Fixes map[string][]string `json:"-"`
}

// ValidationInput is the code at a revision, its model, and the fixes that
// followed it.
type ValidationInput struct {
	Program Program
	Model   Model
	History FixHistory
}

// Validation says, for each package metric at a revision, whether it
// predicted where the following commits fixed things, with package size
// held constant (IMPROVEMENTS.md §10.5). It is a report, never a gate: the
// computation is deterministic for a pinned history, but its conclusion is
// statistical.
type Validation struct {
	Schema   string     `json:"schema"`
	History  FixHistory `json:"history"`
	Packages int        `json:"packages"`
	// Metrics lists each metric's test, in a fixed order.
	Metrics     []MetricValidation `json:"metrics"`
	Assumptions []Assumption       `json:"assumptions"`
}

// MetricValidation is one metric's test: Rho is the Spearman correlation of
// the metric with the packages' fix commits, partialled on their lines of
// code, and P its two-sided p-value; both are 0 without enough data.
type MetricValidation struct {
	Metric  string  `json:"metric"`
	Rho     float64 `json:"rho"`
	P       float64 `json:"p"`
	Verdict string  `json:"verdict"`
}

// samples are the packages in order, and for each the value of every
// metric, its fix commits and its lines of code, aligned with packages.
type samples struct {
	packages     []string
	metrics      map[string][]float64
	fixes, lines []float64
}

// IsFix reports whether a commit subject reads as a fix.
func IsFix(subject string) bool { return fixSubject.MatchString(subject) }

// Validate tests each package metric of in's program against the fixes in
// its history. It is a pure function of its input.
func Validate(in *ValidationInput) Validation {
	p, m := &in.Program, &in.Model
	samples := validationSamples(p, m, &in.History)
	v := Validation{
		Schema:   validationSchema,
		History:  in.History,
		Packages: len(samples.packages),
		Metrics: make(
			[]MetricValidation,
			0,
			len(samples.metrics),
		),
		Assumptions: validationAssumptions(),
	}
	testable := len(samples.packages) >= minValidationPackages &&
		in.History.FixCommits >= minFixCommits
	for _, name := range slices.Sorted(maps.Keys(samples.metrics)) {
		v.Metrics = append(v.Metrics, samples.test(name, testable))
	}
	return v
}

// test tests one metric against the fixes, when the history is testable.
func (s *samples) test(metric string, testable bool) MetricValidation {
	mv := MetricValidation{Metric: metric, Verdict: metricInsufficientData}
	if !testable {
		return mv
	}
	rho, ok := partialSpearman(s.metrics[metric], s.fixes, s.lines)
	if !ok {
		return mv
	}
	mv.Rho, mv.P = round6(rho), round6(tTest(rho, len(s.packages)))
	mv.Verdict = metricNotPredictive
	if mv.P < significance {
		mv.Verdict = metricPredictive
	}
	return mv
}

// validationSamples measures every package under analysis: Martin's
// metrics, summed cyclomatic complexity in the model's variant and summed
// cognitive complexity, against its distinct fix commits and its lines.
func validationSamples(p *Program, m *Model, h *FixHistory) samples {
	g := BuildGraphs(p, m)
	variant := m.Limits().ComplexityVariant
	lines, complexity, cognitive := map[string]int{}, map[string]int{}, map[string]int{}
	fixes := map[string]map[string]bool{}
	for _, f := range p.Files {
		lines[f.Package] += f.Lines
		for _, c := range h.Fixes[f.Path] {
			if fixes[f.Package] == nil {
				fixes[f.Package] = map[string]bool{}
			}
			fixes[f.Package][c] = true
		}
	}
	for i := range p.Funcs {
		fn := &p.Funcs[i]
		complexity[fn.Package] += fn.Complexity.Of(variant)
		cognitive[fn.Package] += fn.Cognitive
	}
	s := samples{metrics: map[string][]float64{}}
	for _, pm := range packageMetricsOf(p, &g) {
		s.packages = append(s.packages, pm.Package)
		s.fixes = append(s.fixes, float64(len(fixes[pm.Package])))
		s.lines = append(s.lines, float64(lines[pm.Package]))
		for name, value := range map[string]float64{
			"ca": float64(pm.Ca), "ce": float64(pm.Ce), "instability": pm.Instability,
			"abstractness": pm.Abstractness, "distance": pm.Distance,
			"complexity": float64(complexity[pm.Package]), "cognitive": float64(cognitive[pm.Package]),
		} {
			s.metrics[name] = append(s.metrics[name], value)
		}
	}
	return s
}

// partialSpearman returns the Spearman correlation of x with y, partialled
// on z: the correlation of their ranks once each one's rank correlation
// with z is removed. False when a ranking does not vary, or z explains x or
// y entirely, so that no correlation is defined.
func partialSpearman(x, y, z []float64) (float64, bool) {
	rx, ry, rz := ranks(x), ranks(y), ranks(z)
	xy, xz, yz := stat.Correlation(rx, ry, nil), stat.Correlation(rx, rz, nil),
		stat.Correlation(ry, rz, nil)
	den := math.Sqrt((1 - xz*xz) * (1 - yz*yz))
	if math.IsNaN(xy) || math.IsNaN(xz) || math.IsNaN(yz) || den < 1e-12 {
		return 0, false
	}
	return (xy - xz*yz) / den, true
}

// tTest returns the two-sided p-value of a partial correlation rho over n
// samples, by Student's t with n − 3 degrees of freedom.
func tTest(rho float64, n int) float64 {
	df := float64(n - 3)
	if math.Abs(rho) >= 1 {
		return 0
	}
	t := rho * math.Sqrt(df/(1-rho*rho))
	return 2 * distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}.Survival(math.Abs(t))
}

// ranks returns each value's rank, from 1, with tied values given the mean
// of the ranks they span.
func ranks(values []float64) []float64 {
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(values[a], values[b]) })
	out := make([]float64, len(values))
	for i := 0; i < len(order); {
		j := i
		for j+1 < len(order) && values[order[j+1]] == values[order[i]] {
			j++
		}
		mean := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			out[order[k]] = mean
		}
		i = j + 1
	}
	return out
}

// validationAssumptions lists the choices behind a validation.
func validationAssumptions() []Assumption {
	return []Assumption{
		{
			ID: "fix-commits",
			Text: "A fix is a first-parent commit whose subject has the word fix, fixes, fixed, " +
				"fixing, bugfix, hotfix, bug, bugs, regression or revert; a package's fixes are the distinct fixes " +
				"that changed one of its files, as they were at the revision measured. Commit " +
				"messages lie, so this is a heuristic, as SZZ's first step is.",
			Source: "IMPROVEMENTS.md 10.5 (Śliwerski, Zimmermann, Zeller, MSR 2005)",
		},
		{
			ID: "validation-test",
			Text: "Each metric's Spearman correlation with fixes is partialled on lines of code, " +
				"since size confounds most metrics (El Emam et al., TSE 2001), and tested two-sided " +
				"by Student's t with n − 3 degrees of freedom at the 5% level. Fewer than 10 " +
				"packages or 5 fixes, or a ranking that does not vary, is insufficient data. Seven " +
				"metrics are tested at once, so one in five runs of chance alone may show one as " +
				"predictive.",
			Source: "IMPROVEMENTS.md 10.5",
		},
	}
}
