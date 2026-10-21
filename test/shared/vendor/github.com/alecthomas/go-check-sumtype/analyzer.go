package gochecksumtype

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// checks exhaustiveness of sum type
// switch statements. Sum types are declared with a //sumtype:decl comment
// above a sealed interface type.
func newAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             "sumtype",
		Doc:              "check exhaustiveness of sum type switch statements",
		Run:              run,
		Flags:            newFlags(),
		FactTypes:        []analysis.Fact{new(sumTypeFact)},
		URL:              "",
		RunDespiteErrors: false,
		Requires:         nil,
		ResultType:       nil,
	}
}

var Analyzer = newAnalyzer()

func run(pass *analysis.Pass) (any, error) {
	cfg := cfgFromFlags(pass.Analyzer.Flags)

	decls, err := findSumTypeDecls(pass.Fset, pass.Files)
	if err != nil {
		return nil, err
	}

	defs, defErrs := findSumTypeDefs(pass, decls)
	for _, e := range defErrs {
		pass.Reportf(e.(Error).Pos(), "%s", e.Error())
	}

	// Export facts so downstream packages can check exhaustiveness
	// against sum types defined here.
	fact := &sumTypeFact{Definitions: make([]sumTypeDefinitionFact, 0, len(defs))}
	for _, def := range defs {
		variantNames := make([]string, len(def.Variants))
		for i, v := range def.Variants {
			variantNames[i] = v.Name()
		}
		fact.Definitions = append(fact.Definitions, sumTypeDefinitionFact{
			TypeName: def.Decl.TypeName,
			Variants: variantNames,
		})
	}
	if len(fact.Definitions) > 0 {
		pass.ExportPackageFact(fact)
	}

	// Import sum type facts from dependencies.
	for _, pkg := range pass.Pkg.Imports() {
		var fact sumTypeFact
		if pass.ImportPackageFact(pkg, &fact) {
			defs = append(defs, factToTypeDefs(pass.Fset, pkg, &fact)...)
		}
	}
	// Check exhaustiveness for all type switches in this package.
	for _, astfile := range pass.Files {
		for _, err := range checkFile(pass, astfile, defs, cfg) {
			pass.Reportf(err.(Error).Pos(), "%s", err.Error())
		}
	}

	return nil, nil
}

// factToTypeDefs reconstructs [sumTypeDef] values from an imported fact.
func factToTypeDefs(fset *token.FileSet, pkg *types.Package, fact *sumTypeFact) []sumTypeDef {
	defs := make([]sumTypeDef, 0, len(fact.Definitions))
	for _, definition := range fact.Definitions {
		obj := pkg.Scope().Lookup(definition.TypeName)
		if obj == nil {
			continue
		}
		iface, ok := obj.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		location := pkg.Path() + "." + definition.TypeName
		if position := fset.Position(obj.Pos()); position.IsValid() {
			location = position.String()
		}
		def := sumTypeDef{
			Decl: sumTypeDecl{
				TypeName: definition.TypeName,
				Pos:      token.NoPos,
				Location: location,
			},
			Ty:       iface,
			Variants: make([]types.Object, 0, len(definition.Variants)),
		}
		for _, name := range definition.Variants {
			vObj := pkg.Scope().Lookup(name)
			if vObj != nil {
				def.Variants = append(def.Variants, vObj)
			}
		}
		defs = append(defs, def)
	}
	return defs
}
