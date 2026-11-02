package pipeline

// Options say where a measurement reads its inputs. The zero value reads the
// current directory, with the model anarch.json there if it exists.
type Options struct {
	// Dir is the directory to analyze, and the root of every reported path.
	Dir string
	// ModelPath is the target-architecture model; empty means anarch.json in
	// Dir if it exists, and an empty model if it does not.
	ModelPath string
	// JudgementsPath optionally supplies verdicts for the judgement slots.
	JudgementsPath string
	// CodeownersPath names the CODEOWNERS file for Balanced Coupling's
	// owner boundary; empty means the first of .github/CODEOWNERS,
	// CODEOWNERS and docs/CODEOWNERS in Dir, if any.
	CodeownersPath string
	// Tags are build tags.
	Tags []string
}
