// Package plantuml reads a PlantUML component diagram into a model
// skeleton, as ArchUnit's adhereToPlantUmlDiagram reads one into rules
// (TODO.md, "The model from a diagram"). Each component whose stereotypes
// name package patterns, such as [Orders] <<example.com/shop/orders/...>>,
// is a subsystem; each arrow, such as [Orders] --> [Catalog], lets one
// component import another; every import the diagram does not draw is
// forbidden, by an isolation per component. A package in no component is a
// finding, as in ArchUnit.
//
// It parses the component subset ArchUnit reads, by hand: no Go library
// parses PlantUML, and the subset is a few line shapes. Other lines, such as
// notes and skinparams, are skipped.
package plantuml

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Line shapes of a component diagram.
var (
	// component matches [Name], component "Name" or package "Name", with
	// stereotypes and an alias: [Orders] <<pkg/...>> as orders.
	component = regexp.MustCompile(`^(?:\[([^\]]+)\]|(?:component|package|node)\s+` +
		`(?:"([^"]+)"|(\S+)))\s*((?:<<[^>]+>>\s*)*)(?:as\s+(\S+))?`)
	stereotype = regexp.MustCompile(`<<([^>]+)>>`)
	// arrow matches A --> B, A ..> B, A -[#red]-> B, and A <-- B.
	arrow = regexp.MustCompile(`^(\[[^\]]+\]|"[^"]+"|\w+)\s*(<?)[-.]+(?:\[[^\]]*\])?[-.]*(>?)\s*` +
		`(\[[^\]]+\]|"[^"]+"|\w+)`)
)

// Diagram is the components a diagram draws and the dependencies between
// them, by name.
type Diagram struct {
	Components []Component
	// Uses maps a component to the components its arrows point at.
	Uses map[string][]string
}

// Component is a box with the package patterns its stereotypes name.
type Component struct {
	Name     string
	Packages []string
}

// parser holds a diagram while it is read: the arrows wait until every
// component and alias is known.
type parser struct {
	d       Diagram
	aliases map[string]string
	arrows  [][2]string
}

// Parse reads a PlantUML component diagram. An arrow to a component the
// diagram does not declare, and a component with no package stereotype, are
// EINVALID, naming the line.
func Parse(r io.Reader) (Diagram, error) {
	p := parser{d: Diagram{Uses: map[string][]string{}}, aliases: map[string]string{}}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		if err := p.line(n, strings.TrimSpace(sc.Text())); err != nil {
			return Diagram{}, err
		}
	}
	if err := sc.Err(); err != nil {
		return Diagram{}, &arch.Error{Code: arch.EINVALID, Message: "read diagram: " + err.Error()}
	}
	return p.d, p.resolveArrows()
}

// line reads one line of the diagram: an arrow, a component, or anything
// else, which is skipped.
func (p *parser) line(n int, line string) error {
	if m := arrow.FindStringSubmatch(line); len(m) == 5 && (m[2] != "" || m[3] != "") {
		from, to := unquote(m[1]), unquote(m[4])
		if m[2] != "" {
			from, to = to, from
		}
		p.arrows = append(p.arrows, [2]string{from, to})
		return nil
	}
	m := component.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	c := Component{Name: m[1] + m[2] + m[3]}
	for _, s := range stereotype.FindAllStringSubmatch(m[4], -1) {
		c.Packages = append(c.Packages, strings.TrimSpace(s[1]))
	}
	if len(c.Packages) == 0 {
		return invalid(n, "component %q names no package in a <<stereotype>>", c.Name)
	}
	p.aliases[c.Name] = c.Name
	if m[5] != "" {
		p.aliases[m[5]] = c.Name
	}
	p.d.Components = append(p.d.Components, c)
	return nil
}

// resolveArrows records each arrow between the components it names.
func (p *parser) resolveArrows() error {
	for _, a := range p.arrows {
		from, okFrom := p.aliases[a[0]]
		to, okTo := p.aliases[a[1]]
		if !okFrom || !okTo {
			return invalid(0, "an arrow joins %q and %q, which are not both components", a[0], a[1])
		}
		p.d.Uses[from] = append(p.d.Uses[from], to)
	}
	return nil
}

// Model returns the model the diagram stands for: a subsystem per
// component, an isolation per component pattern allowing the component's
// own packages and those of the components its arrows point at, and
// fitness.requireAssigned, so a package in no component is a finding. The
// source names the diagram in each isolation's reason.
func (d *Diagram) Model(source string) arch.Model {
	m := arch.Model{Version: 1, Fitness: arch.Fitness{RequireAssigned: true}}
	packages := map[string][]string{}
	for _, c := range d.Components {
		packages[c.Name] = c.Packages
		m.Subsystems = append(m.Subsystems, arch.Subsystem{Name: c.Name, Packages: c.Packages})
	}
	for _, c := range d.Components {
		allowed := slices.Clone(c.Packages)
		uses := slices.Sorted(slices.Values(d.Uses[c.Name]))
		for _, to := range slices.Compact(uses) {
			allowed = append(allowed, packages[to]...)
		}
		for _, pat := range c.Packages {
			m.Isolations = append(m.Isolations, arch.Isolation{
				From: pat, MayImport: allowed,
				Reason: fmt.Sprintf("%s draws what %s may depend on", source, c.Name),
			})
		}
	}
	return m
}

// unquote strips a name's brackets or quotes.
func unquote(name string) string {
	return strings.Trim(name, `[]"`)
}

// invalid is an EINVALID error about the diagram, at line n when n > 0.
func invalid(n int, format string, args ...any) error {
	at := "diagram"
	if n > 0 {
		at = fmt.Sprintf("diagram line %d", n)
	}
	return &arch.Error{Code: arch.EINVALID, Message: at + ": " + fmt.Sprintf(format, args...)}
}
