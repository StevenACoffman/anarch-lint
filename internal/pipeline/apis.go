package pipeline

import (
	"context"
	"path/filepath"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/openapi"
)

// readAPIs reads the OpenAPI document each fitness.openapi rule names, in
// the rules' order. The document, and every file it refers to, may not
// leave the repository that holds Dir, as for the domain models.
func (o *Options) readAPIs(ctx context.Context, m *arch.Model) ([]arch.APIDocument, error) {
	const op = "pipeline.Options.readAPIs"
	if len(m.Fitness.OpenAPI) == 0 {
		return nil, nil
	}
	root, err := repositoryRoot(o.Dir)
	if err != nil {
		return nil, &arch.Error{Op: op, Err: err}
	}
	read := func(file string) ([]byte, error) {
		resolved, err := confined(root, file)
		if err != nil {
			return nil, err
		}
		return ReadFile(resolved)
	}
	out := make([]arch.APIDocument, 0, len(m.Fitness.OpenAPI))
	for i := range m.Fitness.OpenAPI {
		rel := m.Fitness.OpenAPI[i].Path
		file, err := filepath.Abs(filepath.Join(o.Dir, rel))
		if err != nil {
			return nil, &arch.Error{Op: op + " " + rel, Err: err}
		}
		doc, err := openapi.Parse(ctx, openapi.Source{
			File: file, Name: filepath.ToSlash(rel), Read: read,
		})
		if err != nil {
			return nil, &arch.Error{Op: op, Err: err}
		}
		out = append(out, doc)
	}
	return out, nil
}
