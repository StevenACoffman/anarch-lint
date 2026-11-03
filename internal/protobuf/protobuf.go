// Package protobuf reads .proto files into arch.ProtoFiles: the gRPC
// services they declare, and the messages whose buf.validate constraints do
// not compile. It compiles through github.com/bufbuild/protocompile, with
// imports resolved against the rule's import paths, the standard imports,
// and buf/validate/validate.proto from protovalidate's generated code, and
// compiles each message's constraints through buf.build/go/protovalidate.
// Every file is read through the reader the shell passes.
package protobuf

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"

	// Registers buf/validate/validate.proto, which constrained files import.
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// validateImport is the file buf.validate constraints are declared in.
const validateImport = "buf/validate/validate.proto"

// ReadFunc reads a file by its absolute path.
type ReadFunc func(file string) ([]byte, error)

// File is a .proto file to compile: its name relative to the import path
// that holds it, as imports name it, and the name positions give it.
type File struct {
	Name    string
	Display string
}

// Source is the .proto files to compile, and the directories, by absolute
// path and in order, that their names and imports resolve against.
type Source struct {
	Files       []File
	ImportPaths []string
	Read        ReadFunc
}

// Parse compiles the files src names. A file that does not compile is an
// error; a message whose constraints do not compile is a problem in the
// result.
func Parse(ctx context.Context, src Source) (arch.ProtoFiles, error) {
	const op = "protobuf.Parse"
	files, err := compile(ctx, src)
	if err != nil {
		return arch.ProtoFiles{}, &arch.Error{
			Op: op, Code: arch.EINVALID, Message: "compile: " + err.Error(),
		}
	}
	validator, err := protovalidate.New()
	if err != nil {
		return arch.ProtoFiles{}, &arch.Error{Op: op, Err: err}
	}
	display := map[string]string{}
	for _, f := range src.Files {
		display[f.Name] = f.Display
	}
	var out arch.ProtoFiles
	for _, f := range files {
		name := display[f.Path()]
		out.Services = append(out.Services, services(f, name)...)
		if imports(f, validateImport) {
			messages := allMessages(f.Messages())
			out.Messages += len(messages)
			out.Problems = append(out.Problems, constraintProblems(validator, f, messages, name)...)
		}
	}
	slices.SortFunc(out.Services, func(a, b arch.ProtoService) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(out.Problems, func(a, b arch.ProtoProblem) int {
		return cmp.Or(cmp.Compare(a.Pos.File, b.Pos.File), cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Message, b.Message))
	})
	return out, nil
}

// compile compiles src's files, with every error reported, not only the
// first.
func compile(ctx context.Context, src Source) (linker.Files, error) {
	sourceResolver := &protocompile.SourceResolver{
		ImportPaths: src.ImportPaths,
		Accessor: func(file string) (io.ReadCloser, error) {
			data, err := src.Read(file)
			if arch.ErrorCode(err) == arch.ENOTFOUND {
				// The resolver tries the next import path only on this error.
				return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, file)
			}
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
	var errs []error
	compiler := protocompile.Compiler{
		Resolver: protocompile.CompositeResolver{
			protocompile.WithStandardImports(sourceResolver),
			protocompile.ResolverFunc(generated),
		},
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter: reporter.NewReporter(func(err reporter.ErrorWithPos) error {
			errs = append(errs, err)
			return nil
		}, nil),
	}
	names := make([]string, len(src.Files))
	for i, f := range src.Files {
		names[i] = f.Name
	}
	files, err := compiler.Compile(ctx, names...)
	if len(errs) > 0 {
		// Files compile in parallel, so sort what they report.
		slices.SortFunc(errs, func(a, b error) int { return cmp.Compare(a.Error(), b.Error()) })
		return nil, errors.Join(errs...)
	}
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	return files, nil
}

// generated resolves buf/validate/validate.proto from the descriptor its
// generated Go package registers, so a constrained file needs no copy of it.
func generated(file string) (protocompile.SearchResult, error) {
	if file != validateImport {
		return protocompile.SearchResult{}, fs.ErrNotExist
	}
	fd, err := protoregistry.GlobalFiles.FindFileByPath(file)
	if err != nil {
		return protocompile.SearchResult{}, fmt.Errorf("%s: %w", file, err)
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

// services lists the services a file declares.
func services(f protoreflect.FileDescriptor, name string) []arch.ProtoService {
	goPackage := ""
	if opts, ok := f.Options().(*descriptorpb.FileOptions); ok {
		goPackage, _, _ = strings.Cut(opts.GetGoPackage(), ";")
	}
	out := make([]arch.ProtoService, 0, f.Services().Len())
	for i := range f.Services().Len() {
		sd := f.Services().Get(i)
		out = append(out, arch.ProtoService{
			Name: string(sd.FullName()), GoPackage: goPackage, Pos: position(f, sd, name),
		})
	}
	return out
}

// imports reports whether f imports the file named imp.
func imports(f protoreflect.FileDescriptor, imp string) bool {
	for i := range f.Imports().Len() {
		if f.Imports().Get(i).Path() == imp {
			return true
		}
	}
	return false
}

// allMessages lists messages and the messages nested in them, map entries
// aside.
func allMessages(mds protoreflect.MessageDescriptors) []protoreflect.MessageDescriptor {
	var out []protoreflect.MessageDescriptor
	for i := range mds.Len() {
		md := mds.Get(i)
		if md.IsMapEntry() {
			continue
		}
		out = append(out, md)
		out = append(out, allMessages(md.Messages())...)
	}
	return out
}

// constraintProblems compiles each message's constraints by validating an
// empty instance: protovalidate compiles them on first use, and reports a
// failure as a CompilationError, while a violation of the constraints by
// the empty message is no problem.
func constraintProblems(
	v protovalidate.Validator, f protoreflect.FileDescriptor,
	messages []protoreflect.MessageDescriptor, name string,
) []arch.ProtoProblem {
	var out []arch.ProtoProblem
	for _, md := range messages {
		err := v.Validate(dynamicpb.NewMessage(md))
		var compileErr *protovalidate.CompilationError
		if !errors.As(err, &compileErr) {
			continue
		}
		out = append(out, arch.ProtoProblem{
			Message: string(md.FullName()),
			Error:   firstLine(strings.TrimPrefix(compileErr.Error(), "compilation error: ")),
			Pos:     position(f, md, name),
		})
	}
	return out
}

// position is where a descriptor is declared; line 1 of its file when the
// source information has no location for it.
func position(f protoreflect.FileDescriptor, d protoreflect.Descriptor, name string) arch.Position {
	loc := f.SourceLocations().ByDescriptor(d)
	return arch.Position{File: name, Line: max(loc.StartLine+1, 1)}
}

// firstLine is s up to its first newline: CEL follows its error with the
// expression and a caret under the fault.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
