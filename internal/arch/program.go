package arch

import (
	"cmp"
	"slices"
	"strings"
)

// Type kinds. Only struct types have an LCOM4; interfaces count as abstract.
const (
	KindStruct    TypeKind = "struct"
	KindInterface TypeKind = "interface"
	KindOther     TypeKind = "other"
)

// Reference kinds: where in the referring declaration a reference is, which
// says how hard it is to cut, hardest first.
const (
	// RefEmbed is an embedded field or interface: the type is part of the
	// referrer's own method set.
	RefEmbed RefKind = "embed"
	// RefField is the type of a named struct field: the referrer's layout.
	RefField RefKind = "field"
	// RefDefinition is the rest of a type declaration, such as the element
	// of a slice type or a type parameter's constraint.
	RefDefinition RefKind = "definition"
	// RefSignature is a method's receiver, parameters and results, or an
	// interface method's: the referrer's API.
	RefSignature RefKind = "signature"
	// RefBody is a method body: an implementation detail, the easiest to
	// move behind an interface.
	RefBody RefKind = "body"
)

// Reasons a source file of an analyzed package is left out of every measure.
const (
	ExcludedGenerated ExclusionReason = "generated"
	ExcludedByBuild   ExclusionReason = "build-constraints"
	// ExcludedOtherModule marks the go.mod of a module below the analysis
	// that its patterns' "..." does not cross into.
	ExcludedOtherModule ExclusionReason = "other-module"
)

// InactiveEvidence is the evidence of a finding about an inactive import.
const InactiveEvidence = "the import is in a file the build constraints exclude from the " +
	"build this report reads; it is read from the file's source"

// ExclusionReason says why no measure reads a source file.
type ExclusionReason string

// TypeKind classifies a named type by its underlying type.
type TypeKind string

// Position is a place in the source: a file path relative to the analysis
// root, and a 1-based line.
type Position struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Span is a range of whole lines in one file, both ends inclusive.
type Span struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Input is everything a measure reads: the program, the target architecture,
// the reviewers' verdicts, and the inputs a report records. The shell builds
// it; every measure reads it and none modifies it.
type Input struct {
	Program  Program
	Model    Model
	Verdicts []Verdict
	Inputs   Inputs
	// Domains are the domain models the model's fitness.domains rules name,
	// in the rules' order.
	Domains []DomainModel
	// Changed is the change set check --new-from-rev measures, nil without
	// one; only the rules on changes read it.
	Changed ChangedLines
}

// Program is everything a loader extracts from the packages under analysis.
// The slices are sorted by ID or path; [Analyze] relies on it, and a loader
// must guarantee it.
type Program struct {
	// Packages are the packages under analysis, the "in-scope" packages.
	Packages []Package `json:"packages"`
	// Files are their non-test, non-generated source files, sorted by path.
	Files []File `json:"files"`
	// Types are the package-level named types of those packages.
	Types []Type `json:"types"`
	// Funcs are their functions and methods, with function literals folded
	// into the declaration that contains them.
	Funcs []Func `json:"funcs"`
	// Refs are references from one in-scope type to another.
	Refs []Ref `json:"refs"`
	// Imports are every import of an in-scope package, including imports of
	// packages outside the scope, which only the forbidden-import rule reads.
	Imports []Import `json:"imports"`
	// InactiveImports are the imports of the files the build constraints
	// exclude, read from their source, for the package-to-package imports
	// the build does not have, sorted like Imports. Only the import rules
	// read them, through RuleImports: the measures measure the build.
	InactiveImports []Import `json:"inactiveImports"`
	// Injections are the calls that hand a concrete in-scope type to a
	// parameter of interface type, sorted.
	Injections []Injection `json:"injections"`
	// Facts are the connascence and Balanced Coupling facts, filled only
	// when the loader is asked for them.
	Facts Facts `json:"facts"`
	// Excluded lists the source files of the packages under analysis that
	// no measure reads, sorted by path: what the report did not see.
	Excluded []ExcludedFile `json:"excluded"`
}

// Package is one Go package under analysis.
type Package struct {
	Path string `json:"path"`
	// Name is the package name; "main" marks a binary.
	Name string `json:"name"`
	// Module is the path of the Go module the package belongs to.
	Module string `json:"module"`
	// Files are the package's non-test source files, relative to the root.
	Files []string `json:"files"`
	// Lines is the package's lines of code (mmi.md §7.3).
	Lines int `json:"lines"`
}

// ExcludedFile is a source file of an analyzed package that no measure
// reads, and why, or the go.mod of a module below the analysis that it does
// not read, with the module's path as Package. Test files are left out by
// design and are not listed.
type ExcludedFile struct {
	// Path is relative to the analysis root.
	Path    string          `json:"path"`
	Package string          `json:"package"`
	Reason  ExclusionReason `json:"reason"`
}

// File is one source file under analysis.
type File struct {
	// Path is relative to the analysis root.
	Path    string `json:"path"`
	Package string `json:"package"`
	// Lines is the file's lines of code (mmi.md §7.3).
	Lines int `json:"lines"`
}

// Type is a package-level named type: the Go counterpart of the
// dissertation's class (mmi.md §3).
type Type struct {
	// ID is the package path, a dot, and the type name.
	ID      string   `json:"id"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Kind    TypeKind `json:"kind"`
	Span    Span     `json:"span"`
	// Lines is the lines of code of the declaration plus every method with
	// this type as receiver (mmi.md §7.2, criterion 1.3.1).
	Lines int `json:"lines"`
	// Fields are the names of a struct type's fields, in declaration order.
	Fields []string `json:"fields,omitempty"`
	// Embeds lists the in-scope types it embeds, sorted: the marker types of
	// MMI 3.3.
	Embeds []string `json:"embeds,omitempty"`
	// Implements lists the in-scope interfaces with methods that the type or
	// a pointer to it implements, sorted.
	Implements []string `json:"implements,omitempty"`
	// Directive is the role an "//arch:role NAME" comment gives the type.
	Directive string `json:"directive,omitempty"`
}

// Func is a function or method declaration.
type Func struct {
	// ID is the package path, a dot, then "Recv." for a method, then the name.
	ID      string `json:"id"`
	Package string `json:"package"`
	// Receiver is the ID of a method's receiver type, or "" for a function.
	Receiver string `json:"receiver,omitempty"`
	Name     string `json:"name"`
	Span     Span   `json:"span"`
	// Lines is the function's lines of code.
	Lines int `json:"lines"`
	// Statements is von Zitzewitz's Number of Statements.
	Statements int `json:"statements"`
	// Complexity holds the three cyclomatic-complexity variants.
	Complexity Complexity `json:"complexity"`
	// MaxNesting is the deepest block nesting inside the body.
	MaxNesting int `json:"maxNesting"`
	// Cognitive is SonarSource's cognitive complexity, as
	// github.com/uudashr/gocognit computes it: a decision costs more the
	// deeper it is nested, and a run of one boolean operator costs one.
	Cognitive int `json:"cognitive"`
	// Uses names the receiver's fields and methods that a method's body
	// selects through the receiver variable, for LCOM4.
	Uses []string `json:"uses,omitempty"`
	// BodyHash identifies the body's shape: a hash of its syntax with every
	// identifier and literal replaced by a placeholder, so two bodies that
	// differ only in names and values hash alike. It is set only with facts,
	// for bodies of at least ten statements (connascence.md §2.5).
	BodyHash string `json:"bodyHash,omitempty"`
}

// Complexity holds the cyclomatic-complexity variants of mmi.md §7.3.
type Complexity struct {
	// Plain is McCabe's count: 1 plus one per if, for, range, non-default
	// case clause and non-default select clause.
	Plain int `json:"plain"`
	// Modified counts a switch or select once instead of once per clause.
	Modified int `json:"modified"`
	// Extended is Plain plus one per && and ||, which gocyclo computes.
	Extended int `json:"extended"`
}

// RefKind says where in the referring declaration a type reference is.
type RefKind string

// Ref is a reference from one in-scope type to another: the dissertation's use
// relationship (mmi.md §3). Weight counts the referring identifiers; Pos is
// the first of them. Kinds lists where the identifiers are, hardest to cut
// first, as [RefKind.Order] orders them.
type Ref struct {
	From   string    `json:"from"`
	To     string    `json:"to"`
	Weight int       `json:"weight"`
	Pos    Position  `json:"pos"`
	Kinds  []RefKind `json:"kinds,omitempty"`
}

// Injection is a call that passes a value of a concrete type to a parameter
// of interface type: at run time, the callee's package depends on the
// concrete type's package although it never imports it.
type Injection struct {
	// Caller is the package the call is in; Callee is the called function.
	Caller        string `json:"caller"`
	Callee        string `json:"callee"`
	CalleePackage string `json:"calleePackage"`
	// Concrete is the ID of the type passed in.
	Concrete string   `json:"concrete"`
	Pos      Position `json:"pos"`
}

// Import is one import declaration of an in-scope package.
type Import struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Pos  Position `json:"pos"`
	// Inactive marks an import of a file the build constraints exclude,
	// read from its source rather than type-checked.
	Inactive bool `json:"inactive,omitempty"`
}

// RuleImports returns the imports the import rules check: the build's, then
// the inactive ones, so a forbidden import in a file of another platform is
// found on every platform.
//
// Ensures: a new slice; Imports' order, then InactiveImports'.
func (p *Program) RuleImports() []Import {
	return slices.Concat(p.Imports, p.InactiveImports)
}

// IsStandard reports whether an import path is in the standard library: its
// first element has no dot, the rule cmd/go applies.
func IsStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// Order is a kind's place among the kinds, from 0 for the hardest to cut,
// an embedding, to 4 for the easiest, a method body.
func (k RefKind) Order() int {
	switch k {
	case RefEmbed:
		return 0
	case RefField:
		return 1
	case RefDefinition:
		return 2
	case RefSignature:
		return 3
	default:
		return 4
	}
}

// ComparePositions orders positions by file, then line.
func ComparePositions(a, b Position) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
}
