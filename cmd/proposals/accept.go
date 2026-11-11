package proposals

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// acceptConfig holds the configuration for proposals accept.
type acceptConfig struct {
	*Config
	flags   *ff.FlagSet
	command *ff.Command
}

func newAccept(parent *Config) {
	cfg := acceptConfig{Config: parent}
	cfg.flags = ff.NewFlagSet("accept").SetParent(parent.Flags)
	cfg.command = &ff.Command{
		Name:      "accept",
		Usage:     "anarch-lint [FLAGS] proposals accept FILE ID...",
		ShortHelp: "add proposed fitness rules to the model",
		LongHelp: `Accept adds the rules of the named proposals of FILE to the JSON model file (default anarch.json in --dir; a missing
file is started), each after the model's own rules of its kind, and writes
the model in its canonical form, as fmt does. A person runs it for the rules
they decided on; to reject a proposal, accept it and set its gate to off.

It refuses a proposal id the file does not hold and a rule name the model
already declares, and then writes nothing. When the model had no fitness
rule before, it says so: from then on every fitness finding gates.

Accepting changes the model's hash, so record the baseline again (baseline)
in a change of its own.`,
		Flags: cfg.flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.command)
}

func (cfg *acceptConfig) exec(_ context.Context, args []string) error {
	if len(args) < 2 {
		return &root.UsageError{
			Err: errors.New("proposals accept: a proposals file and an id are required"),
		}
	}
	props, err := readProposals(args[0])
	if err != nil {
		return fmt.Errorf("proposals accept: %w", err)
	}
	path := cfg.ModelFile()
	m, err := readModel(path)
	if err != nil {
		return fmt.Errorf("proposals accept: %w", err)
	}
	gated := m.Fitness.Enabled()
	if m, err = acceptAll(&m, &props, args[1:]); err != nil {
		return fmt.Errorf("proposals accept: %w", err)
	}
	// renameio writes a temporary file and renames it into place, so a
	// failed write leaves the old file whole; on Windows it writes in place.
	if err := maybe.WriteFile(path, m.Format(), 0o644); err != nil {
		return fmt.Errorf("proposals accept: %w",
			&arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("write %s: %v", path, err)})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "proposals accept: %d rule(s) added to %s\n", len(args)-1, path)
	if !gated {
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"proposals accept: %s had no fitness rule; every fitness finding "+
				"now gates (requireAssigned, rules matching nothing, ...)\n",
			path,
		)
	}
	return nil
}

// readModel reads the JSON model at path, or starts an empty one when the
// file does not exist. A YAML model is refused, as fmt refuses it: writing
// it back would lose its comments.
func readModel(path string) (arch.Model, error) {
	const op = "proposals.readModel"
	if ext := filepath.Ext(path); ext != ".json" {
		return arch.Model{}, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
			"%s: only a JSON model is written; writing YAML would lose its comments", path)}
	}
	src, err := pipeline.ReadFile(path)
	if arch.ErrorCode(err) == arch.ENOTFOUND {
		return arch.Model{Version: 1}, nil
	}
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op, Err: err}
	}
	m, err := pipeline.DecodeModelFile(path, src)
	if err != nil {
		return arch.Model{}, &arch.Error{Op: op, Err: err}
	}
	return m, nil
}

// acceptAll returns m with the rules of the proposals named by ids, in the
// order given.
func acceptAll(m *arch.Model, props *arch.Proposals, ids []string) (arch.Model, error) {
	byID := make(map[string]*arch.Proposal, len(props.Proposals))
	for i := range props.Proposals {
		byID[props.Proposals[i].ID] = &props.Proposals[i]
	}
	out := *m
	for _, id := range ids {
		p, ok := byID[id]
		if !ok {
			return arch.Model{}, &arch.Error{
				Code: arch.EINVALID, Message: fmt.Sprintf("no proposal has id %q", id),
			}
		}
		next, err := out.Accept(p)
		if err != nil {
			return arch.Model{}, &arch.Error{Op: "proposals.acceptAll " + id, Err: err}
		}
		out = next
	}
	return out, nil
}
