// Package graphql reads GraphQL schemas into arch.GraphQLSchema: the fields
// of the root operation types, and the errors of operation documents
// validated against the schema. It parses and validates through
// github.com/vektah/gqlparser/v2, the parser gqlgen and genqlient are built
// on. Every file is read through the reader the shell passes.
package graphql

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vektah/gqlparser/v2/validator/rules"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// ReadFunc reads a file by its absolute path.
type ReadFunc func(file string) ([]byte, error)

// File is a file to read: its absolute path, and the name positions give
// it.
type File struct {
	Path string
	Name string
}

// Source is one schema, spread over files, and the operation documents to
// validate against it.
type Source struct {
	Schema     []File
	Operations []File
	Read       ReadFunc
}

// Parse reads the schema src names and validates its operation documents.
// A schema that does not load is an error; an operation document that does
// not parse or validate is a problem in the result.
func Parse(src Source) (arch.GraphQLSchema, error) {
	const op = "graphql.Parse"
	schemaSources, err := sources(src.Schema, src.Read)
	if err != nil {
		return arch.GraphQLSchema{}, &arch.Error{Op: op, Err: err}
	}
	schema, err := gqlparser.LoadSchema(schemaSources...)
	if err != nil {
		return arch.GraphQLSchema{}, &arch.Error{
			Op: op, Code: arch.EINVALID, Message: "load the schema: " + describe(err),
		}
	}
	opSources, err := sources(src.Operations, src.Read)
	if err != nil {
		return arch.GraphQLSchema{}, &arch.Error{Op: op, Err: err}
	}
	out := arch.GraphQLSchema{Fields: rootFields(schema), Operations: len(opSources)}
	for _, s := range opSources {
		out.Problems = append(out.Problems, validate(schema, s)...)
	}
	slices.SortFunc(out.Problems, func(a, b arch.GraphQLProblem) int {
		return cmp.Or(cmp.Compare(a.Pos.File, b.Pos.File), cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Message, b.Message))
	})
	return out, nil
}

// sources reads files as gqlparser sources, named as positions give them.
func sources(files []File, read ReadFunc) ([]*ast.Source, error) {
	out := make([]*ast.Source, 0, len(files))
	for _, f := range files {
		data, err := read(f.Path)
		if err != nil {
			return nil, err
		}
		out = append(out, &ast.Source{Name: f.Name, Input: string(data)})
	}
	return out, nil
}

// rootFields lists the fields of a schema's root operation types, sorted
// by root type and name, without the introspection fields.
func rootFields(schema *ast.Schema) []arch.GraphQLField {
	var out []arch.GraphQLField
	for _, root := range []*ast.Definition{schema.Query, schema.Mutation, schema.Subscription} {
		if root == nil {
			continue
		}
		for _, f := range root.Fields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			out = append(out, arch.GraphQLField{
				Root: root.Name, Name: f.Name, Pos: position(f.Position),
			})
		}
	}
	slices.SortFunc(out, func(a, b arch.GraphQLField) int {
		return cmp.Or(cmp.Compare(a.Root, b.Root), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// validate parses one operation document and validates it against schema,
// returning its errors as problems.
func validate(schema *ast.Schema, src *ast.Source) []arch.GraphQLProblem {
	doc, err := parser.ParseQuery(src)
	if err != nil {
		return []arch.GraphQLProblem{problem(src.Name, err)}
	}
	var out []arch.GraphQLProblem
	for _, e := range validator.ValidateWithRules(schema, doc, rules.NewDefaultRules()) {
		out = append(out, problem(src.Name, e))
	}
	return out
}

// problem is an operation document's error, at its first location.
func problem(name string, err error) arch.GraphQLProblem {
	p := arch.GraphQLProblem{Message: err.Error(), Pos: arch.Position{File: name, Line: 1}}
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) {
		p.Message = gqlErr.Message
		if len(gqlErr.Locations) > 0 {
			p.Pos.Line = gqlErr.Locations[0].Line
		}
	}
	return p
}

// describe is a schema error with the file and line it names.
func describe(err error) string {
	var gqlErr *gqlerror.Error
	if !errors.As(err, &gqlErr) {
		return err.Error()
	}
	where := ""
	if file, ok := gqlErr.Extensions["file"].(string); ok {
		where = file
	}
	if len(gqlErr.Locations) > 0 {
		where += fmt.Sprintf(":%d", gqlErr.Locations[0].Line)
	}
	if where == "" {
		return gqlErr.Message
	}
	return where + ": " + gqlErr.Message
}

// position is where a schema element is declared.
func position(p *ast.Position) arch.Position {
	if p == nil || p.Src == nil {
		return arch.Position{}
	}
	return arch.Position{File: p.Src.Name, Line: p.Line}
}
