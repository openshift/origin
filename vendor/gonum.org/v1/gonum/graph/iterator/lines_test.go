// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iterator_test

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

type line struct{ f, t int }

func (l line) From() graph.Node         { return simple.Node(l.f) }
func (l line) To() graph.Node           { return simple.Node(l.t) }
func (l line) ReversedLine() graph.Line { return line{f: l.t, t: l.f} }
func (l line) ID() int64                { return 1 }

var orderedLinesTests = []struct {
	lines []graph.Line
}{
	{lines: nil},
	{lines: []graph.Line{line{f: 1, t: 2}}},
	{lines: []graph.Line{line{f: 1, t: 2}, line{f: 2, t: 3}, line{f: 3, t: 4}, line{f: 4, t: 5}}},
	{lines: []graph.Line{line{f: 5, t: 4}, line{f: 4, t: 3}, line{f: 3, t: 2}, line{f: 2, t: 1}}},
}

func TestOrderedLinesIterate(t *testing.T) {
	for _, test := range orderedLinesTests {
		it := iterator.NewOrderedLines(test.lines)
		for i := 0; i < 2; i++ {
			if it.Len() != len(test.lines) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.lines))
			}
			var got []graph.Line
			for it.Next() {
				got = append(got, it.Line())
			}
			want := test.lines
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

func TestOrderedLinesSlice(t *testing.T) {
	for _, test := range orderedLinesTests {
		it := iterator.NewOrderedLines(test.lines)
		for i := 0; i < 2; i++ {
			got := it.LineSlice()
			want := test.lines
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

type weightedLine struct {
	f, t int
	w    float64
}

func (l weightedLine) From() graph.Node         { return simple.Node(l.f) }
func (l weightedLine) To() graph.Node           { return simple.Node(l.t) }
func (l weightedLine) ReversedLine() graph.Line { l.f, l.t = l.t, l.f; return l }
func (l weightedLine) Weight() float64          { return l.w }
func (l weightedLine) ID() int64                { return 1 }

var orderedWeightedLinesTests = []struct {
	lines []graph.WeightedLine
}{
	{lines: nil},
	{lines: []graph.WeightedLine{weightedLine{f: 1, t: 2, w: 1}}},
	{lines: []graph.WeightedLine{weightedLine{f: 1, t: 2, w: 1}, weightedLine{f: 2, t: 3, w: 2}, weightedLine{f: 3, t: 4, w: 3}, weightedLine{f: 4, t: 5, w: 4}}},
	{lines: []graph.WeightedLine{weightedLine{f: 5, t: 4, w: 4}, weightedLine{f: 4, t: 3, w: 3}, weightedLine{f: 3, t: 2, w: 2}, weightedLine{f: 2, t: 1, w: 1}}},
}

func TestOrderedWeightedLinesIterate(t *testing.T) {
	for _, test := range orderedWeightedLinesTests {
		it := iterator.NewOrderedWeightedLines(test.lines)
		for i := 0; i < 2; i++ {
			if it.Len() != len(test.lines) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.lines))
			}
			var got []graph.WeightedLine
			for it.Next() {
				got = append(got, it.WeightedLine())
			}
			want := test.lines
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

func TestOrderedWeightedLinesSlice(t *testing.T) {
	for _, test := range orderedWeightedLinesTests {
		it := iterator.NewOrderedWeightedLines(test.lines)
		for i := 0; i < 2; i++ {
			got := it.WeightedLineSlice()
			want := test.lines
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}
