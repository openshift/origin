// This file is dual licensed under CC0 and The gonum license.
//
// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copyright ©2017 Robin Eklind.
// This file is made available under a Creative Commons CC0 1.0
// Universal Public Domain Dedication.

package ast_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

func TestParseFile(t *testing.T) {
	golden := []struct {
		in  string
		out string
	}{
		{in: "../internal/testdata/empty.dot"},
		{in: "../internal/testdata/graph.dot"},
		{in: "../internal/testdata/digraph.dot"},
		{in: "../internal/testdata/strict.dot"},
		{in: "../internal/testdata/multi.dot"},
		{in: "../internal/testdata/named_graph.dot"},
		{in: "../internal/testdata/node_stmt.dot"},
		{in: "../internal/testdata/edge_stmt.dot"},
		{in: "../internal/testdata/attr_stmt.dot"},
		{in: "../internal/testdata/attr.dot"},
		{
			in:  "../internal/testdata/subgraph.dot",
			out: "../internal/testdata/subgraph.golden",
		},
		{
			in:  "../internal/testdata/semi.dot",
			out: "../internal/testdata/semi.golden",
		},
		{
			in:  "../internal/testdata/empty_attr.dot",
			out: "../internal/testdata/empty_attr.golden",
		},
		{
			in:  "../internal/testdata/attr_lists.dot",
			out: "../internal/testdata/attr_lists.golden",
		},
		{
			in:  "../internal/testdata/attr_sep.dot",
			out: "../internal/testdata/attr_sep.golden",
		},
		{in: "../internal/testdata/subgraph_vertex.dot"},
		{in: "../internal/testdata/port.dot"},
	}
	for _, g := range golden {
		file, err := dot.ParseFile(g.in)
		if err != nil {
			t.Errorf("%q: unable to parse file; %v", g.in, err)
			continue
		}
		// If no output path is specified, the input is already golden.
		out := g.in
		if len(g.out) > 0 {
			out = g.out
		}
		buf, err := ioutil.ReadFile(out)
		if err != nil {
			t.Errorf("%q: unable to read file; %v", g.in, err)
			continue
		}
		got := file.String()
		// Remove trailing newline.
		want := string(bytes.TrimSpace(buf))
		if got != want {
			t.Errorf("%q: graph mismatch; expected %q, got %q", g.in, want, got)
		}
	}
}

// Verify that all statements implement the Stmt interface.
var (
	_ ast.Stmt = (*ast.NodeStmt)(nil)
	_ ast.Stmt = (*ast.EdgeStmt)(nil)
	_ ast.Stmt = (*ast.AttrStmt)(nil)
	_ ast.Stmt = (*ast.Attr)(nil)
	_ ast.Stmt = (*ast.Subgraph)(nil)
)

// Verify that all vertices implement the Vertex interface.
var (
	_ ast.Vertex = (*ast.Node)(nil)
	_ ast.Vertex = (*ast.Subgraph)(nil)
)
