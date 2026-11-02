package gopackages

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// constantExpr returns e as a constant fact without its site: its type,
// exact value and context, when it is constant-valued or, as a call
// argument, a call to fmt.Sprintf of constants (foldSprintf).
func constantExpr(info *types.Info, e ast.Expr, ctx arch.ConstContext) (arch.ConstExpr, bool) {
	tv := info.Types[e]
	if tv.Value == nil && ctx == arch.ContextArgument {
		s, ok := foldSprintf(info, e)
		return arch.ConstExpr{Type: "string", Value: strconv.Quote(s), Context: ctx}, ok
	}
	if tv.Value == nil || tv.Type == nil {
		return arch.ConstExpr{}, false
	}
	return arch.ConstExpr{
		Type:    types.TypeString(types.Default(tv.Type), nil),
		Value:   tv.Value.ExactString(),
		Literal: isLiteral(e),
		Context: ctx,
	}, true
}

// foldSprintf returns the value of a call to fmt.Sprintf whose format and
// arguments are all constants, as Go source would print it, so a route
// written fmt.Sprintf("/tasks/{id:%s}", uuidPattern) is as constant as the
// literal it stands for; false for any other expression. Formatting
// constants is deterministic.
func foldSprintf(info *types.Info, e ast.Expr) (string, bool) {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || !isSprintf(info, c) {
		return "", false
	}
	format, ok := constantString(info, c.Args[0])
	if !ok {
		return "", false
	}
	args, ok := constantArgs(info, c.Args[1:])
	if !ok {
		return "", false
	}
	return fmt.Sprintf(format, args...), true
}

// isSprintf reports a call to fmt.Sprintf with a format and no spread
// argument.
func isSprintf(info *types.Info, c *ast.CallExpr) bool {
	if len(c.Args) == 0 || c.Ellipsis.IsValid() {
		return false
	}
	fn, ok := typeutil.Callee(info, c).(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" && fn.Name() == "Sprintf"
}

// constantArgs returns the values of arguments that are all constants.
func constantArgs(info *types.Info, exprs []ast.Expr) ([]any, bool) {
	args := make([]any, 0, len(exprs))
	for _, a := range exprs {
		v, ok := constantValue(info.Types[a].Value)
		if !ok {
			return nil, false
		}
		args = append(args, v)
	}
	return args, true
}

// constantString returns a constant string expression's value.
func constantString(info *types.Info, e ast.Expr) (string, bool) {
	v := info.Types[e].Value
	if v == nil || v.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(v), true
}

// constantValue returns a constant as the Go value fmt formats: a string,
// a bool, an int64 or a float64; false for a value none of them holds
// exactly.
func constantValue(v constant.Value) (any, bool) {
	if v == nil {
		return nil, false
	}
	switch v.Kind() {
	case constant.String:
		return constant.StringVal(v), true
	case constant.Bool:
		return constant.BoolVal(v), true
	case constant.Int:
		return constant.Int64Val(v)
	case constant.Float:
		return constant.Float64Val(v)
	default:
		return nil, false
	}
}
