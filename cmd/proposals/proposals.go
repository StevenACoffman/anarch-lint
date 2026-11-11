// Package proposals implements the "proposals" CLI command and its
// subcommands report and accept.
package proposals

import (
	"bytes"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// Config holds the configuration shared by the proposals subcommands.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the proposals command and its subcommands.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("proposals").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "proposals",
		Usage:     "anarch-lint [FLAGS] proposals report|accept",
		ShortHelp: "review the fitness rules another tool proposes",
		LongHelp: `Proposals reads a proposals file (anarch-lint schema --proposals), which a
tool such as codetospec writes: candidate fitness rules, each with the
evidence it was proposed from.

  report  loads the packages and says what each proposed rule would do:
          declared, when the model already has a rule of its name, or its
          outcome on the code as it is, with its violations; writes nothing
  accept  adds the named proposals' rules to the model

A proposal records what the code does today, not what it should do: a
person decides. To reject a proposal, accept it and set its gate to off, so
the decision stays in the model and the proposing tool skips the rule from
then on.`,
		Flags: cfg.Flags,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	newReport(&cfg)
	newAccept(&cfg)
	return &cfg
}

// readProposals reads and decodes the proposals file at path.
func readProposals(path string) (arch.Proposals, error) {
	const op = "proposals.readProposals"
	src, err := pipeline.ReadFile(path)
	if err != nil {
		return arch.Proposals{}, &arch.Error{Op: op, Err: err}
	}
	props, err := arch.DecodeProposals(bytes.NewReader(src))
	if err != nil {
		return arch.Proposals{}, &arch.Error{Op: op + " " + path, Err: err}
	}
	return props, nil
}
