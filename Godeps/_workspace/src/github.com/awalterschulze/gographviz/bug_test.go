package gographviz

import (
	"github.com/awalterschulze/gographviz/ast"
	"github.com/awalterschulze/gographviz/parser"
	"testing"
)

type bugSubGraphWorldVisitor struct {
	t     *testing.T
	found bool
}

func (this *bugSubGraphWorldVisitor) Visit(v ast.Elem) ast.Visitor {
	edge, ok := v.(ast.EdgeStmt)
	if !ok {
		return this
	}
	if edge.Source.GetId().String() != "2" {
		return this
	}
	dst := edge.EdgeRHS[0].Destination
	if _, ok := dst.(*ast.SubGraph); !ok {
		this.t.Fatalf("2 -> Not SubGraph")
	} else {
		this.found = true
	}
	return this
}

func TestBugSubGraphWorld(t *testing.T) {
	g := analtest(t, "world.gv.txt")
	st, err := parser.ParseString(g.String())
	check(t, err)
	s := &bugSubGraphWorldVisitor{
		t: t,
	}
	st.Walk(s)
	if !s.found {
		t.Fatalf("2 -> SubGraph not found")
	}
}
