package arch

// Shrink returns the baseline with what in shows was fixed taken out: the
// findings in no longer has, and each metric at the better of its two values.
// It never adds a finding or a metric, so the accepted state only ever gets
// stricter (ArchUnit's FreezingArchRule, without its refreeze). A baseline
// measured under another model is returned as it is: its numbers mean
// something else. So is one measured in another build, whose findings this
// build may not see: shrinking it would drop them, and they would be new
// again in their own build.
func Shrink(b *Baseline, in *GateInput) Baseline {
	out := Baseline{
		Schema:    b.Schema,
		ModelHash: b.ModelHash,
		Findings:  make([]BaselineFinding, 0, len(b.Findings)),
		Metrics:   append(make([]Metric, 0, len(b.Metrics)), b.Metrics...),
		Build:     b.Build,
	}
	if b.ModelHash != in.ModelHash || otherBuild(b.Build, &in.Build) != nil {
		out.Findings = append(out.Findings, b.Findings...)
		return out
	}
	current := map[string]bool{}
	for i := range in.Findings {
		current[in.Findings[i].Fingerprint] = true
	}
	for _, f := range b.Findings {
		if current[f.Fingerprint] {
			out.Findings = append(out.Findings, f)
		}
	}
	_, better := metricChanges(b.Metrics, in.Metrics)
	improved := map[string]float64{}
	for _, c := range better {
		improved[c.Metric] = c.Current
	}
	for i := range out.Metrics {
		if v, ok := improved[out.Metrics[i].Name]; ok {
			out.Metrics[i].Value = v
		}
	}
	return out
}
