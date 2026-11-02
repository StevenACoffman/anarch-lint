// Package judge implements the "judge" CLI command and its subcommands emit,
// admit and report.
package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2/maybe"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/anarch-lint/cmd/root"
	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/judging"
	"github.com/StevenACoffman/anarch-lint/internal/pipeline"
)

// ErrUntrustworthy exits with the status of a judging run in which a judge
// configuration missed a canary: its answers in that family were refused,
// and CI should know the judges cannot all be trusted (ADH's exit 15).
const ErrUntrustworthy root.ExitError = 4

// The files of a judging run.
const (
	manifestFile = "manifest.json"
	promptSuffix = ".prompt.md"
	replySuffix  = ".reply.md"
)

// Config holds the configuration shared by the judge subcommands.
type Config struct {
	*root.Config
	// RunDir holds a run's manifest, prompts and replies.
	RunDir string
	// CacheDir keeps every admitted reply by key, so a later run reuses it.
	CacheDir string
	// GoldPath names a file of human-labelled questions.
	GoldPath string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the judge command and its subcommands.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("judge").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.RunDir, 0, "run", ".anarch/judging/run",
		"directory of the run's manifest, prompts and replies, relative to --dir")
	cfg.Flags.StringVar(&cfg.CacheDir, 0, "cache", ".anarch/judging/cache",
		"directory of admitted replies, by key, relative to --dir")
	cfg.Flags.StringVar(&cfg.GoldPath, 0, "gold", "",
		"file of human-labelled questions, asked to measure the judges")
	cfg.Command = &ff.Command{
		Name:      "judge",
		Usage:     "anarch-lint [FLAGS] judge emit|admit|report",
		ShortHelp: "ask judges the judgement slots, and admit their answers",
		LongHelp: `Judge relays the judgement slots to judges anarch-lint does not run itself, such
as LLM agents, and admits their answers (mmi-plan.md §9.4):

  emit    writes one prompt per pending or stale slot, judge configuration and
          sample, with canaries of known answer mixed in
  admit   parses the replies strictly, refuses a configuration's answers in a
          family whose canaries it missed, tallies the rest, and records them
          in the judgements file as judges' events
  report  shows the canaries, the gold set's agreement and calibration, the
          tallies and the rejected replies, writing nothing

A judge's verdict is a draft: it applies only once a person attests it with a
human event. admit exits 4 when a configuration missed a canary.`,
		Flags: cfg.Flags,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	newEmit(&cfg)
	newAdmit(&cfg)
	newReport(&cfg)
	return &cfg
}

// path resolves a run-relative path against --dir.
func (cfg *Config) path(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(cfg.Dir, p)
}

// gold reads the gold set, if one is named.
func (cfg *Config) gold() ([]judging.Labelled, error) {
	if cfg.GoldPath == "" {
		return nil, nil
	}
	const op = "judge.Config.gold"
	src, err := pipeline.ReadFile(cfg.path(cfg.GoldPath))
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	gold, err := judging.DecodeLabelled(bytes.NewReader(src))
	if err != nil {
		return nil, &arch.Error{Op: op + " " + cfg.GoldPath, Err: err}
	}
	return gold, nil
}

// readManifest reads the run's manifest.
func (cfg *Config) readManifest() (judging.Manifest, error) {
	var m judging.Manifest
	src, err := pipeline.ReadFile(filepath.Join(cfg.path(cfg.RunDir), manifestFile))
	if err != nil {
		return m, &arch.Error{Op: "judge.Config.readManifest", Err: err}
	}
	if err := json.Unmarshal(src, &m); err != nil {
		return m, &arch.Error{Code: arch.EINVALID, Message: "manifest: " + err.Error()}
	}
	if m.PromptVersion != judging.PromptVersion {
		return m, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
			"the run was emitted for prompt version %d, this build renders %d; emit again",
			m.PromptVersion, judging.PromptVersion)}
	}
	return m, nil
}

// replies reads each item's reply from the run, or else from the cache;
// with cacheOnly, from the cache alone.
func (cfg *Config) replies(m *judging.Manifest, cacheOnly bool) (map[string]string, error) {
	out := map[string]string{}
	for i := range m.Items {
		it := &m.Items[i]
		dirs := []string{cfg.path(cfg.RunDir), cfg.path(cfg.CacheDir)}
		if cacheOnly {
			dirs = dirs[1:]
		}
		for _, dir := range dirs {
			src, err := os.ReadFile(filepath.Join(dir, it.Key+replySuffix))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, &arch.Error{Code: arch.EINVALID, Message: err.Error()}
			}
			out[it.Key] = string(src)
			break
		}
	}
	return out, nil
}

// cache keeps every reply by key.
func (cfg *Config) cache(replies map[string]string) error {
	dir := cfg.path(cfg.CacheDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return &arch.Error{Code: arch.EINVALID, Message: err.Error()}
	}
	for key, reply := range replies {
		if err := maybe.WriteFile(
			filepath.Join(dir, key+replySuffix),
			[]byte(reply),
			0o600,
		); err != nil {
			return &arch.Error{Code: arch.EINVALID, Message: err.Error()}
		}
	}
	return nil
}

// admit parses the run's replies against the recorded verdicts.
func (cfg *Config) admit(cacheOnly bool) (judging.Result, string, error) {
	m, err := cfg.readManifest()
	if err != nil {
		return judging.Result{}, "", err
	}
	replies, err := cfg.replies(&m, cacheOnly)
	if err != nil {
		return judging.Result{}, "", err
	}
	canaries, err := judging.Canaries()
	if err != nil {
		return judging.Result{}, "", &arch.Error{Op: "judge.Config.admit", Err: err}
	}
	gold, err := cfg.gold()
	if err != nil {
		return judging.Result{}, "", err
	}
	signer, verdicts, err := cfg.recorded()
	if err != nil {
		return judging.Result{}, "", err
	}
	r := judging.Admit(&judging.AdmitInput{
		Manifest: m, Replies: replies, Verdicts: verdicts, Canaries: canaries, Gold: gold,
	})
	if err := cfg.cache(replies); err != nil {
		return judging.Result{}, "", err
	}
	return r, signer, nil
}

// recorded reads the judgements file, which may not exist yet.
func (cfg *Config) recorded() (string, []arch.Verdict, error) {
	if cfg.JudgementsPath == "" {
		return "", nil, &root.UsageError{Err: errors.New("judge: --judgements names no file")}
	}
	const op = "judge.Config.recorded"
	src, err := pipeline.ReadFile(cfg.JudgementsPath)
	if arch.ErrorCode(err) == arch.ENOTFOUND {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, &arch.Error{Op: op, Err: err}
	}
	signer, verdicts, err := arch.DecodeSignedJudgements(bytes.NewReader(src))
	if err != nil {
		return "", nil, &arch.Error{Op: op + " " + cfg.JudgementsPath, Err: err}
	}
	return signer, verdicts, nil
}
