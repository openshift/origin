package finder

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name:       "finder",
	Doc:        "find various language entities",
	Run:        run,
	ResultType: reflect.TypeFor[*Result](),
	FactTypes:  []analysis.Fact{new(isTypeDecl)},
}

type Result struct {
	// TypeDecls is the set of type name symbols seen in top-level type
	// declarations. The following forms of type declarations are
	// considered: type definition, alias declaration.
	// The Type() of the key is either a *types.Alias or *types.Named.
	TypeDecls map[*types.TypeName]struct{}
}

func run(pass *analysis.Pass) (any, error) {
	findTypeDecls(pass)

	allFacts := pass.AllObjectFacts()
	result := &Result{
		TypeDecls: make(map[*types.TypeName]struct{}, len(allFacts)),
	}
	for _, f := range allFacts {
		switch f.Fact.(type) {
		case *isTypeDecl:
			tn := f.Object.(*types.TypeName)
			result.TypeDecls[tn] = struct{}{}
		}
	}
	return result, nil
}

func findTypeDecls(pass *analysis.Pass) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				obj := pass.TypesInfo.Defs[spec.(*ast.TypeSpec).Name]
				tn, ok := obj.(*types.TypeName)
				if !ok {
					continue
				}
				switch tn.Type().(type) {
				case *types.Alias:
					pass.ExportObjectFact(tn, &isTypeDecl{Version: 1})
				case *types.Named:
					pass.ExportObjectFact(tn, &isTypeDecl{Version: 1})
				}
			}
		}
	}
}

var _ analysis.Fact = (*isTypeDecl)(nil)

type isTypeDecl struct{ Version int }

func (*isTypeDecl) AFact()           {}
func (f *isTypeDecl) String() string { return "typedecl" }
