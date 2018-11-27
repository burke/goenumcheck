package goenumcheck

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
	"honnef.co/go/lint"
)

type Checker struct{}

func NewChecker() *Checker {
	return &Checker{}
}

func (c *Checker) Init(*lint.Program) {}

func (c *Checker) Funcs() map[string]lint.Func {
	return map[string]lint.Func{
		"EC1000": CheckSwitch,
	}
}

func assertCases(node *ast.SwitchStmt, switchedTypeName string, etype []string) error {
	found := make(map[string]bool)
	for _, e := range etype {
		found[e] = false
	}

	for _, caseClause := range node.Body.List {
		if c, ok := caseClause.(*ast.CaseClause); ok {
			if c.List == nil {
				// this is a default clause. When there's a default clause, we don't
				// require coverage.
				return nil
			}
			for _, expr := range c.List {
				if ident, ok := expr.(*ast.Ident); ok {
					found[ident.Name] = true
				}
			}
		}
	}

	var missing []string
	for name, fnd := range found {
		if !fnd {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf(
			"uncovered cases for %v enum switch\n\t- %v",
			switchedTypeName,
			strings.Join(missing, "\n\t- "),
		)
	}
	return nil
}

func validateSwitch(pkgs []*types.Package, switchStmt *ast.SwitchStmt, pkgPath string) error {
	if switchedType, ok := typeNameOf(switchStmt); ok {
		names, ok := enumNamesFor(pkgs, pkgPath, switchedType)
		if !ok {
			return nil
		}
		return assertCases(switchStmt, switchedType, names)
	}

	return nil
}

// E.g. with `type demo int`, and a switch ranging over a value of type `demo`,
// this would return ("demo", true). Most other cases are ("", false).
func typeNameOf(switchStmt *ast.SwitchStmt) (string, bool) {
	switch tag := switchStmt.Tag.(type) {
	case *ast.CallExpr: // e.g. switch action(...) in galaxy-go/snapshot/diff.go
		switch fun := tag.Fun.(type) {
		case *ast.SelectorExpr:
			var selector *ast.Ident = fun.Sel
			_ = selector

			switch recv := fun.X.(type) {
			case *ast.Ident:
				fmt.Printf("%#v\n", recv.Obj.Decl)
			}
		case *ast.Ident:
			if obj := fun.Obj; obj != nil {
				if decl, ok := obj.Decl.(*ast.FuncDecl); ok {
					var resFL *ast.FieldList = decl.Type.Results
					if len(resFL.List) == 1 {
						var field *ast.Field = resFL.List[0]
						if resType, ok := field.Type.(*ast.Ident); ok {
							return resType.Name, true
						}
					}
				}
			}
		}
	case *ast.Ident:
		if fld, ok := tag.Obj.Decl.(*ast.Field); ok {
			if tid, ok := fld.Type.(*ast.Ident); ok {
				obj := tid.Obj
				if obj != nil && obj.Kind == ast.Typ {
					return obj.Name, true
				}
			}
		}
	}
	return "", false
}

func CheckSwitch(f *lint.File) {
	allPkgs := typeutil.Dependencies(f.Pkg.TypesPkg)

	pkgPath := f.Pkg.TypesPkg.Path()

	fn := func(node ast.Node) bool {
		switchStmt, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		// fmt.Printf("%s, %#v\n", f.Filename, switchStmt.Tag)
		if err := validateSwitch(allPkgs, switchStmt, pkgPath); err != nil {
			f.Errorf(switchStmt, err.Error())
			return false
		}
		return true
	}
	f.Walk(fn)
}

// If we have a source file containing:
//   type foo int ; const a foo = iota ; const b foo = iota
// and we invoke this with the path of that package and "foo",
// we get back []string{"a", "b"}, true.
func enumNamesFor(pkgs []*types.Package, pkgPath, typeName string) (names []string, ok bool) {
	for _, pkg := range pkgs {
		// is it the package we're looking for?
		if pkg.Path() != pkgPath {
			continue
		}

		for _, name := range pkg.Scope().Names() {
			// is it the name we're looking for?
			if name != typeName {
				continue
			}

			// is it a type?
			obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}

			// is a type alias?
			typ, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}

			// is it an alias to `int`?
			if types.Typ[types.Int] != typ.Underlying() {
				continue
			}

			// Find all package-local instances of this type.
			for _, name := range pkg.Scope().Names() {
				if obj, ok := pkg.Scope().Lookup(name).(*types.Const); ok {
					if obj.Type() == typ {
						names = append(names, obj.Name())
					}
				}
			}
		}
	}
	return names, len(names) != 0
}
