// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testgraphs

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

var _ graph.Graph = (*Grid)(nil)

func join(g ...string) string { return strings.Join(g, "\n") }

type node int64

func (n node) ID() int64 { return int64(n) }

func TestGrid(t *testing.T) {
	g := NewGrid(4, 4, false)

	got := g.String()
	want := join(
		"****",
		"****",
		"****",
		"****",
	)
	if got != want {
		t.Fatalf("unexpected grid rendering:\ngot: %q\nwant:%q", got, want)
	}

	var ops = []struct {
		r, c  int
		state bool
		want  string
	}{
		{
			r: 0, c: 1,
			state: true,
			want: join(
				"*.**",
				"****",
				"****",
				"****",
			),
		},
		{
			r: 0, c: 1,
			state: false,
			want: join(
				"****",
				"****",
				"****",
				"****",
			),
		},
		{
			r: 0, c: 1,
			state: true,
			want: join(
				"*.**",
				"****",
				"****",
				"****",
			),
		},
		{
			r: 0, c: 2,
			state: true,
			want: join(
				"*..*",
				"****",
				"****",
				"****",
			),
		},
		{
			r: 1, c: 2,
			state: true,
			want: join(
				"*..*",
				"**.*",
				"****",
				"****",
			),
		},
		{
			r: 2, c: 2,
			state: true,
			want: join(
				"*..*",
				"**.*",
				"**.*",
				"****",
			),
		},
		{
			r: 3, c: 2,
			state: true,
			want: join(
				"*..*",
				"**.*",
				"**.*",
				"**.*",
			),
		},
	}
	for _, test := range ops {
		g.Set(test.r, test.c, test.state)
		got := g.String()
		if got != test.want {
			t.Fatalf("unexpected grid rendering after set (%d, %d) open state to %t:\ngot: %q\nwant:%q",
				test.r, test.c, test.state, got, test.want)
		}
	}

	// Match the last state from the loop against the
	// explicit description of the grid.
	got = NewGridFrom(
		"*..*",
		"**.*",
		"**.*",
		"**.*",
	).String()
	want = g.String()
	if got != want {
		t.Fatalf("unexpected grid rendering from NewGridFrom:\ngot: %q\nwant:%q", got, want)
	}

	var paths = []struct {
		path     []graph.Node
		diagonal bool
		want     string
	}{
		{
			path:     nil,
			diagonal: false,
			want: join(
				"*..*",
				"**.*",
				"**.*",
				"**.*",
			),
		},
		{
			path:     []graph.Node{node(1), node(2), node(6), node(10), node(14)},
			diagonal: false,
			want: join(
				"*So*",
				"**o*",
				"**o*",
				"**G*",
			),
		},
		{
			path:     []graph.Node{node(1), node(6), node(10), node(14)},
			diagonal: false,
			want: join(
				"*S.*",
				"**!*",
				"**.*",
				"**.*",
			),
		},
		{
			path:     []graph.Node{node(1), node(6), node(10), node(14)},
			diagonal: true,
			want: join(
				"*S.*",
				"**o*",
				"**o*",
				"**G*",
			),
		},
		{
			path:     []graph.Node{node(1), node(5), node(9)},
			diagonal: false,
			want: join(
				"*S.*",
				"*!.*",
				"**.*",
				"**.*",
			),
		},
	}
	for _, test := range paths {
		g.AllowDiagonal = test.diagonal
		got, err := g.Render(test.path)
		errored := err != nil
		if bytes.Contains(got, []byte{'!'}) != errored {
			t.Fatalf("unexpected error return: got:%v want:%v", err, errors.New("grid: not a path in graph"))
		}
		if string(got) != test.want {
			t.Fatalf("unexpected grid path rendering for %v:\ngot: %q\nwant:%q", test.path, got, want)
		}
	}

	var coords = []struct {
		r, c int
		id   int64
	}{
		{r: 0, c: 0, id: 0},
		{r: 0, c: 3, id: 3},
		{r: 3, c: 0, id: 12},
		{r: 3, c: 3, id: 15},
	}
	for _, test := range coords {
		if id := g.NodeAt(test.r, test.c).ID(); id != test.id {
			t.Fatalf("unexpected ID for node at (%d, %d):\ngot: %d\nwant:%d", test.r, test.c, id, test.id)
		}
		if r, c := g.RowCol(test.id); r != test.r || c != test.c {
			t.Fatalf("unexpected row/col for node %d:\ngot: (%d, %d)\nwant:(%d, %d)", test.id, r, c, test.r, test.c)
		}
	}

	var reach = []struct {
		from     graph.Node
		diagonal bool
		to       []graph.Node
	}{
		{
			from:     node(0),
			diagonal: false,
			to:       nil,
		},
		{
			from:     node(2),
			diagonal: false,
			to:       []graph.Node{simple.Node(1), simple.Node(6)},
		},
		{
			from:     node(1),
			diagonal: false,
			to:       []graph.Node{simple.Node(2)},
		},
		{
			from:     node(1),
			diagonal: true,
			to:       []graph.Node{simple.Node(2), simple.Node(6)},
		},
	}
	for _, test := range reach {
		g.AllowDiagonal = test.diagonal
		got := graph.NodesOf(g.From(test.from.ID()))
		if !reflect.DeepEqual(got, test.to) {
			t.Fatalf("unexpected nodes from %d with allow diagonal=%t:\ngot: %v\nwant:%v",
				test.from, test.diagonal, got, test.to)
		}
	}
}
