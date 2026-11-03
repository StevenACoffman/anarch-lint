package arch

import "strings"

// ProtoFiles is what a fitness.protobuf rule reads: the gRPC services a set
// of .proto files declare, and the buf.validate constraints among them that
// do not compile.
type ProtoFiles struct {
	// Services are the files' services, sorted by full name.
	Services []ProtoService
	// Messages is how many messages had their constraints compiled.
	Messages int
	// Problems are the messages whose constraints protovalidate cannot
	// compile, sorted by position.
	Problems []ProtoProblem
}

// ProtoService is one service a .proto file declares, and where.
type ProtoService struct {
	// Name is the service's full name, such as "tasks.v1.Tasks".
	Name string
	// GoPackage is the import path of the file's go_package option, without
	// a ";name" suffix; "" when the file has none.
	GoPackage string
	Pos       Position
}

// GRPCService is a gRPC contract's service: the generated Go package that
// holds it, and the service's Go name.
type GRPCService struct {
	GoPackage string
	Name      string
}

// ProtoProblem is a message whose buf.validate constraints do not compile.
type ProtoProblem struct {
	// Message is the message's full name.
	Message string
	// Error is why its constraints do not compile.
	Error string
	Pos   Position
}

// ContractID is the ID of the gRPC contract the service's generated code
// provides, "grpc::example.com/pb.Tasks", as ContractsOf finds it; "" when
// the file names no go_package.
func (s *ProtoService) ContractID() string {
	if s.GoPackage == "" {
		return ""
	}
	name := s.Name[strings.LastIndex(s.Name, ".")+1:]
	return grpcID(s.GoPackage, name)
}

// GRPCService returns a gRPC contract's service; false for a contract of
// another kind.
func (c *Contract) GRPCService() (GRPCService, bool) {
	rest, ok := strings.CutPrefix(c.ID, "grpc::")
	if !ok {
		return GRPCService{}, false
	}
	dot := strings.LastIndex(rest, ".")
	if dot < 0 {
		return GRPCService{}, false
	}
	return GRPCService{GoPackage: rest[:dot], Name: rest[dot+1:]}, true
}
