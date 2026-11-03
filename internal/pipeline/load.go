// Package pipeline is the imperative shell every measurement shares: it reads
// the model, the judgements, the domain models, the baseline and the
// waivers, loads the packages, and runs the measures a check gates on. The
// CLI's commands and the anarchtest package both measure through it, so
// neither repeats the other's wiring; each maps its own configuration onto
// Options.
package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// Default file names, looked for in Dir when no option names a file.
const (
	defaultModel = "anarch.json"
	// DefaultBaseline is the baseline file baseline writes and check reads.
	DefaultBaseline = "anarch-baseline.json"
	// DefaultWaivers is the waivers file looked for in --dir when --waivers
	// is unset.
	DefaultWaivers = "anarch-waivers.json"
)

// The facts modes.
const (
	// NoFacts loads only what the MMI reads.
	NoFacts FactsMode = iota
	// AllFacts loads the facts, for the connascence and balance commands.
	AllFacts
	// GatedFacts loads them when the model opts a measure that reads them
	// into the gate, for baseline and check.
	GatedFacts
)

// FactsMode says when Load extracts the connascence and Balanced Coupling
// facts.
type FactsMode int

// Load reads the model and judgements and loads the packages, with the facts
// as the mode asks. A missing default model is not an error but an empty model, whose
// model-dependent measures report themselves unmeasured; a file named by a
// flag must exist.
func (o *Options) Load(
	ctx context.Context,
	patterns []string,
	facts FactsMode,
) (arch.Input, error) {
	const op = "pipeline.Options.Load"
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	in, err := o.readInputs(ctx)
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	m := &in.Model
	gated := m.Connascence.Enabled() || m.Balance.Enabled() || m.Fitness.Enabled()
	if in.Program, err = gopackages.Load(ctx, gopackages.Config{
		Dir: o.Dir, Patterns: patterns, Tags: o.Tags,
		Facts: facts == AllFacts || (facts == GatedFacts && gated),
		Dispatches: slices.ContainsFunc(m.ForbiddenImports,
			func(fi arch.ForbiddenImport) bool { return fi.Injected }),
	}); err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	paths := make([]string, len(in.Program.Packages))
	for i, pkg := range in.Program.Packages {
		paths[i] = pkg.Path
	}
	in.Model = in.Model.Expand(paths)
	in.Model = in.Model.Declare(in.Program.Packages)
	platform, err := gopackages.Platform(ctx, o.Dir, nil)
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	in.Inputs = arch.Inputs{
		Tool:      "anarch-lint " + ToolVersion(),
		GoVersion: runtime.Version(),
		Tags:      slices.Sorted(slices.Values(o.Tags)),
		Platform:  platform,
		Patterns:  patterns,
	}
	return in, nil
}

// readInputs reads the files a measurement reads besides the code: the
// model, the judgements, the domain models and the contract files.
func (o *Options) readInputs(ctx context.Context) (arch.Input, error) {
	model, err := o.readModel()
	if err != nil {
		return arch.Input{}, err
	}
	verdicts, err := o.readVerdicts()
	if err != nil {
		return arch.Input{}, err
	}
	domains, err := o.readDomains(&model)
	if err != nil {
		return arch.Input{}, err
	}
	contracts, err := o.readContractFiles(ctx, &model)
	if err != nil {
		return arch.Input{}, err
	}
	return arch.Input{
		Model: model, Verdicts: verdicts, Domains: domains,
		APIs: contracts.apis, GraphQL: contracts.graphql, Protos: contracts.protos,
	}, nil
}

// Analyze loads the packages and measures them for the MMI report.
func (o *Options) Analyze(ctx context.Context, patterns []string) (arch.Report, error) {
	l, err := o.Load(ctx, patterns, NoFacts)
	if err != nil {
		return arch.Report{}, err
	}
	return arch.Analyze(&l), nil
}

// readModel reads the model file, or returns an empty model when the default
// file does not exist.
func (o *Options) readModel() (arch.Model, error) {
	const op = "pipeline.Options.readModel"
	path, required := o.ModelFile(), o.ModelPath != ""
	src, err := ReadFile(path)
	if !required && arch.ErrorCode(err) == arch.ENOTFOUND {
		return arch.Model{Version: 1}, nil
	}
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op, Err: err}
	}
	m, err := DecodeModelFile(path, src)
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op + " " + path, Err: err}
	}
	return m, nil
}

// DecodeModelFile decodes a model file's contents: JSON, or YAML when the
// file is named *.yaml or *.yml, which is read as the JSON it stands for, so
// both spellings of a model mean, validate and hash alike.
func DecodeModelFile(path string, src []byte) (arch.Model, error) {
	if ext := filepath.Ext(path); ext == ".yaml" || ext == ".yml" {
		js, err := yaml.YAMLToJSON(src)
		if err != nil {
			return arch.Model{}, &arch.Error{
				Op:  "pipeline.DecodeModelFile",
				Err: arch.InvalidModel(arch.ProblemMalformed, "not YAML: %v", err),
			}
		}
		src = js
	}
	m, err := arch.DecodeModel(bytes.NewReader(src))
	if err != nil {
		return arch.Model{}, &arch.Error{Op: "pipeline.DecodeModelFile", Err: err}
	}
	return m, nil
}

// ModelFile is the model file ModelPath names; or the first of anarch.json,
// anarch.yaml and anarch.yml in Dir that exists, anarch.json when none does.
func (o *Options) ModelFile() string {
	if o.ModelPath != "" {
		return o.ModelPath
	}
	for _, name := range []string{defaultModel, "anarch.yaml", "anarch.yml"} {
		if _, err := os.Stat(filepath.Join(o.Dir, name)); err == nil {
			return filepath.Join(o.Dir, name)
		}
	}
	return filepath.Join(o.Dir, defaultModel)
}

// readVerdicts reads the judgements file, if one is named.
func (o *Options) readVerdicts() ([]arch.Verdict, error) {
	const op = "pipeline.Options.readVerdicts"
	if o.JudgementsPath == "" {
		return nil, nil
	}
	src, err := ReadFile(o.JudgementsPath)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	vs, err := arch.DecodeJudgements(bytes.NewReader(src))
	if err != nil {
		return nil, &arch.Error{Op: op + " " + o.JudgementsPath, Err: err}
	}
	return vs, nil
}

// ReadFile reads an input file, translating a missing file to
// ENOTFOUND and any other failure to EINVALID, so a command can report it
// without knowing about the os package's errors.
func ReadFile(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, &arch.Error{
			Code:    arch.ENOTFOUND,
			Message: fmt.Sprintf("%s does not exist", path),
		}
	case err != nil:
		return nil, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("read %s: %v", path, err)}
	}
	return src, nil
}

// ToolVersion is the module version this binary was built from, or "devel".
func ToolVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" &&
		bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "devel"
}
