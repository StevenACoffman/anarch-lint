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
	// RefFieldWrite is an assignment to one of the type's fields in a method
	// body, or a keyed composite literal setting one: the body depends on the
	// type's layout and changes its state.
	RefFieldWrite RefKind = "fieldWrite"
	// RefFieldRead is a read of one of the type's fields in a method body.
	RefFieldRead RefKind = "fieldRead"
	// RefLiteral is a composite literal of the type in a method body.
	RefLiteral RefKind = "literal"
	// RefConversion is a conversion to the type in a method body.
	RefConversion RefKind = "conversion"
	// RefCall is a call of one of the type's methods, or a method value, in a
	// method body.
	RefCall RefKind = "call"
	// RefTypeArgument is the type as a type argument of an instantiation in
	// a method body.
	RefTypeArgument RefKind = "typeArgument"
	// RefBody is any other use in a method body, such as a variable's type:
	// an implementation detail, the easiest to move behind an interface.
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

// How an edge a finding rests on was known (IMPROVEMENTS.md §3.2).
const (
	// ProvenanceTypes is an edge the type checker resolved: the build's
	// imports and every reference.
	ProvenanceTypes Provenance = "types"
	// ProvenanceSource is an import read from a file's source without
	// type-checking, because the build constraints exclude the file.
	ProvenanceSource Provenance = "source"
	// ProvenanceInjected is a run-time dependency: a value of a concrete
	// type passed in through an interface parameter at a call.
	ProvenanceInjected Provenance = "injected"
	// ProvenanceDispatch is a run-time dependency a VTA call graph finds: a
	// dynamic call that reaches a concrete method of another package.
	ProvenanceDispatch Provenance = "dispatch"
)

// Provenance says how an edge a finding rests on was known, so a reader can
// tell a resolved dependency from one read without types or inferred.
type Provenance string

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
	// APIs are the OpenAPI documents the model's fitness.openapi rules
	// name, in the rules' order.
	APIs []APIDocument
	// GraphQL are the GraphQL schemas the model's fitness.graphql rules
	// name, with their operation documents checked, in the rules' order.
	GraphQL []GraphQLSchema
	// Protos are the .proto files the model's fitness.protobuf rules name,
	// in the rules' order.
	Protos []ProtoFiles
	// Changed is the change set check --new-from-rev measures, nil without
	// one; only the rules on changes read it.
	Changed ChangedLines
	// Coverage is a test coverage profile attributed to the program's
	// functions, nil without one; balance ranks packages by it.
	Coverage *Coverage
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
	// TestImports are the imports of the packages' test files, internal
	// and external, read from their source and sorted like Imports. Only the
	// import rules that opt in with tests read them.
	TestImports []Import `json:"testImports"`
	// Enforcers are the functions and tests an "//arch:invariant ID"
	// comment marks, sorted by invariant, function and position.
	Enforcers []Enforcer `json:"enforcers"`
	// Injections are the calls that hand a concrete in-scope type to a
	// parameter of interface type, sorted.
	Injections []Injection `json:"injections"`
	// Dispatches are the run-time wiring a VTA call graph finds, sorted;
	// loaded only when a forbidden-import rule asks for injected
	// dependencies.
	Dispatches []Dispatch `json:"dispatches,omitempty"`
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
	// Subsystem is what an "//arch:subsystem NAME" directive in a file's
	// package comment names, and Exposes marks an "//arch:exposes": the
	// package declares its own subsystem, and that it is part of the
	// subsystem's interface, in the code (ArchUnit's modules declared by
	// annotation).
	Subsystem string `json:"subsystem,omitempty"`
	Exposes   bool   `json:"exposes,omitempty"`
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
	// WireNames are the names a struct's tags give its fields in each
	// encoding, as "json:id", sorted: the wire format it agrees on.
	WireNames []string `json:"wireNames,omitempty"`
	// Embeds lists the in-scope types it embeds, sorted: the marker types of
	// MMI 3.3.
	Embeds []string `json:"embeds,omitempty"`
	// Implements lists the in-scope interfaces with methods that the type or
	// a pointer to it implements, sorted.
	Implements []string `json:"implements,omitempty"`
	// Directive is the role an "//arch:role NAME" comment gives the type.
	Directive string `json:"directive,omitempty"`
	// Entity is the domain-model entity an "//arch:entity NAME" comment
	// says the type expresses.
	Entity string `json:"entity,omitempty"`
}

// Enforcer is a function or test that an "//arch:invariant ID" comment says
// enforces a domain model's invariant: where a person who knows the code
// says it keeps the invariant (docs/adr/0016).
type Enforcer struct {
	Invariant string `json:"invariant"`
	// Func is the function's ID; a test's is built the same way from its
	// package and its name.
	Func string `json:"func"`
	// Package is the enforcer's package; an external test package counts
	// as the package it tests.
	Package string   `json:"package"`
	Pos     Position `json:"pos"`
	// Test marks a function of a test file, read from its source, since
	// test files are not loaded.
	Test bool `json:"test,omitempty"`
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
	// ExactHash identifies the body itself: a hash of it as go/format prints
	// it, without its comments and blank lines, so two bodies hash alike only
	// when they are copies. It is set for the bodies BodyHash covers.
	ExactHash string `json:"exactHash,omitempty"`
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

// Dispatch is a dynamic call, through an interface or a function value, that
// a VTA call graph resolves to a method of a concrete type in another
// package under analysis: run-time wiring, which the composition root sets
// up and no import shows (mmi.md §7.2).
type Dispatch struct {
	// Caller is the function the call is in, CallerPackage its package.
	Caller        string `json:"caller"`
	CallerPackage string `json:"callerPackage"`
	// Callee is the concrete method called; Concrete the type it belongs to.
	Callee   string   `json:"callee"`
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
	// Test marks an import of a test file, read from its source.
	Test bool `json:"test,omitempty"`
}

// Provenance returns how the import was known: from source for an inactive
// import or a test's, by the type checker otherwise.
func (imp *Import) Provenance() Provenance {
	if imp.Inactive || imp.Test {
		return ProvenanceSource
	}
	return ProvenanceTypes
}

// objectID is the object of an import rule's finding about the import: the
// imported package, marked when a test imports it, so the finding about a
// test and the one about the package itself keep apart.
func (imp *Import) objectID() string {
	if imp.Test {
		return imp.To + " (test)"
	}
	return imp.To
}

// inTest is " in a test" for an import of a test file, and "" otherwise.
func (imp *Import) inTest() string {
	if imp.Test {
		return " in a test"
	}
	return ""
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
// an embedding, to the easiest, a plain use in a method body.
func (k RefKind) Order() int {
	if i := slices.Index(refKinds(), k); i >= 0 {
		return i
	}
	return len(refKinds())
}

// refKinds lists the kinds, hardest to cut first: what is part of the
// referrer's layout or API, then what a body does with the type, from
// changing its state to merely naming it.
func refKinds() []RefKind {
	return []RefKind{
		RefEmbed, RefField, RefDefinition, RefSignature, RefFieldWrite, RefFieldRead,
		RefLiteral, RefConversion, RefCall, RefTypeArgument, RefBody,
	}
}

// CompareSites orders sites by every field: file, line, package, function
// and type.
func CompareSites(a, b Site) int {
	return cmp.Or(
		cmp.Compare(a.File, b.File),
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(a.Package, b.Package),
		cmp.Compare(a.Func, b.Func),
		cmp.Compare(a.Type, b.Type),
	)
}

// ComparePositions orders positions by file, then line.
func ComparePositions(a, b Position) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
}
