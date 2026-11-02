package root

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
	"github.com/StevenACoffman/anarch-lint/internal/sarif"
	"github.com/StevenACoffman/anarch-lint/internal/text"
)

// ErrGateFailed exits with the status of a check whose gate failed: distinct from
// 1, a run that could not measure, and 2, a usage error, so CI can tell an
// architecture regression from a broken pipeline.
const ErrGateFailed ExitError = 3

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

// SARIFOptions are what check's SARIF log records besides the result: the
// tool's version, and the model file relative to --dir.
func (cfg *Config) SARIFOptions() sarif.Options {
	model := filepath.ToSlash(cfg.ModelFile())
	if rel, err := filepath.Rel(cfg.Dir, cfg.ModelFile()); err == nil &&
		!strings.HasPrefix(rel, "..") {
		model = filepath.ToSlash(rel)
	}
	return sarif.Options{Version: pipeline.ToolVersion(), ModelPath: model}
}
