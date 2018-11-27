package goenumcheck

import (
	"go/ast"
	"go/types"
	"log"

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

func assertCases(node *ast.SwitchStmt, etype []string) bool {
	found := make(map[string]bool)
	for _, e := range etype {
		found[e] = false
	}

	for _, caseClause := range node.Body.List {
		c, ok := caseClause.(*ast.CaseClause)
		if !ok {
			log.Printf("got unexpected node: %v", caseClause)
			continue
		}
		if c.List == nil {
			// TODO: handle default clause
		} else {
			for _, expr := range c.List {
				ident, ok := expr.(*ast.Ident)
				if !ok {
					return false
				}

				fnd, ok := found[ident.Name]
				if !ok {
					log.Println("found unexpected case:", ident.Name)
				}

				if fnd {
					log.Println("duplicate case:", ident.Name)
				}

				found[ident.Name] = true
			}
		}
	}

	for name, fnd := range found {
		if !fnd {
			log.Println("missing case:", name)
			return false
		}
	}
	return true
}

func validateSwitch(pkgs []*types.Package, node *ast.SwitchStmt, pkgPath string) bool {
	if ident, ok := node.Tag.(*ast.Ident); ok {
		if fld, ok := ident.Obj.Decl.(*ast.Field); ok {
			if tid, ok := fld.Type.(*ast.Ident); ok {
				obj := tid.Obj
				if obj.Kind == ast.Typ {
					// obj might be an enumType
					names, ok := enumNamesFor(pkgs, pkgPath, obj.Name)
					if !ok {
						return true
					}
					return assertCases(node, names)
				}
			}
		}
	}
	return true
}

func CheckSwitch(f *lint.File) {
	allPkgs := typeutil.Dependencies(f.Pkg.TypesPkg)

	pkgPath := f.Pkg.TypesPkg.Path()

	fn := func(node ast.Node) bool {
		switchStmt, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		return validateSwitch(allPkgs, switchStmt, pkgPath)
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
