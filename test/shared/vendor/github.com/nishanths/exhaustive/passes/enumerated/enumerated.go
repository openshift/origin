package enumerated

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// control whether additional facts are exported,
// for use by tests only.
var testingExtraFacts = false

var Analyzer = &analysis.Analyzer{
	Name:       "enumerated",
	Doc:        "find declarations of enumerated types and enumerated constants",
	Run:        run,
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: reflect.TypeFor[Result](),
	FactTypes:  []analysis.Fact{new(isEnumeratedFact)},
}

func init() {
	Analyzer.Flags.BoolVar(&fIota, "i", fIota, "only consider constants declared using iota")
	Analyzer.Flags.BoolVar(&fNoBitwise, "B", fNoBitwise, "ignore constants declared using bitwise operators")
	Analyzer.Flags.BoolVar(&fPkgLevel, "p", fPkgLevel, "only consider types and constants with top-level declarations")
}

var (
	fIota      = false
	fNoBitwise = false
	fPkgLevel  = false
)

func resetFlags() {
	fIota = false
	fNoBitwise = false
	fPkgLevel = false
}

type Result map[*types.Named][]*types.Const

func run(pass *analysis.Pass) (any, error) {
	find(pass, &options{
		requirePkgLevel: fPkgLevel,
		requireIota:     fIota,
		rejectBitwise:   fNoBitwise,
	})

	var cs []*types.Const
	for _, f := range pass.AllObjectFacts() {
		switch ff := f.Fact.(type) {
		case *isEnumeratedFact:
			switch ff.Kind {
			case "constant":
				cs = append(cs, f.Object.(*types.Const))
			}
		}
	}

	m := make(map[*types.Named][]*types.Const)
	for _, c := range cs {
		t := c.Type().(*types.Named)
		m[t] = append(m[t], c)
	}
	return Result(m), nil
}

type options struct {
	requirePkgLevel bool
	requireIota     bool
	rejectBitwise   bool
}

var bitwiseOps = map[token.Token]struct{}{
	token.AND:     {},
	token.OR:      {},
	token.XOR:     {}, // both binary and unary
	token.SHL:     {},
	token.SHR:     {},
	token.AND_NOT: {},
}

func find(pass *analysis.Pass, opts *options) {
	in := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var cs []*types.Const
	constValues := make(map[*types.Const]ast.Expr) // value expressions for all const declarations

	if testingExtraFacts {
		in.Preorder([]ast.Node{&ast.GenDecl{}}, func(n ast.Node) {
			genDecl := n.(*ast.GenDecl)
			if genDecl.Tok != token.TYPE {
				return
			}
			for _, spec := range genDecl.Specs {
				obj := pass.TypesInfo.Defs[spec.(*ast.TypeSpec).Name]
				_, ok := permittedEnumeratedType(obj.Type())
				if !ok {
					continue
				}
				if opts.requirePkgLevel && !isPkgLevel(obj) {
					continue
				}
				pass.ExportObjectFact(obj, &permittedTypeFact{})
			}
		})
	}

	in.Preorder([]ast.Node{&ast.GenDecl{}}, func(n ast.Node) {
		decl := n.(*ast.GenDecl)
		if decl.Tok != token.CONST {
			return
		}

		var last []ast.Expr
		for _, spec := range decl.Specs {
			spec := spec.(*ast.ValueSpec)

			// Go spec: "an empty list is equivalent to
			// the textual substitution of the first
			// preceding non-empty expression list ...".
			var values []ast.Expr
			if spec.Values == nil {
				values = last // implicit repetition
			} else {
				values, last = spec.Values, spec.Values
			}
			assert(len(spec.Names) == len(values))

			for i := 0; i < len(spec.Names); i++ {
				obj := pass.TypesInfo.Defs[spec.Names[i]]
				c, ok := permittedEnumeratedConstant(obj, pass.TypesInfo)
				if !ok {
					continue
				}
				if opts.requirePkgLevel && !isPkgLevel(c) {
					continue
				}
				constValues[c] = values[i]
				cs = append(cs, c)
			}
		}
	})

	if opts.requireIota {
		cs = slices.DeleteFunc(cs, func(c *types.Const) bool {
			return !constExprUsesIota(pass, constValues[c], constValues)
		})
	}

	if opts.rejectBitwise {
		cs = slices.DeleteFunc(cs, func(c *types.Const) bool {
			return constExprUsesOp(pass, bitwiseOps, constValues[c], constValues)
		})
	}

	// Note that the logic in this function implies that each
	// enumerated type in m has at least one enumerated
	// constant.
	// The implementation matches the behavior documented in
	// the package comment.
	//
	m := make(map[*types.Named][]*types.Const)
	for _, c := range cs {
		t := c.Type().(*types.Named)
		m[t] = append(m[t], c)
	}

	for t, cs := range m {
		pass.ExportObjectFact(t.Obj(), &isEnumeratedFact{1, "type"})
		for _, c := range cs {
			pass.ExportObjectFact(c, &isEnumeratedFact{1, "constant"})
		}
	}

	if testingExtraFacts {
		for t, cs := range m {
			pass.ExportObjectFact(t.Obj(), &enumeratedTypeFact{slicemap(cs, toConstant)})
			for _, c := range cs {
				pass.ExportObjectFact(c, &enumeratedConstantFact{t.Obj().Name()})
			}
		}
	}
}

func permittedEnumeratedType(t types.Type) (*types.Named, bool) {
	// As of go1.27, the Go spec and package go/types vary in
	// terminology.
	// For our purposes, an enumerated type must be (in Go
	// spec terminology) a defined type. It must not be one
	// of the predeclared types[*]. This concept is
	// represented by *types.Named in go/types.
	//
	// [*] Note: parts of the Go spec say that bool, the
	// predeclared numeric types (except byte and rune), and
	// string are defined types.
	//
	// Note: *types.Named in packages go/types is different
	// from the term "named type" as used in the Go spec.
	n, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}
	// The type definition must not specify type parameters.
	if n.TypeParams() != nil {
		return nil, false
	}
	// The underlying type must be a numeric or string type.
	basic, ok := n.Underlying().(*types.Basic)
	if !ok {
		return nil, false
	}
	if info := basic.Info(); info&(types.IsNumeric|types.IsString) == 0 {
		return nil, false
	}
	return n, true
}

func permittedEnumeratedConstant(obj types.Object, info *types.Info) (*types.Const, bool) {
	c, ok := obj.(*types.Const)
	if !ok {
		return nil, false
	}
	// The type of the constant firstly must be a permitted
	// enumerated type.
	t, ok := permittedEnumeratedType(c.Type())
	if !ok {
		return nil, false
	}
	if isBlankIdentifier(obj) {
		// Note that blank identifier objects have a nil parent
		// scope in package go/types. This would be relevant for
		// the upcoming scope check if this early return did not
		// exist.
		return nil, false
	}
	// To be considered an enumerated constant of a given
	// enumerated type, the type declaration and the constant
	// declaration must be in the same block.
	//
	// The above requirement implies that a enumerated
	// constant of a given enumerated type must be declared
	// in the same package as the type.
	if t.Obj().Parent() != c.Parent() {
		return nil, false
	}
	return c, true
}

func isPkgLevel(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Pkg().Scope().Lookup(obj.Name()) == obj
}

func isBlankIdentifier(obj types.Object) bool {
	// Note: go/types/decl.go performs a direct comparison
	// like this. It appears that this is the canonical way
	// to check for the blank identifier.
	return obj.Name() == "_"
}

var universeIota = types.Universe.Lookup("iota")

func constExprUsesIota(pass *analysis.Pass, expr ast.Expr, exprs map[*types.Const]ast.Expr) bool {
	checkobj := func(obj types.Object) bool {
		if obj.Pkg() == pass.Pkg {
			c, ok := obj.(*types.Const)
			if !ok {
				return false
			}
			e, ok := exprs[c]
			if !ok {
				return false
			}
			return constExprUsesIota(pass, e, exprs)
		}
		f := new(isEnumeratedFact)
		return pass.ImportObjectFact(obj, f) && f.Kind == "constant"
	}

	for n := range ast.Preorder(expr) {
		switch n := n.(type) {
		case *ast.Ident:
			if obj := pass.TypesInfo.Uses[n]; obj != nil && (obj == universeIota || checkobj(obj)) {
				return true
			}
		}
	}
	return false
}

func constExprUsesOp(pass *analysis.Pass, ops map[token.Token]struct{}, expr ast.Expr, exprs map[*types.Const]ast.Expr) bool {
	checkobj := func(obj types.Object) bool {
		if obj.Pkg() == pass.Pkg {
			c, ok := obj.(*types.Const)
			if !ok {
				return false
			}
			e, ok := exprs[c]
			if !ok {
				return false
			}
			return constExprUsesOp(pass, ops, e, exprs)
		}
		f := new(isEnumeratedFact)
		return pass.ImportObjectFact(obj, f) && f.Kind == "constant"
	}

	for n := range ast.Preorder(expr) {
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if _, ok := ops[n.Op]; ok {
				return true
			}
		case *ast.UnaryExpr:
			if _, ok := ops[n.Op]; ok {
				return true
			}
		case *ast.Ident:
			if obj := pass.TypesInfo.Uses[n]; obj != nil && checkobj(obj) {
				return true
			}
		}
	}
	return false
}

func slicemap[S ~[]E, E, F any](s S, fn func(E) F) []F {
	var ret []F
	if s != nil {
		ret = make([]F, len(s))
		for i := range s {
			ret[i] = fn(s[i])
		}
	}
	return ret
}

func assert(x bool) {
	if !x {
		panic("assertion failed")
	}
}

var _ analysis.Fact = (*isEnumeratedFact)(nil)

type isEnumeratedFact struct {
	Version int
	Kind    string // one of: "type", "constant"
}

func (*isEnumeratedFact) AFact()           {}
func (f *isEnumeratedFact) String() string { return "enumerated" }

// The following fact types are emitted only in tests.

var _ analysis.Fact = (*enumeratedTypeFact)(nil)
var _ analysis.Fact = (*permittedTypeFact)(nil)
var _ analysis.Fact = (*enumeratedConstantFact)(nil)

type enumeratedTypeFact struct {
	// The enumerated constants of this type.
	Elements []constant
}

type constant struct {
	Name  string
	Value string
}

func toConstant(c *types.Const) constant {
	return constant{
		Name:  c.Name(),
		Value: c.Val().ExactString(),
	}
}

func (*enumeratedTypeFact) AFact() {}

func (f *enumeratedTypeFact) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "elements:")
	for i, c := range f.Elements {
		fmt.Fprintf(&buf, "%s = %s", c.Name, c.Value)
		if i != len(f.Elements)-1 {
			fmt.Fprint(&buf, ", ")
		}
	}
	return buf.String()
}

type enumeratedConstantFact struct {
	Type string
}

func (f *enumeratedConstantFact) AFact()         {}
func (f *enumeratedConstantFact) String() string { return fmt.Sprintf("elementof:%s", f.Type) }

type permittedTypeFact struct{}

func (*permittedTypeFact) AFact()         {}
func (*permittedTypeFact) String() string { return "permittedtype" }
