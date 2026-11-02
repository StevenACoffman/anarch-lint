package pipeline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/modelith"
)

// readDomains reads the domain model each fitness.domains rule names, in the
// rules' order. A path may not leave the repository that holds Dir, after
// symlinks are resolved, so a model cannot make anarch-lint read arbitrary
// files (modelith's ADR-0013); nothing is fetched over the network.
func (o *Options) readDomains(m *arch.Model) ([]arch.DomainModel, error) {
	const op = "pipeline.Options.readDomains"
	if len(m.Fitness.Domains) == 0 {
		return nil, nil
	}
	root, err := repositoryRoot(o.Dir)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	out := make([]arch.DomainModel, 0, len(m.Fitness.Domains))
	for i := range m.Fitness.Domains {
		rel := m.Fitness.Domains[i].Path
		file, err := confined(root, filepath.Join(o.Dir, rel))
		if err != nil {
			return nil, &arch.Error{Op: op + " " + rel, Err: err}
		}
		src, err := ReadFile(file)
		if err != nil {
			return nil, &arch.Error{Op: op, Err: err}
		}
		d, err := modelith.Parse(filepath.ToSlash(rel), src)
		if err != nil {
			return nil, &arch.Error{Op: op, Err: err}
		}
		out = append(out, d)
	}
	return out, nil
}

// repositoryRoot returns the nearest directory at or above dir that holds a
// .git entry, symlinks resolved, or dir itself when there is none: outside a
// repository, assume the least.
func repositoryRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return "", &arch.Error{
			Code:    arch.EINVALID,
			Message: fmt.Sprintf("directory %q: %v", dir, err),
		}
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return abs, nil
		}
	}
}

// confined returns file with its symlinks resolved, or EINVALID when it lies
// outside root, and ENOTFOUND when it does not exist.
func confined(root, file string) (string, error) {
	resolved, err := filepath.EvalSymlinks(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", &arch.Error{
			Code:    arch.ENOTFOUND,
			Message: fmt.Sprintf("%s does not exist", file),
		}
	}
	if err != nil {
		return "", &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf("%s: %v", file, err)}
	}
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
			"%s resolves to %s, outside the repository at %s", file, resolved, root)}
	}
	return resolved, nil
}
