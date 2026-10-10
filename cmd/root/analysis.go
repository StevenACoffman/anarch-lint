package root

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

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
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

// Analyze is the imperative shell every measuring command shares: it reads the
// model and judgements, loads the packages, and hands them to the pure core.
// A missing default model is not an error but an empty model, whose
// model-dependent criteria report themselves unmeasured; a file named by a
// flag must exist.
func (cfg *Config) Analyze(ctx context.Context, patterns []string) (arch.Report, error) {
	const op = "root.Config.Analyze"
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	model, err := cfg.readModel()
	if err != nil {
		return arch.Report{}, &arch.Error{Op: op, Err: err}
	}
	verdicts, err := cfg.readVerdicts()
	if err != nil {
		return arch.Report{}, &arch.Error{Op: op, Err: err}
	}
	prog, err := gopackages.Load(
		ctx,
		gopackages.Config{Dir: cfg.Dir, Patterns: patterns, Tags: cfg.Tags},
	)
	if err != nil {
		return arch.Report{}, &arch.Error{Op: op, Err: err}
	}
	inputs := arch.Inputs{
		Tool:      "anarch-lint " + toolVersion(),
		GoVersion: runtime.Version(),
		Tags:      slices.Sorted(slices.Values(cfg.Tags)),
		Patterns:  patterns,
	}
	return arch.Analyze(&prog, &model, verdicts, &inputs), nil
}

// WriteReport writes r to Stdout in the format --format selects.
func (cfg *Config) WriteReport(r *arch.Report) error {
	const op = "root.Config.WriteReport"
	var err error
	if cfg.Format == "json" {
		err = arch.WriteJSON(cfg.Stdout, r)
	} else {
		err = text.WriteReport(cfg.Stdout, r)
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
	path, required := cfg.ModelPath, true
	if path == "" {
		path, required = filepath.Join(cfg.Dir, defaultModel), false
	}
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
