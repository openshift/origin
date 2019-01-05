// This file is dual licensed under CC0 and The gonum license.
//
// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copyright ©2017 Robin Eklind.
// This file is made available under a Creative Commons CC0 1.0
// Universal Public Domain Dedication.

package parser_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"gonum.org/v1/gonum/graph/formats/dot"
)

func TestParseFile(t *testing.T) {
	golden := []struct {
		in  string
		out string
	}{
		{in: "../testdata/empty.dot"},
		{in: "../testdata/graph.dot"},
		{in: "../testdata/digraph.dot"},
		{in: "../testdata/strict.dot"},
		{in: "../testdata/multi.dot"},
		{in: "../testdata/named_graph.dot"},
		{in: "../testdata/node_stmt.dot"},
		{in: "../testdata/edge_stmt.dot"},
		{in: "../testdata/attr_stmt.dot"},
		{in: "../testdata/attr.dot"},
		{
			in:  "../testdata/subgraph.dot",
			out: "../testdata/subgraph.golden",
		},
		{
			in:  "../testdata/semi.dot",
			out: "../testdata/semi.golden",
		},
		{
			in:  "../testdata/empty_attr.dot",
			out: "../testdata/empty_attr.golden",
		},
		{
			in:  "../testdata/attr_lists.dot",
			out: "../testdata/attr_lists.golden",
		},
		{
			in:  "../testdata/attr_sep.dot",
			out: "../testdata/attr_sep.golden",
		},
		{in: "../testdata/subgraph_vertex.dot"},
		{in: "../testdata/port.dot"},
		{in: "../testdata/quoted_id.dot"},
		{
			in:  "../testdata/backslash_newline_id.dot",
			out: "../testdata/backslash_newline_id.golden",
		},
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
			t.Errorf("%q: graph mismatch; expected `%s`, got `%s`", g.in, want, got)
		}
	}
}

func TestParseError(t *testing.T) {
	golden := []struct {
		path string
		want string
	}{
		{
			path: "../testdata/error.dot",
			want: `Error in S30: INVALID(0,~), Pos(offset=13, line=2, column=7), expected one of: { } graphx ; -- -> node edge [ = subgraph : id `,
		},
	}
	for _, g := range golden {
		_, err := dot.ParseFile(g.path)
		if err == nil {
			t.Errorf("%q: expected error, got nil", g.path)
			continue
		}
		got := err.Error()
		if got != g.want {
			t.Errorf("%q: error mismatch; expected `%v`, got `%v`", g.path, g.want, got)
			continue
		}
	}
}
