package text

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// renderer writes Markdown. It escapes the model's prose, so that a reason
// such as "<img onerror=…>" renders as text, never as HTML (modelith's
// ADR-0014). A table cell also escapes its column separator. A code span
// shows entities literally and no HTML, so it escapes only the separator
// and its own delimiter. A quoted Mermaid label uses Mermaid's entity codes.
type renderer struct {
	b                      strings.Builder
	prose, cell, code, mer *strings.Replacer
}

// WriteArchitecture renders the model as Markdown for people and agents to
// read: the subsystems, the layers, the measured subsystem dependencies as a
// Mermaid flowchart, the import rules, the fitness rules with their reasons,
// the finding rules, the rules grouped by quality attribute, and the domain
// models. The output depends only on the input, so a file
// rendered from it can be checked for drift. Every piece of model text is
// escaped.
func WriteArchitecture(w io.Writer, in *arch.Input) error {
	r := newRenderer()
	m := &in.Model
	b := &r.b
	b.WriteString("# Architecture\n\n")
	fmt.Fprintf(b, "<!-- Rendered by anarch-lint render from the model %s. "+
		"Edit the model, not this file. -->\n", short(m.Hash()))
	r.subsystemTable(m.Subsystems)
	r.layerList(m.Layers)
	r.dependencyChart(m, arch.BuildGraphs(&in.Program, m).Subsystems)
	r.importRules(m)
	r.ruleReasons(m.Fitness.Rules())
	r.findingRules(m.FindingRules)
	r.qualityAttributes(m.ReasonedRules())
	r.domainModels(in)
	if _, err := io.WriteString(w, b.String()); err != nil {
		return &arch.Error{Op: "text.WriteArchitecture", Err: err}
	}
	return nil
}

func newRenderer() *renderer {
	return &renderer{
		prose: strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;"),
		cell: strings.NewReplacer(
			"&", "&amp;", "<", "&lt;", ">", "&gt;", "|", `\|`, "\n", " ", "\r", " ",
		),
		code: strings.NewReplacer("|", `\|`, "`", "'", "\n", " ", "\r", " "),
		mer: strings.NewReplacer(
			"&", "#38;", `"`, "#34;", "<", "#60;", ">", "#62;", "\n", " ", "\r", " ",
		),
	}
}

func (r *renderer) subsystemTable(subs []arch.Subsystem) {
	rows := make([][]string, len(subs))
	for i, s := range subs {
		rows[i] = []string{
			r.cellOf(s.Name), r.cellsOf(s.Packages), r.cellsOf(s.Interface), r.cellOf(s.Owner),
		}
	}
	r.table("Subsystems", []string{"Subsystem", "Packages", "Interface", "Owner"}, rows)
}

// table writes a section holding an aligned table of already-escaped cells,
// or nothing when there are no rows.
func (r *renderer) table(title string, header []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, len(header))
	for _, row := range append([][]string{header}, rows...) {
		for i, c := range row {
			widths[i] = max(widths[i], 3, utf8.RuneCountInString(c))
		}
	}
	line := func(row []string) {
		for i, c := range row {
			fmt.Fprintf(
				&r.b,
				"| %s%s ",
				c,
				strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)),
			)
		}
		r.b.WriteString("|\n")
	}
	fmt.Fprintf(&r.b, "\n## %s\n\n", title)
	line(header)
	dashes := make([]string, len(widths))
	for i, w := range widths {
		dashes[i] = strings.Repeat("-", w)
	}
	line(dashes)
	for _, row := range rows {
		line(row)
	}
}

func (r *renderer) layerList(layers []arch.Layer) {
	b := &r.b
	if len(layers) == 0 {
		return
	}
	b.WriteString("\n## Layers\n\nFrom the top, which may import every layer below it:\n\n")
	for i, l := range layers {
		fmt.Fprintf(b, "%d. **%s**: %s\n", i+1, r.prose.Replace(l.Name),
			r.cellsOf(l.Packages))
	}
}

// dependencyChart draws the measured dependencies between declared
// subsystems, each edge labelled with the number of package imports it
// stands for, and an edge that runs up the subsystem order or lies on a
// cycle drawn in red and labelled so (IMPROVEMENTS.md §9.9).
func (r *renderer) dependencyChart(m *arch.Model, g arch.Graph) {
	b := &r.b
	if len(m.Subsystems) == 0 {
		return
	}
	id := map[string]string{}
	b.WriteString(
		"\n## Subsystem Dependencies\n\nAs measured in the code:\n\n```mermaid\nflowchart TD\n",
	)
	for i, s := range m.Subsystems {
		id[s.Name] = fmt.Sprintf("s%d", i)
		fmt.Fprintf(b, "  %s[\"%s\"]\n", id[s.Name], r.mer.Replace(s.Name))
	}
	problems := edgeProblems(m, g)
	var red []string
	n := 0
	for _, e := range g.Edges() {
		from, okFrom := id[e.From]
		to, okTo := id[e.To]
		if !okFrom || !okTo {
			continue
		}
		label := strconv.Itoa(e.Weight)
		if p := problems[[2]string{e.From, e.To}]; p != "" {
			label += ", " + p
			red = append(red, strconv.Itoa(n))
		}
		fmt.Fprintf(b, "  %s -->|%s| %s\n", from, label, to)
		n++
	}
	if len(red) > 0 {
		fmt.Fprintf(b, "  linkStyle %s stroke:#d62728,stroke-width:2px\n", strings.Join(red, ","))
	}
	b.WriteString("```\n")
}

// edgeProblems names what is wrong with each edge between subsystems:
// "cycle" when both ends are in one cycle group, else "upward" when it runs
// up the subsystem order.
func edgeProblems(m *arch.Model, g arch.Graph) map[[2]string]string {
	group := map[string]int{}
	for i, grp := range g.CycleGroups() {
		for _, sub := range grp {
			group[sub] = i + 1
		}
	}
	out := map[[2]string]string{}
	for _, e := range g.Edges() {
		from, to := slices.Index(m.SubsystemOrder, e.From), slices.Index(m.SubsystemOrder, e.To)
		switch {
		case group[e.From] != 0 && group[e.From] == group[e.To]:
			out[[2]string{e.From, e.To}] = "cycle"
		case from >= 0 && to >= 0 && from > to:
			out[[2]string{e.From, e.To}] = "upward"
		}
	}
	return out
}

func (r *renderer) importRules(m *arch.Model) {
	forbidden := make([][]string, len(m.ForbiddenImports))
	for i, f := range m.ForbiddenImports {
		forbidden[i] = []string{r.cellsOf(f.From), r.cellsOf(f.Imports), r.cellOf(f.Reason)}
	}
	r.table("Forbidden Imports", []string{"From", "May not import", "Reason"}, forbidden)
	isolations := make([][]string, len(m.Isolations))
	for i, iso := range m.Isolations {
		isolations[i] = []string{
			r.cellsOf([]string{iso.From}), r.cellsOf(iso.MayImport), r.cellOf(iso.Reason),
		}
	}
	r.table("Isolations", []string{"From", "May import only", "Reason"}, isolations)
}

func (r *renderer) ruleReasons(rules []arch.DeclaredRule) {
	rows := make([][]string, len(rules))
	for i := range rules {
		rule := &rules[i]
		rows[i] = []string{
			r.cellOf(rule.Name), rule.Kind, string(cmp.Or(rule.Gate, arch.GateFail)),
			r.cellOf(rule.Reason),
		}
	}
	r.table("Fitness Rules", []string{"Rule", "Kind", "Gate", "Reason"}, rows)
}

// findingRules lists the gates and reasons the model gives the rules that
// have no entry of their own.
func (r *renderer) findingRules(rules map[arch.RuleName]arch.FindingRule) {
	rows := make([][]string, 0, len(rules))
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		fr := rules[name]
		gate := "as the rule gives"
		if fr.Gate != "" {
			gate = string(fr.Gate)
		}
		rows = append(rows, []string{"`" + string(name) + "`", gate, r.cellOf(fr.Reason)})
	}
	r.table("Finding Rules", []string{"Rule", "Gate", "Reason"}, rows)
}

// qualityAttributes groups the declared rules by the quality each protects,
// in ISO 25010's order: the model read as the module's nonfunctional
// requirements. A rule that names no attribute is left out.
func (r *renderer) qualityAttributes(rules []arch.ReasonedRule) {
	var rows [][]string
	for _, q := range arch.QualityAttributes() {
		var names []string
		for _, rule := range rules {
			if rule.QualityAttribute == q {
				names = append(names, rule.Name)
			}
		}
		if len(names) > 0 {
			rows = append(rows, []string{string(q), r.cellsOf(names)})
		}
	}
	r.table("Quality Attributes", []string{"Attribute", "Rules"}, rows)
}

// domainModels lists each domain model a fitness rule names: the
// subsystem it describes, and its entities and enums.
//
// Requires: in.Domains holds one model per fitness.domains rule, in order.
func (r *renderer) domainModels(in *arch.Input) {
	b := &r.b
	rules := in.Model.Fitness.Domains
	if len(rules) == 0 {
		return
	}
	b.WriteString("\n## Domain Models\n")
	for i := range rules {
		d := &in.Domains[i]
		fmt.Fprintf(b, "\n### %s\n\n", r.cellsOf([]string{d.Path}))
		if sub := rules[i].Subsystem; sub != "" {
			fmt.Fprintf(b, "Describes subsystem %s.", r.prose.Replace(sub))
		} else {
			b.WriteString("Describes the whole program.")
		}
		if d.Shared {
			b.WriteString(" Shared with other models.")
		}
		b.WriteString("\n")
		entities := make([]string, len(d.Entities))
		for j, e := range d.Entities {
			entities[j] = e.Name
		}
		r.names("Entities", entities)
		enums := make([]string, len(d.Enums))
		for j, e := range d.Enums {
			enums[j] = e.Name + " (" + strings.Join(e.Values, ", ") + ")"
		}
		r.names("Enums", enums)
	}
}

func (r *renderer) names(label string, names []string) {
	b := &r.b
	if len(names) > 0 {
		fmt.Fprintf(b, "\n%s: %s.\n", label, r.prose.Replace(strings.Join(names, ", ")))
	}
}

// cellOf escapes model text for a Markdown table cell.
func (r *renderer) cellOf(s string) string { return r.cell.Replace(s) }

// cellsOf joins patterns into one table cell, each as code.
func (r *renderer) cellsOf(patterns []string) string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		out[i] = "`" + r.code.Replace(p) + "`"
	}
	return strings.Join(out, ", ")
}
