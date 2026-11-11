# A Pass Says What It Could Not See

Each fitness rule's result counts, as `unseen`, what its check cannot see
through in the packages whose code it reads: a `//go:linkname`, an import of
`plugin` or `C`, reflection by name, an `unsafe` conversion and, for a
`calls` rule with `dispatch: true`, a dynamic call the VTA call graph
resolves to nothing. Such a rule also reads the callees its dynamic calls
may reach, and one it forbids is a `possible-forbidden-call` or
`possible-call-restriction` warning. Neither ever fails a rule.

Pinned by `TestADR_0019_APassSaysWhatItCouldNotSee` in `internal/fitness`.

## Context

A rule's checks read static facts: the imports, the calls the type checker
resolves, the field uses. A pass then means that nothing the check reads
breaks the rule, not that nothing does. `simplicity.md` rates three rules
(BND 2.1, FCIS 1.4, SME 2.1) only partly checked because I/O through an
interface escapes the `calls` rules: a core function that writes to the
`io.Writer` it is given calls `(io.Writer).Write`, and only an open list of
interface methods catches it. The survey behind IMPROVEMENTS.md §10.1 and
§10.3 pairs an over-approximation with an under-approximation, so what lies
between them is a counted unknown, and treats each construct an analysis
does not model as a defeater a reviewer can eliminate.

## Decision

**What a call will do and what it may do stay apart.** Static calls are
facts, and a forbidden one fails its rule as before. The callees a VTA call
graph gives a dynamic call are possibilities: VTA follows the types that
flow into an interface value or a function value, so `os.Stdout` passed to
a core function's `io.Writer` makes `(*os.File).Write` a callee of its
`w.Write`. But it over-approximates, so each is a warning carrying the
`dispatch` provenance, for a person to confirm. A call site of the
`injected` kind, recorded where the shell passes the value in, would see
only a subset of what VTA follows, so it is not added.

**The unknowns are counted, not guessed.** A dynamic call that VTA resolves
to nothing, such as a call on a value a library made, is unseen, beside the
constructs no check models. Each rule counts only what can hide what its own
kind reads, from one table: a `//go:linkname` or a plugin hides a dependency from
every kind; reflection hides a call, a field access or a use; an unsafe
conversion a field access or a use; cgo a call. An empty rule, or one turned
off, checked nothing, so it counts nothing.

**Only a rule that asks pays.** The whole-program SSA build happens when a
`calls` rule sets `dispatch` or a forbidden import sets `injected`, and one
graph serves both.

## Consequences

A pass reads "passed, 17 unseen", and the report's `unseen` lists where each
construct is. Library bodies are built from types alone, so a dynamic call
inside one, such as `fmt.Fprintf` calling `w.Write`, stays invisible and
still needs the open list of callees. The self-check opts its core rule in.
