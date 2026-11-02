// Package openapi reads OpenAPI documents into arch.APIDocument: each
// operation's method, path template and declaring line, and the servers'
// base paths. It parses through github.com/getkin/kin-openapi, resolving
// $ref across files; a Swagger 2.0 document is converted to OpenAPI 3
// first, through kin-openapi's openapi2conv, and its lines are then
// unknown. Every file is read through the reader the shell passes, so a
// document cannot make anarch-lint read outside the repository, and nothing
// is fetched over the network.
package openapi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ReadFunc reads a file the document refers to, by its absolute path.
type ReadFunc func(file string) ([]byte, error)

// Source is a document to parse: its absolute path, the name positions give
// it, and how to read it and every file it refers to.
type Source struct {
	File string
	Name string
	Read ReadFunc
}

// Parse reads the OpenAPI document src names.
func Parse(ctx context.Context, src Source) (arch.APIDocument, error) {
	const op = "openapi.Parse"
	data, err := src.Read(src.File)
	if err != nil {
		return arch.APIDocument{}, &arch.Error{Op: op, Err: err}
	}
	doc, err := load(ctx, src.File, data, src.Read)
	if err != nil {
		return arch.APIDocument{}, &arch.Error{
			Op: op, Code: arch.EINVALID, Message: fmt.Sprintf("%s: %v", src.Name, err),
		}
	}
	return documentOf(doc, src.File, src.Name), nil
}

// load parses src as an OpenAPI 3 document, or as a Swagger 2.0 one
// converted to it, resolving references through read.
func load(ctx context.Context, file string, src []byte, read ReadFunc) (*openapi3.T, error) {
	var version struct {
		Swagger string `json:"swagger"`
		OpenAPI string `json:"openapi"`
	}
	if err := yaml.Unmarshal(src, &version); err != nil {
		return nil, fmt.Errorf("read the version: %w", err)
	}
	switch {
	case version.Swagger != "":
		var v2 openapi2.T
		if err := yaml.Unmarshal(src, &v2); err != nil {
			return nil, fmt.Errorf("read the Swagger 2.0 document: %w", err)
		}
		doc, err := openapi2conv.ToV3(&v2)
		if err != nil {
			return nil, fmt.Errorf("convert the Swagger 2.0 document: %w", err)
		}
		return doc, nil
	case version.OpenAPI == "":
		return nil, errors.New("not an OpenAPI document: it has no openapi or swagger version")
	}
	loader := openapi3.NewLoader()
	loader.Context = ctx
	loader.IncludeOrigin = true
	loader.ReadFromURIFunc = func(_ *openapi3.Loader, u *url.URL) ([]byte, error) {
		if u.Scheme != "" && u.Scheme != "file" {
			return nil, fmt.Errorf("%s: only files in the repository are read", u)
		}
		return read(filepath.FromSlash(u.Path))
	}
	doc, err := loader.LoadFromDataWithPath(src, &url.URL{Path: filepath.ToSlash(file)})
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return doc, nil
}

// documentOf lists a document's operations, sorted by path and method.
func documentOf(doc *openapi3.T, file, rel string) arch.APIDocument {
	out := arch.APIDocument{Path: rel}
	for _, s := range doc.Servers {
		if bp, err := s.BasePath(); err == nil && bp != "/" && bp != "" {
			out.BasePaths = append(out.BasePaths, strings.TrimSuffix(bp, "/"))
		}
	}
	slices.Sort(out.BasePaths)
	out.BasePaths = slices.Compact(out.BasePaths)
	if doc.Paths == nil {
		return out
	}
	for p, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			out.Operations = append(out.Operations, arch.APIOperation{
				Method: strings.ToUpper(method), Path: p, Pos: position(o.Origin, file, rel),
			})
		}
	}
	slices.SortFunc(out.Operations, func(a, b arch.APIOperation) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Method, b.Method))
	})
	return out
}

// position is where an origin is, named relative to the document's own
// name; line 1 of the document when the origin is unknown, as in a
// converted Swagger 2.0 document.
func position(o *openapi3.Origin, file, rel string) arch.Position {
	if o == nil || o.Key == nil {
		return arch.Position{File: rel, Line: 1}
	}
	name := rel
	if k := filepath.FromSlash(o.Key.File); k != "" && k != file {
		if r, err := filepath.Rel(filepath.Dir(file), k); err == nil {
			name = path.Join(path.Dir(rel), filepath.ToSlash(r))
		}
	}
	return arch.Position{File: name, Line: o.Key.Line}
}
