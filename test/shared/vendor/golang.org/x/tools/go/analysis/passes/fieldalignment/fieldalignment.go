// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fieldalignment defines an Analyzer that detects structs that would use less
// memory if their fields were sorted.
package fieldalignment

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/internal/astutil"
)

const Doc = `find structs that would use less memory if their fields were sorted

This analyzer finds structs that can be rearranged to use less memory, and provides
a suggested edit with the most compact order.

Note that there are two different diagnostics reported. One checks struct size,
and the other reports "pointer bytes" used. Pointer bytes is how many bytes of the
object that the garbage collector has to potentially scan for pointers, for example:

	struct { uint32; string }

have 16 pointer bytes because the garbage collector has to scan up through the string's
inner pointer.

	struct { string; *uint32 }

has 24 pointer bytes because it has to scan further through the *uint32.

	struct { string; uint32 }

has 8 because it can stop immediately after the string pointer.

Be aware that the most compact order is not always the most efficient.
In rare cases it may cause two variables each updated by its own goroutine
to occupy the same CPU cache line, inducing a form of memory contention
known as "false sharing" that slows down both goroutines.

Unlike most analyzers, which report likely mistakes, the diagnostics
produced by fieldanalyzer very rarely indicate a significant problem,
so the analyzer is not included in typical suites such as vet or
gopls. Use this standalone command to run it on your code:

   $ go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
   $ fieldalignment [packages]

`

var Analyzer = &analysis.Analyzer{
	Name:     "fieldalignment",
	Doc:      Doc,
	URL:      "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/fieldalignment",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	for curStruct := range inspect.Root().Preorder((*ast.StructType)(nil)) {
		s := curStruct.Node().(*ast.StructType)
		// For every named struct defined as "type Name struct { ... }",
		// the *ast.StructType node has a parent *ast.TypeSpec,
		// which contains the struct's name in its Name field.
		name := "struct" // (anonymous)
		if spec, ok := curStruct.Parent().Node().(*ast.TypeSpec); ok {
			name = spec.Name.Name
		}
		fieldalignment(pass, s, name)
	}

	return nil, nil
}

func fieldalignment(pass *analysis.Pass, node *ast.StructType, name string) {
	var (
		sizes = &gcSizes{
			wordSize: pass.TypesSizes.Sizeof(types.Typ[types.UnsafePointer]),
			maxAlign: pass.TypesSizes.Alignof(types.Typ[types.UnsafePointer]),
		}

		typ              = pass.TypesInfo.TypeOf(node).(*types.Struct)
		optimal, indexes = optimalOrder(typ, sizes)

		actualSize = sizes.sizeof(typ)
		actualPtrs = sizes.ptrdata(typ)

		optimalSize = sizes.sizeof(optimal)
		optimalPtrs = sizes.ptrdata(optimal)
	)

	var message strings.Builder
	if actualSize != optimalSize {
		// Struct could be smaller.
		// TODO(adonovan): IMHO the criterion should be "significantly smaller".
		fmt.Fprintf(&message, "%s has size %d", name, actualSize)
		actualClass := classSize(actualSize)
		if actualClass == -1 {
			actualClass = actualSize
			fmt.Fprint(&message, " (uses global allocator)")
		} else if actualClass != actualSize {
			fmt.Fprintf(&message, " (allocator size class %d)", actualClass)
		}

		fmt.Fprintf(&message, " but the optimal size is %d", optimalSize)
		optimalClass := classSize(optimalSize)
		if optimalClass == -1 {
			optimalClass = optimalSize
		} else if optimalClass != optimalSize {
			fmt.Fprintf(&message, " (allocator size class %d)", optimalClass)
		}

		if wastage := actualClass - optimalClass; wastage > 0 {
			fmt.Fprintf(&message, " leading to a waste of %d bytes (%d%%)", wastage, wastage*100/actualClass)
		}
	} else if actualPtrs != optimalPtrs {
		// Struct could place pointers more efficiently for GC marking.
		fmt.Fprintf(&message, "%s has %d leading bytes of pointer data but optimal value is %d", name, actualPtrs, optimalPtrs)
	} else {
		// Already optimal order.
		return
	}

	// Analyzers borrow syntax tree; they do not own them and must modify them.
	// This Clone operation is a quick fix to the data race introduced
	// in CL 278872 by the clearing of the Comment and Doc fields below.
	node = astutil.CloneNode(node)

	// Flatten the ast node since it could have multiple field names per list item while
	// *types.Struct only have one item per field.
	// TODO: Preserve multi-named fields instead of flattening.
	var flat []*ast.Field
	for _, f := range node.Fields.List {
		// TODO: Preserve comment, for now get rid of them.
		//       See https://github.com/golang/go/issues/20744
		f.Comment = nil
		f.Doc = nil
		if len(f.Names) <= 1 {
			flat = append(flat, f)
			continue
		}
		for _, name := range f.Names {
			flat = append(flat, &ast.Field{
				Names: []*ast.Ident{name},
				Type:  f.Type,
			})
		}
	}

	// Sort fields according to the optimal order.
	var reordered []*ast.Field
	for _, index := range indexes {
		reordered = append(reordered, flat[index])
	}

	newStr := &ast.StructType{
		Fields: &ast.FieldList{
			List: reordered,
		},
	}

	// Write the newly aligned struct node to get the content for suggested fixes.
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), newStr); err != nil {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     node.Pos(),
		End:     node.Pos() + token.Pos(len("struct")),
		Message: message.String(),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Rearrange fields",
			TextEdits: []analysis.TextEdit{{
				Pos:     node.Pos(),
				End:     node.End(),
				NewText: buf.Bytes(),
			}},
		}},
	})
}

func optimalOrder(str *types.Struct, sizes *gcSizes) (*types.Struct, []int) {
	nf := str.NumFields()

	type elem struct {
		index   int
		alignof int64
		sizeof  int64
		ptrdata int64
	}

	elems := make([]elem, nf)
	for i := range nf {
		field := str.Field(i)
		ft := field.Type()
		elems[i] = elem{
			i,
			sizes.alignof(ft),
			sizes.sizeof(ft),
			sizes.ptrdata(ft),
		}
	}

	sort.Slice(elems, func(i, j int) bool {
		ei := &elems[i]
		ej := &elems[j]

		// Place zero sized objects before non-zero sized objects.
		zeroi := ei.sizeof == 0
		zeroj := ej.sizeof == 0
		if zeroi != zeroj {
			return zeroi
		}

		// Next, place more tightly aligned objects before less tightly aligned objects.
		if ei.alignof != ej.alignof {
			return ei.alignof > ej.alignof
		}

		// Place pointerful objects before pointer-free objects.
		noptrsi := ei.ptrdata == 0
		noptrsj := ej.ptrdata == 0
		if noptrsi != noptrsj {
			return noptrsj
		}

		if !noptrsi {
			// If both have pointers...

			// ... then place objects with less trailing
			// non-pointer bytes earlier. That is, place
			// the field with the most trailing
			// non-pointer bytes at the end of the
			// pointerful section.
			traili := ei.sizeof - ei.ptrdata
			trailj := ej.sizeof - ej.ptrdata
			if traili != trailj {
				return traili < trailj
			}
		}

		// Lastly, order by size.
		if ei.sizeof != ej.sizeof {
			return ei.sizeof > ej.sizeof
		}

		return false
	})

	fields := make([]*types.Var, nf)
	indexes := make([]int, nf)
	for i, e := range elems {
		fields[i] = str.Field(e.index)
		indexes[i] = e.index
	}
	return types.NewStruct(fields, nil), indexes
}

// gcSizes implements cmd/compile layout rules, providing ptrdata (GC
// scanning limits) and trailing zero-size field padding not available
// in [types.Sizes].

type gcSizes struct {
	wordSize int64
	maxAlign int64
}

func (s *gcSizes) alignof(T types.Type) int64 {
	// For arrays and structs, alignment is defined in terms
	// of alignment of the elements and fields, respectively.
	switch t := T.Underlying().(type) {
	case *types.Array:
		// spec: "For a variable x of array type: unsafe.Alignof(x)
		// is the same as unsafe.Alignof(x[0]), but at least 1."
		return s.alignof(t.Elem())
	case *types.Struct:
		// spec: "For a variable x of struct type: unsafe.Alignof(x)
		// is the largest of the values unsafe.Alignof(x.f) for each
		// field f of x, but at least 1."
		max := int64(1)
		for i, nf := 0, t.NumFields(); i < nf; i++ {
			if a := s.alignof(t.Field(i).Type()); a > max {
				max = a
			}
		}
		return max
	}
	a := s.sizeof(T) // may be 0
	// spec: "For a variable x of any type: unsafe.Alignof(x) is at least 1."
	if a < 1 {
		return 1
	}
	if a > s.maxAlign {
		return s.maxAlign
	}
	return a
}

var basicSizes = [...]byte{
	types.Bool:       1,
	types.Int8:       1,
	types.Int16:      2,
	types.Int32:      4,
	types.Int64:      8,
	types.Uint8:      1,
	types.Uint16:     2,
	types.Uint32:     4,
	types.Uint64:     8,
	types.Float32:    4,
	types.Float64:    8,
	types.Complex64:  8,
	types.Complex128: 16,
}

func (s *gcSizes) sizeof(T types.Type) int64 {
	switch t := T.Underlying().(type) {
	case *types.Basic:
		k := t.Kind()
		if int(k) < len(basicSizes) {
			if s := basicSizes[k]; s > 0 {
				return int64(s)
			}
		}
		if k == types.String {
			return s.wordSize * 2
		}
	case *types.Array:
		return t.Len() * s.sizeof(t.Elem())
	case *types.Slice:
		return s.wordSize * 3
	case *types.Struct:
		nf := t.NumFields()
		if nf == 0 {
			return 0
		}

		var o int64
		max := int64(1)
		for i := range nf {
			ft := t.Field(i).Type()
			a, sz := s.alignof(ft), s.sizeof(ft)
			if a > max {
				max = a
			}
			if i == nf-1 && sz == 0 && o != 0 {
				sz = 1
			}
			o = align(o, a) + sz
		}
		return align(o, max)
	case *types.Interface:
		return s.wordSize * 2
	}
	return s.wordSize // catch-all
}

// align returns the smallest y >= x such that y % a == 0.
func align(x, a int64) int64 {
	y := x + a - 1
	return y - y%a
}

func (s *gcSizes) ptrdata(T types.Type) int64 {
	switch t := T.Underlying().(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.String, types.UnsafePointer:
			return s.wordSize
		}
		return 0
	case *types.Chan, *types.Map, *types.Pointer, *types.Signature, *types.Slice:
		return s.wordSize
	case *types.Interface:
		return 2 * s.wordSize
	case *types.Array:
		n := t.Len()
		if n == 0 {
			return 0
		}
		a := s.ptrdata(t.Elem())
		if a == 0 {
			return 0
		}
		z := s.sizeof(t.Elem())
		return (n-1)*z + a
	case *types.Struct:
		nf := t.NumFields()
		if nf == 0 {
			return 0
		}

		var o, p int64
		for i := range nf {
			ft := t.Field(i).Type()
			a, sz := s.alignof(ft), s.sizeof(ft)
			fp := s.ptrdata(ft)
			o = align(o, a)
			if fp != 0 {
				p = o + fp
			}
			o += sz
		}
		return p
	}

	panic("impossible")
}

// Code below based on tools/gopls/internal/golang/hover.go

// classSize reports the size class for a struct of the specified size, or -1 if unknown.
// See GOROOT/src/runtime/msize.go for details.
func classSize(size int64) int64 {
	if size > 1<<15 {
		return -1 // avoid allocation
	}
	// We assume that bytes.Clone doesn't trim,
	// and reports the underlying size class
	return int64(cap(bytes.Clone(make([]byte, size))))
}
