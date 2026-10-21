package root

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
	"github.com/StevenACoffman/anarch-lint/internal/sarif"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// ErrGateFailed exits with the status of a check whose gate failed: distinct from
// 1, a run that could not measure, and 2, a usage error, so CI can tell an
// architecture regression from a broken pipeline.
const ErrGateFailed ExitError = 3

// Default file names, looked for in --dir when no flag names a file.
const (
	defaultModel = "anarch.json"
	// DefaultBaseline is the baseline file baseline writes and check reads.
	DefaultBaseline = "anarch-baseline.json"
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

// Load is the imperative shell every measuring command shares: it reads the
// model and judgements and loads the packages, with the facts as the mode
// asks. A missing default model is not an error but an empty model, whose
// model-dependent measures report themselves unmeasured; a file named by a
// flag must exist.
func (cfg *Config) Load(
	ctx context.Context,
	patterns []string,
	facts FactsMode,
) (arch.Input, error) {
	const op = "root.Config.Load"
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	model, err := cfg.readModel()
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	verdicts, err := cfg.readVerdicts()
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	domains, err := cfg.readDomains(&model)
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	gated := model.Connascence.Enabled() || model.Balance.Enabled() || model.Fitness.Enabled()
	prog, err := gopackages.Load(ctx, gopackages.Config{
		Dir: cfg.Dir, Patterns: patterns, Tags: cfg.Tags,
		Facts: facts == AllFacts || (facts == GatedFacts && gated),
	})
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	platform, err := gopackages.Platform(ctx, cfg.Dir, nil)
	if err != nil {
		return arch.Input{}, &arch.Error{Op: op, Err: err}
	}
	return arch.Input{
		Model:    model,
		Verdicts: verdicts,
		Domains:  domains,
		Program:  prog,
		Inputs: arch.Inputs{
			Tool:      "anarch-lint " + toolVersion(),
			GoVersion: runtime.Version(),
			Tags:      slices.Sorted(slices.Values(cfg.Tags)),
			Platform:  platform,
			Patterns:  patterns,
		},
	}, nil
}

// Analyze loads the packages and measures them for the MMI report.
func (cfg *Config) Analyze(ctx context.Context, patterns []string) (arch.Report, error) {
	l, err := cfg.Load(ctx, patterns, NoFacts)
	if err != nil {
		return arch.Report{}, err
	}
	return arch.Analyze(&l), nil
}

// WriteReport writes r to Stdout in the format --format selects.
func (cfg *Config) WriteReport(r *arch.Report) error {
	return cfg.Write(r, func(w io.Writer) error { return text.WriteReport(w, r) })
}

// Write writes v to Stdout: as JSON with --format json, and with writeText
// otherwise.
func (cfg *Config) Write(v any, writeText func(io.Writer) error) error {
	const op = "root.Config.Write"
	var err error
	switch cfg.Format {
	case "json":
		err = arch.WriteJSON(cfg.Stdout, v)
	case "sarif":
		err = &arch.Error{Code: arch.EINVALID, Message: "--format sarif is for check only"}
	default:
		err = writeText(cfg.Stdout)
	}
	if err != nil {
		return &arch.Error{Op: op, Err: err}
	}
	return nil
}

// readModel reads the model file, or returns an empty model when the default
// file does not exist.
func (cfg *Config) readModel() (arch.Model, error) {
	const op = "root.Config.readModel"
	path, required := cfg.ModelFile(), cfg.ModelPath != ""
	src, err := ReadFile(path)
	if !required && arch.ErrorCode(err) == arch.ENOTFOUND {
		return arch.Model{Version: 1}, nil
	}
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op, Err: err}
	}
	m, err := arch.DecodeModel(bytes.NewReader(src))
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op + " " + path, Err: err}
	}
	return m, nil
}

// SARIFOptions are what check's SARIF log records besides the result: the
// tool's version, and the model file relative to --dir.
func (cfg *Config) SARIFOptions() sarif.Options {
	model := filepath.ToSlash(cfg.ModelFile())
	if rel, err := filepath.Rel(
		cfg.Dir,
		cfg.ModelFile(),
	); err == nil &&
		!strings.HasPrefix(rel, "..") {
		model = filepath.ToSlash(rel)
	}
	return sarif.Options{Version: toolVersion(), ModelPath: model}
}

// ModelFile is the model file --model names, or anarch.json in --dir.
func (cfg *Config) ModelFile() string {
	if cfg.ModelPath != "" {
		return cfg.ModelPath
	}
	return filepath.Join(cfg.Dir, defaultModel)
}

// readVerdicts reads the judgements file, if one is named.
func (cfg *Config) readVerdicts() ([]arch.Verdict, error) {
	const op = "root.Config.readVerdicts"
	if cfg.JudgementsPath == "" {
		return nil, nil
	}
	src, err := ReadFile(cfg.JudgementsPath)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	vs, err := arch.DecodeJudgements(bytes.NewReader(src))
	if err != nil {
		return nil, &arch.Error{Op: op + " " + cfg.JudgementsPath, Err: err}
	}
	return vs, nil
}

// ReadFile reads an input file named by a flag, translating a missing file to
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

// toolVersion is the module version this binary was built from, or "devel".
func toolVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" &&
		bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "devel"
}
