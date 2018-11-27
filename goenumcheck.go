package goenumcheck

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"

	"honnef.co/go/lint"
)

type enumType struct {
	typ   *types.Named
	names []string
}

var enumTypes map[string][]string

func init() {
	enumTypes = make(map[string][]string)
}

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

func assertCases(node *ast.SwitchStmt, info types.Info, etype []string) bool {
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

func resolve(node *ast.SwitchStmt, info types.Info, pkgName string) bool {
	if ident, ok := node.Tag.(*ast.Ident); ok {
		if fld, ok := ident.Obj.Decl.(*ast.Field); ok {
			if tid, ok := fld.Type.(*ast.Ident); ok {
				obj := tid.Obj
				if obj.Kind == ast.Typ {
					if etype, ok := enumTypes[pkgName+"."+obj.Name]; ok {
						return assertCases(node, info, etype)
					}
				}
			}
		}
	}
	return true
}

func CheckSwitch(f *lint.File) {
	doOtherShit()
	// all := typeutil.Dependencies(f.Pkg.TypesPkg)
	info := f.Pkg.TypesInfo
	pkgName := f.Pkg.TypesPkg.Name()

	fn := func(node ast.Node) bool {
		switchStmt, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		if !resolve(switchStmt, info, pkgName) {
			return false
		}
		return true
	}
	f.Walk(fn)
}

func doOtherShit() {
	fset := token.NewFileSet()
	rf, err := ioutil.ReadFile("lol/lol.go")
	if err != nil {
		panic(err)
	}
	f, err := parser.ParseFile(fset, "src.go", string(rf), 0)
	if err != nil {
		panic(err)
	}
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("self", fset, []*ast.File{f}, nil)
	if err != nil {
		panic(err)
	}

	// pkgscope := typeutil.Dependencies(pkg)

	for _, name := range pkg.Scope().Names() {
		obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if ok && obj != nil {
			// is a type alias
			if typ, ok := obj.Type().(*types.Named); ok {
				// is an alias to `int`
				if types.Typ[types.Int] == typ.Underlying() {
					// find all constant assignments
					for _, name := range pkg.Scope().Names() {
						if obj, ok := pkg.Scope().Lookup(name).(*types.Const); ok {
							if obj.Type() == typ {
								enumTypes[typ.String()] = append(enumTypes[typ.String()], obj.Name())
							}
						}
					}
				}
			}
		}
	}
}
