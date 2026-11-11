package judge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/renameio/v2/maybe"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/judging"
)

// emitConfig holds the configuration for judge emit.
type emitConfig struct {
	*Config
	configs         []string
	adversaries     []string
	maxExhibits     int
	maxExhibitBytes int
	flags           *ff.FlagSet
	command         *ff.Command
}

func newEmit(parent *Config) {
	cfg := emitConfig{Config: parent}
	budget := judging.DefaultBudget()
	cfg.flags = ff.NewFlagSet("emit").SetParent(parent.Flags)
	cfg.flags.StringListVar(&cfg.configs, 0, "config",
		"a judge configuration NAME=MODEL or NAME=MODEL:VARIANT (repeatable; at least two)")
	cfg.flags.StringListVar(&cfg.adversaries, 0, "adversary",
		"an adversary NAME=MODEL[:VARIANT] that challenges the judges' drafts awaiting a human, "+
			"shown their coverage but not their answers (repeatable)")
	cfg.flags.IntVar(&cfg.maxExhibits, 0, "max-exhibits", budget.MaxExhibits,
		"the most exhibits one prompt shows; the rest are counted")
	cfg.flags.IntVar(&cfg.maxExhibitBytes, 0, "max-exhibit-bytes", budget.MaxExhibitBytes,
		"the longest exhibit one prompt shows; the rest is counted")
	cfg.command = &ff.Command{
		Name:      "emit",
		Usage:     "anarch-lint judge emit --config NAME=MODEL --config NAME=MODEL [PACKAGES]",
		ShortHelp: "write a prompt per pending or stale slot, configuration and sample",
		Flags:     cfg.flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.command)
}

func (cfg *emitConfig) exec(ctx context.Context, args []string) error {
	configs, err := parseConfigs(cfg.configs, cfg.adversaries)
	if err != nil {
		return &root.UsageError{Err: fmt.Errorf("judge emit: %w", err)}
	}
	// Before the first admit, the judgements file does not exist yet.
	opts := cfg.Options
	if _, err := os.Stat(opts.JudgementsPath); errors.Is(err, fs.ErrNotExist) {
		opts.JudgementsPath = ""
	}
	slots, err := opts.JudgementSlots(ctx, args)
	if err != nil {
		return fmt.Errorf("judge emit: %w", err)
	}
	canaries, err := judging.Canaries()
	if err != nil {
		return fmt.Errorf("judge emit: %w", err)
	}
	gold, err := cfg.gold()
	if err != nil {
		return fmt.Errorf("judge emit: %w", err)
	}
	m := judging.Emit(&judging.EmitInput{
		Slots: slots, Canaries: canaries, Gold: gold, Configs: configs,
		Budget: judging.Budget{MaxExhibits: cfg.maxExhibits, MaxExhibitBytes: cfg.maxExhibitBytes},
	})
	written, err := cfg.write(&m)
	if err != nil {
		return fmt.Errorf("judge emit: %w", err)
	}
	if _, err := fmt.Fprintf(cfg.Stdout, "%d items, %d prompts to answer in %s: write each reply "+
		"beside its prompt as KEY%s, then run judge admit\n",
		len(m.Items), written, cfg.path(cfg.RunDir), replySuffix); err != nil {
		return fmt.Errorf("judge emit: %w", err)
	}
	return nil
}

// write writes the manifest, and a prompt for every item whose reply is not
// cached, returning how many prompts it wrote.
func (cfg *emitConfig) write(m *judging.Manifest) (int, error) {
	dir := cfg.path(cfg.RunDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
	}
	var manifest bytes.Buffer
	if err := arch.WriteJSON(&manifest, m); err != nil {
		return 0, &arch.Error{Op: "judge.emitConfig.write", Err: err}
	}
	if err := maybe.WriteFile(
		filepath.Join(dir, manifestFile),
		manifest.Bytes(),
		0o600,
	); err != nil {
		return 0, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
	}
	cached, err := cfg.replies(m, true)
	if err != nil {
		return 0, err
	}
	written := 0
	for i := range m.Items {
		it := &m.Items[i]
		if _, ok := cached[it.Key]; ok {
			continue
		}
		prompt := []byte(judging.Render(it))
		if err := maybe.WriteFile(
			filepath.Join(dir, it.Key+promptSuffix),
			prompt,
			0o600,
		); err != nil {
			return 0, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
		}
		written++
	}
	return written, nil
}

// parseConfigs reads NAME=MODEL[:VARIANT] flags: at least two judges, and
// any adversaries, each name once, since independence comes from different
// configurations.
func parseConfigs(judges, adversaries []string) ([]judging.Config, error) {
	var out []judging.Config
	seen := map[string]bool{}
	for i, f := range slices.Concat(judges, adversaries) {
		flag := "--config"
		if i >= len(judges) {
			flag = "--adversary"
		}
		name, spec, ok := strings.Cut(f, "=")
		model, variant, _ := strings.Cut(spec, ":")
		if !ok || name == "" || model == "" || seen[name] {
			return nil, fmt.Errorf("%s %q is not a new NAME=MODEL[:VARIANT]", flag, f)
		}
		seen[name] = true
		out = append(out, judging.Config{
			Name: name, Model: model, Variant: variant, Adversary: i >= len(judges),
		})
	}
	if len(judges) < 2 {
		return nil, errors.New("give at least two --config, so judges are independent")
	}
	return out, nil
}
