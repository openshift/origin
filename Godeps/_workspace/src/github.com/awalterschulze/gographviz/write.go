//Copyright 2013 Vastech SA (PTY) LTD
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package gographviz

import (
	"fmt"
	"github.com/awalterschulze/gographviz/ast"
)

type writer struct {
	*Graph
	writtenLocations map[string]bool
}

func newWriter(g *Graph) *writer {
	return &writer{g, make(map[string]bool)}
}

func appendAttrs(list ast.StmtList, attrs Attrs) ast.StmtList {
	for _, name := range attrs.SortedNames() {
		stmt := &ast.Attr{
			Field: ast.Id(name),
			Value: ast.Id(attrs[name]),
		}
		list = append(list, stmt)
	}
	return list
}

func (this *writer) newSubGraph(name string) *ast.SubGraph {
	sub := this.SubGraphs.SubGraphs[name]
	this.writtenLocations[sub.Name] = true
	s := &ast.SubGraph{}
	s.Id = ast.Id(sub.Name)
	s.StmtList = appendAttrs(s.StmtList, sub.Attrs)
	children := this.Relations.SortedChildren(name)
	for _, child := range children {
		s.StmtList = append(s.StmtList, this.newNodeStmt(child))
	}
	return s
}

func (this *writer) newNodeId(name string, port string) *ast.NodeId {
	node := this.Nodes.Lookup[name]
	return ast.MakeNodeId(node.Name, port)
}

func (this *writer) newNodeStmt(name string) *ast.NodeStmt {
	node := this.Nodes.Lookup[name]
	id := ast.MakeNodeId(node.Name, "")
	this.writtenLocations[node.Name] = true
	return &ast.NodeStmt{
		id,
		ast.PutMap(node.Attrs),
	}
}

func (this *writer) newLocation(name string, port string) ast.Location {
	if this.IsNode(name) {
		return this.newNodeId(name, port)
	} else if this.IsSubGraph(name) {
		if len(port) != 0 {
			panic(fmt.Sprintf("subgraph cannot have a port: %v", port))
		}
		return this.newSubGraph(name)
	}
	panic(fmt.Sprintf("%v is not a node or a subgraph", name))
}

func (this *writer) newEdgeStmt(edge *Edge) *ast.EdgeStmt {
	src := this.newLocation(edge.Src, edge.SrcPort)
	dst := this.newLocation(edge.Dst, edge.DstPort)
	stmt := &ast.EdgeStmt{
		Source: src,
		EdgeRHS: ast.EdgeRHS{
			&ast.EdgeRH{
				ast.EdgeOp(edge.Dir),
				dst,
			},
		},
		Attrs: ast.PutMap(edge.Attrs),
	}
	return stmt
}

func (this *writer) Write() *ast.Graph {
	t := &ast.Graph{}
	t.Strict = this.Strict
	t.Type = ast.GraphType(this.Directed)
	t.Id = ast.Id(this.Name)

	t.StmtList = appendAttrs(t.StmtList, this.Attrs)

	for _, edge := range this.Edges.Edges {
		t.StmtList = append(t.StmtList, this.newEdgeStmt(edge))
	}

	subGraphs := this.SubGraphs.Sorted()
	for _, s := range subGraphs {
		if _, ok := this.writtenLocations[s.Name]; !ok {
			t.StmtList = append(t.StmtList, this.newSubGraph(s.Name))
		}
	}

	nodes := this.Nodes.Sorted()
	for _, n := range nodes {
		if _, ok := this.writtenLocations[n.Name]; !ok {
			t.StmtList = append(t.StmtList, this.newNodeStmt(n.Name))
		}
	}

	return t
}

//Creates an Abstract Syntrax Tree from the Graph.
func (g *Graph) WriteAst() *ast.Graph {
	w := newWriter(g)
	return w.Write()
}

//Returns a DOT string representing the Graph.
func (g *Graph) String() string {
	return g.WriteAst().String()
}
