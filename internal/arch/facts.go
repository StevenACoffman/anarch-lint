package arch

// Object kinds of a [Use].
const (
	ObjectType   ObjectKind = "type"
	ObjectFunc   ObjectKind = "func"
	ObjectMethod ObjectKind = "method"
	ObjectVar    ObjectKind = "var"
	ObjectConst  ObjectKind = "const"
	ObjectField  ObjectKind = "field"
)

// Contexts a [ConstExpr] can appear in.
const (
	ContextCompare  ConstContext = "compare"
	ContextCase     ConstContext = "case"
	ContextArgument ConstContext = "argument"
)

// ObjectKind classifies the declared object a [Use] refers to.
type ObjectKind string

// ConstContext says where a constant-valued expression is used.
type ConstContext string

// Site is a place in the source with the declarations around it, so the core
// can tell how far apart two sites lie without re-reading the code
// (connascence.md §1.2).
type Site struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Package string `json:"package"`
	// Func is the ID of the enclosing function or method, "" at package
	// level. Function literals belong to the declaration that holds them.
	Func string `json:"func,omitempty"`
	// Type is the ID of the enclosing method's receiver type, or of the type
	// whose declaration holds the site; "" otherwise.
	Type string `json:"type,omitempty"`
}

// Facts are the facts the connascence and Balanced Coupling measures need
// beyond a [Program]'s own (connascence.md §4, §9). A loader fills them only
// when asked; each slice is sorted by site and then by every other field.
type Facts struct {
	Uses           []Use           `json:"uses"`
	Signatures     []Signature     `json:"signatures"`
	ConstExprs     []ConstExpr     `json:"constExprs"`
	Constants      []Constant      `json:"constants"`
	NilNils        []NilNil        `json:"nilNils"`
	UnkeyedLits    []UnkeyedLit    `json:"unkeyedLits"`
	ConstIndexes   []ConstIndex    `json:"constIndexes"`
	Switches       []Switch        `json:"switches"`
	GlobalUses     []GlobalUse     `json:"globalUses"`
	PointerEqs     []PointerEq     `json:"pointerEqs"`
	AlgoCalls      []AlgoCall      `json:"algoCalls"`
	TimeCalls      []TimeCall      `json:"timeCalls"`
	Guards         []FieldAccess   `json:"guards"`
	FieldWrites    []FieldAccess   `json:"fieldWrites"`
	LockCalls      []LockCall      `json:"lockCalls"`
	Linknames      []Linkname      `json:"linknames"`
	UnsafeConvs    []UnsafeConv    `json:"unsafeConvs"`
	ReflectByNames []ReflectByName `json:"reflectByNames"`
}

// Use is one function's references to one declared in-scope object that is
// declared outside it: the raw material of connascence of name (§2.1). Count
// is the number of referring identifiers; Site is the first of them.
type Use struct {
	// Object is the object's ID: "pkg.Name" for a package-level object,
	// "pkg.Type.Name" for a method or a field.
	Object   string     `json:"object"`
	Kind     ObjectKind `json:"kind"`
	Package  string     `json:"package"`
	Exported bool       `json:"exported"`
	// Owner is the type a method or field belongs to, "" otherwise.
	Owner string `json:"owner,omitempty"`
	Count int    `json:"count"`
	Site  Site   `json:"site"`
}

// TypeRef is a type as it appears in a signature.
type TypeRef struct {
	// Type is the type's full spelling, with package paths.
	Type string `json:"type"`
	// Named is the ID of the named type at its core, after pointers,
	// slices, arrays and map values are removed, and Package declares it;
	// both are "" for a basic or unnamed type.
	Named   string `json:"named,omitempty"`
	Package string `json:"package,omitempty"`
	// Basic reports an unnamed basic type such as string or int.
	Basic bool `json:"basic"`
}

// Signature is a function's or method's parameter and result types.
type Signature struct {
	Func     string    `json:"func"`
	Exported bool      `json:"exported"`
	Params   []TypeRef `json:"params"`
	Results  []TypeRef `json:"results"`
	Variadic bool      `json:"variadic"`
	Site     Site      `json:"site"`
}

// ConstExpr is a constant-valued expression used in a comparison, as a case,
// or as a call argument.
type ConstExpr struct {
	// Type is the expression's type, the default type for an untyped one.
	Type string `json:"type"`
	// Value is the go/constant exact value, so 0x10 and 16 are one value.
	Value string `json:"value"`
	// Literal reports a value written as a literal rather than a name.
	Literal bool         `json:"literal"`
	Context ConstContext `json:"context"`
	// Callee is the called function's full name, for an argument.
	Callee string `json:"callee,omitempty"`
	Arg    int    `json:"arg"`
	Site   Site   `json:"site"`
}

// Constant is a package-level constant.
type Constant struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Exported bool   `json:"exported"`
	// Alias reports a constant written as another constant's name, such as
	// InitialState = Draft.
	Alias bool `json:"alias"`
	Site  Site `json:"site"`
}

// NilNil is a return of nil, nil from a function whose results are a
// nillable value and an error (connascence.md §2.3, detector 4).
type NilNil struct {
	Site Site `json:"site"`
}

// UnkeyedLit is a composite literal of a named struct type without field
// names, which depends on the field order (§2.4, detector 1).
type UnkeyedLit struct {
	Type    string `json:"type"`
	Package string `json:"package"`
	Site    Site   `json:"site"`
}

// ConstIndex is a constant index into a value that a call returned, directly
// or through a local assigned once (§2.4, detector 4).
type ConstIndex struct {
	// Origin is the full name of the function whose result is indexed.
	Origin string `json:"origin"`
	Index  string `json:"index"`
	Site   Site   `json:"site"`
}

// Switch is an expression switch on a named type with a basic underlying
// type (§3.3, detector 1).
type Switch struct {
	Type       string `json:"type"`
	Package    string `json:"package"`
	HasDefault bool   `json:"hasDefault"`
	Site       Site   `json:"site"`
}

// GlobalUse is a read or a write of an in-scope package-level variable.
type GlobalUse struct {
	Var      string `json:"var"`
	Package  string `json:"package"`
	Exported bool   `json:"exported"`
	Write    bool   `json:"write"`
	Site     Site   `json:"site"`
}

// PointerEq is an == or != between two pointers (§3.4, detector 3).
type PointerEq struct {
	// Type is the pointed-to named type, Package its package.
	Type    string `json:"type"`
	Package string `json:"package"`
	Site    Site   `json:"site"`
}

// AlgoCall is a call into a hash, checksum, encoding or encryption package of
// the standard library (§2.5, detector 3).
type AlgoCall struct {
	// Algorithm is the package, plus the concrete receiver type of a method:
	// "crypto/sha256", "encoding/base64.Encoding".
	Algorithm string `json:"algorithm"`
	Site      Site   `json:"site"`
}

// TimeCall is a call of a function of package time.
type TimeCall struct {
	Callee string `json:"callee"`
	Site   Site   `json:"site"`
}

// FieldAccess is a method's access to a field of its receiver: a guard that
// returns early on the field (§3.1, detector 1), or a write to it.
type FieldAccess struct {
	Type   string `json:"type"`
	Field  string `json:"field"`
	Method string `json:"method"`
	Site   Site   `json:"site"`
}

// LockCall is a call of Lock, Unlock, RLock or RUnlock on a sync.Mutex or
// sync.RWMutex.
type LockCall struct {
	// Expr is the receiver expression as written, such as "s.mu".
	Expr     string `json:"expr"`
	Method   string `json:"method"`
	Deferred bool   `json:"deferred"`
	Site     Site   `json:"site"`
}

// Linkname is a //go:linkname directive: access to another package's symbol
// that bypasses its interface, intrusive coupling (§9.2).
type Linkname struct {
	Local   string `json:"local"`
	Target  string `json:"target"`
	Package string `json:"package"`
	Site    Site   `json:"site"`
}

// UnsafeConv is a conversion between unsafe.Pointer and a pointer to a named
// type (§9.2).
type UnsafeConv struct {
	Type    string `json:"type"`
	Package string `json:"package"`
	Site    Site   `json:"site"`
}

// ReflectByName is a reflect lookup of a field or method by name (§9.2).
type ReflectByName struct {
	Method string `json:"method"`
	// Name is the constant name looked up, "" when it is not a constant.
	Name string `json:"name,omitempty"`
	Site Site   `json:"site"`
}

// Churn is the change history a revision range shows: for each file, the
// commits that changed it. It is evidence for a volatility judgement, never
// volatility itself, since frequent change can be accidental (connascence.md
// §9.4). Since and Head are the resolved commits that bound the range, so two
// reports compare only when both agree.
type Churn struct {
	Since string `json:"since"`
	Head  string `json:"head"`
	// Commits maps a file, relative to the analysis root, to the sorted
	// hashes of the commits in the range that changed it.
	Commits map[string][]string `json:"commits"`
}
