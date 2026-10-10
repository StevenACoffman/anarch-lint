package arch

// Type kinds. Only struct types have an LCOM4; interfaces count as abstract.
const (
	KindStruct    TypeKind = "struct"
	KindInterface TypeKind = "interface"
	KindOther     TypeKind = "other"
)

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
	// Injections are the calls that hand a concrete in-scope type to a
	// parameter of interface type, sorted.
	Injections []Injection `json:"injections"`
}

// Package is one Go package under analysis.
type Package struct {
	Path string `json:"path"`
	// Files are the package's non-test source files, relative to the root.
	Files []string `json:"files"`
	// Lines is the package's lines of code (mmi.md §7.3).
	Lines int `json:"lines"`
	// Generated is the number of generated files left out of Files.
	Generated int `json:"generated"`
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
	// Uses names the receiver's fields and methods that a method's body
	// selects through the receiver variable, for LCOM4.
	Uses []string `json:"uses,omitempty"`
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

// Ref is a reference from one in-scope type to another: the dissertation's use
// relationship (mmi.md §3). Weight counts the referring identifiers; Pos is
// the first of them.
type Ref struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Weight int      `json:"weight"`
	Pos    Position `json:"pos"`
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
}
