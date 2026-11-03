package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/graphql"
	"github.com/StevenACoffman/anarch-lint/internal/openapi"
	"github.com/StevenACoffman/anarch-lint/internal/protobuf"
)

// contractFiles are the contract files the model's fitness rules name, read
// for arch.Input.
type contractFiles struct {
	apis    []arch.APIDocument
	graphql []arch.GraphQLSchema
	protos  []arch.ProtoFiles
}

// contractReader reads contract files under dir through read.
type contractReader struct {
	dir  string
	read func(file string) ([]byte, error)
}

// readContractFiles reads the contract files the model's fitness rules name:
// OpenAPI documents, GraphQL schemas and .proto files, each in its rules'
// order. No file
// may leave the repository that holds Dir, as for the domain models, and
// nothing is fetched over the network.
func (o *Options) readContractFiles(ctx context.Context, m *arch.Model) (contractFiles, error) {
	const op = "pipeline.Options.readContractFiles"
	var out contractFiles
	if len(m.Fitness.OpenAPI) == 0 && len(m.Fitness.GraphQL) == 0 &&
		len(m.Fitness.Protobuf) == 0 {
		return out, nil
	}
	read, err := confinedReader(o.Dir)
	if err != nil {
		return out, &arch.Error{Op: op, Err: err}
	}
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return out, &arch.Error{Op: op, Err: err}
	}
	r := &contractReader{dir: dir, read: read}
	if out.apis, err = r.openAPI(ctx, m.Fitness.OpenAPI); err != nil {
		return out, &arch.Error{Op: op, Err: err}
	}
	if out.graphql, err = r.graphQL(m.Fitness.GraphQL); err != nil {
		return out, &arch.Error{Op: op, Err: err}
	}
	if out.protos, err = r.protobuf(ctx, m.Fitness.Protobuf); err != nil {
		return out, &arch.Error{Op: op, Err: err}
	}
	return out, nil
}

// openAPI reads each rule's OpenAPI document.
func (r *contractReader) openAPI(
	ctx context.Context, rules []arch.OpenAPIRule,
) ([]arch.APIDocument, error) {
	out := make([]arch.APIDocument, 0, len(rules))
	for i := range rules {
		rel := rules[i].Path
		doc, err := openapi.Parse(ctx, openapi.Source{
			File: filepath.Join(r.dir, rel), Name: filepath.ToSlash(rel), Read: r.read,
		})
		if err != nil {
			return nil, &arch.Error{Op: "pipeline.contractReader.openAPI", Err: err}
		}
		out = append(out, doc)
	}
	return out, nil
}

// graphQL reads each rule's GraphQL schema and operation documents.
func (r *contractReader) graphQL(rules []arch.GraphQLRule) ([]arch.GraphQLSchema, error) {
	out := make([]arch.GraphQLSchema, 0, len(rules))
	for i := range rules {
		g := &rules[i]
		schema, err := r.graphQLFiles(g.Name, g.Schema)
		if err != nil {
			return nil, err
		}
		ops, err := r.graphQLFiles(g.Name, g.Operations)
		if err != nil {
			return nil, err
		}
		s, err := graphql.Parse(graphql.Source{Schema: schema, Operations: ops, Read: r.read})
		if err != nil {
			return nil, &arch.Error{Op: "pipeline.contractReader.graphQL", Err: err}
		}
		out = append(out, s)
	}
	return out, nil
}

// protobuf compiles each rule's .proto files.
func (r *contractReader) protobuf(
	ctx context.Context, rules []arch.ProtobufRule,
) ([]arch.ProtoFiles, error) {
	out := make([]arch.ProtoFiles, 0, len(rules))
	for i := range rules {
		files, importPaths, err := r.protoFiles(&rules[i])
		if err != nil {
			return nil, err
		}
		pf, err := protobuf.Parse(ctx, protobuf.Source{
			Files: files, ImportPaths: importPaths, Read: r.read,
		})
		if err != nil {
			return nil, &arch.Error{Op: "pipeline.contractReader.protobuf", Err: err}
		}
		out = append(out, pf)
	}
	return out, nil
}

// confinedReader returns a reader of files by absolute path that refuses
// any outside the repository that holds dir.
func confinedReader(dir string) (func(file string) ([]byte, error), error) {
	root, err := repositoryRoot(dir)
	if err != nil {
		return nil, err
	}
	return func(file string) ([]byte, error) {
		resolved, err := confined(root, file)
		if err != nil {
			return nil, err
		}
		return ReadFile(resolved)
	}, nil
}

// glob returns the files under dir that a rule's doublestar patterns match,
// as slash-separated paths relative to it, sorted and without duplicates;
// a pattern that matches no file is EINVALID, so a moved file cannot pass
// by checking nothing.
func (r *contractReader) glob(rule string, patterns []string) ([]string, error) {
	var names []string
	for _, pat := range patterns {
		if !filepath.IsLocal(filepath.FromSlash(pat)) {
			return nil, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
				"fitness rule %q: %q leaves the analysis root", rule, pat)}
		}
		matches, err := doublestar.Glob(os.DirFS(r.dir), pat, doublestar.WithFilesOnly())
		if err != nil || len(matches) == 0 {
			return nil, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
				"fitness rule %q: %q matches no file under %s", rule, pat, r.dir)}
		}
		names = append(names, matches...)
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// abs is the absolute path of a name glob returned.
func (r *contractReader) abs(name string) string {
	return filepath.Join(r.dir, filepath.FromSlash(name))
}

// graphQLFiles is the files a rule's patterns match, for the graphql
// adapter.
func (r *contractReader) graphQLFiles(rule string, patterns []string) ([]graphql.File, error) {
	names, err := r.glob(rule, patterns)
	if err != nil {
		return nil, err
	}
	out := make([]graphql.File, len(names))
	for i, n := range names {
		out[i] = graphql.File{Path: r.abs(n), Name: n}
	}
	return out, nil
}

// protoFiles is the files a rule's patterns match, each named relative to
// the first of its import paths that holds it, and the import paths
// themselves, for the protobuf adapter; a file under none of them is
// EINVALID.
func (r *contractReader) protoFiles(p *arch.ProtobufRule) ([]protobuf.File, []string, error) {
	names, err := r.glob(p.Name, p.Files)
	if err != nil {
		return nil, nil, err
	}
	importPaths := p.ImportPaths
	if len(importPaths) == 0 {
		importPaths = []string{"."}
	}
	dirs := make([]string, len(importPaths))
	for i, ip := range importPaths {
		if !filepath.IsLocal(filepath.FromSlash(ip)) && ip != "." {
			return nil, nil, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
				"fitness rule %q: import path %q leaves the analysis root", p.Name, ip)}
		}
		dirs[i] = r.abs(ip)
	}
	out := make([]protobuf.File, 0, len(names))
	for _, n := range names {
		i := slices.IndexFunc(importPaths, func(ip string) bool {
			return ip == "." || strings.HasPrefix(n, strings.TrimSuffix(ip, "/")+"/")
		})
		if i < 0 {
			return nil, nil, &arch.Error{Code: arch.EINVALID, Message: fmt.Sprintf(
				"fitness rule %q: %s lies under none of the import paths", p.Name, n)}
		}
		rel := n
		if importPaths[i] != "." {
			rel = strings.TrimPrefix(n, strings.TrimSuffix(importPaths[i], "/")+"/")
		}
		out = append(out, protobuf.File{Name: rel, Display: n})
	}
	return out, dirs, nil
}
