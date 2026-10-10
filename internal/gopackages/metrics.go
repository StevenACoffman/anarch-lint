package gopackages

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// complexityOf returns the three cyclomatic-complexity variants of a function
// (mmi.md §7.3), folding function literals into the function that contains
// them, as gocyclo does:
//
//   - Plain is 1, plus one per if, for and range, per case clause that is not
//     default, and per select clause that is not default.
//   - Modified counts a switch, type switch or select once instead of once
//     per clause (von Zitzewitz).
//   - Extended is Plain plus one per && and ||; it equals gocyclo's value.
func complexityOf(d *ast.FuncDecl) arch.Complexity {
	c := arch.Complexity{Plain: 1, Modified: 1}
	logical := 0
	ast.Inspect(d, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c.Plain++
			c.Modified++
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			c.Modified++
		case *ast.CaseClause:
			if n.List != nil {
				c.Plain++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				c.Plain++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				logical++
			}
		}
		return true
	})
	c.Extended = c.Plain + logical
	return c
}

// countStatements returns von Zitzewitz's Number of Statements: every
// statement in the function, including those in a for loop's header and in
// function literals, but not blocks, labels, empty statements or the case and
// select clauses that only group other statements.
func countStatements(d *ast.FuncDecl) int {
	n := 0
	ast.Inspect(d, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.BlockStmt, *ast.LabeledStmt, *ast.EmptyStmt, *ast.CaseClause, *ast.CommClause:
		case ast.Stmt:
			n++
		}
		return true
	})
	return n
}

// maxNesting returns how deeply the function's most indented statement is
// nested, with the body's own statements at depth 1 (von Zitzewitz's
// indentation depth, mmi.md §7.3). Depth follows gofmt: the body of an if,
// for, range or block adds a level; a switch or select adds one level for the
// statements of its clauses, which sit at the switch's own indentation; and a
// function literal's body adds a level below the statement that holds it.
func maxNesting(d *ast.FuncDecl) int {
	if d.Body == nil {
		return 0
	}
	return listDepth(d.Body.List, 1)
}

// listDepth returns the deepest nesting among statements at depth.
func listDepth(list []ast.Stmt, depth int) int {
	deepest := 0
	for _, s := range list {
		deepest = max(deepest, depth, stmtDepth(s, depth))
	}
	return deepest
}

// stmtDepth returns the deepest nesting inside s, which sits at depth.
func stmtDepth(s ast.Stmt, depth int) int {
	switch s := s.(type) {
	case *ast.BlockStmt:
		return listDepth(s.List, depth+1)
	case *ast.IfStmt:
		return max(listDepth(s.Body.List, depth+1), elseDepth(s.Else, depth))
	case *ast.ForStmt:
		return listDepth(s.Body.List, depth+1)
	case *ast.RangeStmt:
		return listDepth(s.Body.List, depth+1)
	case *ast.SwitchStmt:
		return clauseDepth(s.Body, depth)
	case *ast.TypeSwitchStmt:
		return clauseDepth(s.Body, depth)
	case *ast.SelectStmt:
		return clauseDepth(s.Body, depth)
	case *ast.LabeledStmt:
		return stmtDepth(s.Stmt, depth)
	default:
		return funcLitDepth(s, depth)
	}
}

// elseDepth returns the deepest nesting of an else branch of an if at depth:
// an else-if sits at the if's own depth, and an else block one deeper.
func elseDepth(s ast.Stmt, depth int) int {
	switch s := s.(type) {
	case nil:
		return 0
	case *ast.BlockStmt:
		return listDepth(s.List, depth+1)
	default:
		return stmtDepth(s, depth)
	}
}

// clauseDepth returns the deepest nesting of the clauses of a switch or select
// at depth, whose statements sit one level deeper.
func clauseDepth(body *ast.BlockStmt, depth int) int {
	deepest := depth
	for _, c := range body.List {
		switch c := c.(type) {
		case *ast.CaseClause:
			deepest = max(deepest, listDepth(c.Body, depth+1))
		case *ast.CommClause:
			deepest = max(deepest, listDepth(c.Body, depth+1))
		}
	}
	return deepest
}

// funcLitDepth returns the deepest nesting of the function literals inside a
// simple statement at depth, or depth when it holds none.
func funcLitDepth(s ast.Stmt, depth int) int {
	deepest := depth
	ast.Inspect(s, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			deepest = max(deepest, listDepth(lit.Body.List, depth+1))
			return false
		}
		return true
	})
	return deepest
}

// receiverUses returns the sorted names of the receiver's fields and methods
// that a method selects through its receiver variable, for LCOM4.
func receiverUses(info *types.Info, d *ast.FuncDecl) []string {
	if d.Recv == nil || len(d.Recv.List) == 0 || len(d.Recv.List[0].Names) == 0 {
		return nil
	}
	recv := info.Defs[d.Recv.List[0].Names[0]]
	if recv == nil {
		return nil
	}
	var uses []string
	ast.Inspect(d.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && info.Uses[x] == recv {
			uses = append(uses, sel.Sel.Name)
		}
		return true
	})
	slices.Sort(uses)
	return slices.Compact(uses)
}
