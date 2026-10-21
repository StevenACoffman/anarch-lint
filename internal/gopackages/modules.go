package gopackages

import (
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// moduleWalk is one search for nested modules: the analysis directory, the
// directories of the modules loaded, and the go.mod files found, by
// directory.
type moduleWalk struct {
	dir    string
	loaded map[string]bool
	found  map[string]arch.ExcludedFile
}

// nestedModules lists the modules a pattern's "..." would reach but for
// their own go.mod, which the go command never crosses, even under a go.work
// file that lists them. Each is an excluded go.mod, relative to dir, with its
// module path as the package, so the report says what it did not read.
//
// It walks below the directory of each relative pattern that ends in "...",
// skipping what the go command skips (names starting with "." or "_",
// testdata and vendor), and lists the outermost go.mod of each directory no
// loaded package's module lives in. A go.mod that cannot be read or parsed is
// listed with no module path: it is still a module the report did not read.
// A directory the walk cannot read is an EINVALID error, as it is to the go
// command.
//
// Ensures: sorted by path, each go.mod once.
func nestedModules(
	dir string,
	patterns []string,
	pkgs []*packages.Package,
) ([]arch.ExcludedFile, error) {
	w := moduleWalk{dir: dir, loaded: map[string]bool{}, found: map[string]arch.ExcludedFile{}}
	for _, p := range pkgs {
		if p.Module != nil && p.Module.Dir != "" {
			w.loaded[filepath.Clean(p.Module.Dir)] = true
		}
	}
	for _, base := range walkBases(patterns) {
		err := fs.WalkDir(os.DirFS(dir), base, func(p string, d fs.DirEntry, err error) error {
			return w.visit(base, p, d, err)
		})
		if err != nil {
			return nil, &arch.Error{
				Code: arch.EINVALID, Message: "look for nested modules: " + err.Error(),
			}
		}
	}
	out := slices.Collect(maps.Values(w.found))
	slices.SortFunc(out, compareExcluded)
	return out, nil
}

// visit is the fs.WalkDirFunc of a walk below base: a directory holding a
// go.mod is a module of its own, listed unless loaded, and not entered.
func (w *moduleWalk) visit(base, p string, d fs.DirEntry, err error) error {
	switch {
	case err != nil:
		return err
	case !d.IsDir() || p == base:
		return nil
	case skippedByGo(d.Name()):
		return fs.SkipDir
	}
	full := filepath.Join(w.dir, filepath.FromSlash(p))
	data, err := os.ReadFile(filepath.Join(full, "go.mod"))
	if os.IsNotExist(err) {
		return nil
	}
	if !w.loaded[full] {
		w.found[p] = arch.ExcludedFile{
			Path: path.Join(p, "go.mod"), Package: modfile.ModulePath(data),
			Reason: arch.ExcludedOtherModule,
		}
	}
	return fs.SkipDir
}

// walkBases returns the directory, in slash form and relative to the
// analysis directory, of each relative pattern that ends in "...": "." for
// "./...", "internal" for "./internal/...". Other patterns, import paths and
// name prefixes such as "./internal..." among them, are not walked.
func walkBases(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		rest, ok := strings.CutSuffix(filepath.ToSlash(p), "...")
		if !ok || !strings.HasPrefix(rest, "./") || !strings.HasSuffix(rest, "/") {
			continue
		}
		out = append(out, path.Clean(rest))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// skippedByGo reports whether the go command leaves a directory of this name
// out of a pattern that ends in "...".
func skippedByGo(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "vendor"
}
