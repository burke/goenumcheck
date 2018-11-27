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
				switch exp := expr.(type) {
				case *ast.Ident:
					found[exp.Name] = true
				case *ast.SelectorExpr:
					// TODO(burke): we're not asserting the package this is from, really.
					// This is kind of a shitty/lazy way of doing this.
					found[exp.Sel.Name] = true
				default:
					panic("idk")
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

func validateSwitch(pkgs []*types.Package, info types.Info, switchStmt *ast.SwitchStmt, pkgPath string) error {
	if switchedType, ok := typeNameOf(info, switchStmt); ok {
		names, ok := enumNamesFor(pkgs, switchedType)
		if !ok {
			return nil
		}
		return assertCases(switchStmt, switchedType.Name(), names)
	}

	return nil
}

// E.g. with `type demo int`, and a switch ranging over a value of type `demo`,
// this would return ("demo", true). Most other cases are ("", false).
func typeNameOf(info types.Info, switchStmt *ast.SwitchStmt) (*types.TypeName, bool) {
	expr, ok := switchStmt.Tag.(ast.Expr)
	if !ok {
		return nil, false
	}

	typeAndValue, ok := info.Types[expr]
	if !ok {
		return nil, false
	}

	namedType, ok := typeAndValue.Type.(*types.Named)
	if !ok {
		return nil, false
	}

	return namedType.Obj(), true
}

func CheckSwitch(f *lint.File) {
	allPkgs := typeutil.Dependencies(f.Pkg.TypesPkg)

	pkgPath := f.Pkg.TypesPkg.Path()
	info := f.Pkg.TypesInfo

	fn := func(node ast.Node) bool {
		switchStmt, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		if err := validateSwitch(allPkgs, info, switchStmt, pkgPath); err != nil {
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
func enumNamesFor(pkgs []*types.Package, typeName *types.TypeName) (names []string, ok bool) {
	for _, pkg := range pkgs {
		// is it the package we're looking for?
		if pkg.Path() != typeName.Pkg().Path() {
			continue
		}

		for _, name := range pkg.Scope().Names() {
			// is it the name we're looking for?
			if name != typeName.Name() {
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

			// is it an alias to `int` or `int32`?
			// TODO: maybe we should be looser about this? I'm not really sure.
			switch typ.Underlying() {
			case types.Typ[types.Int]:
			case types.Typ[types.Int32]:
			default:
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
