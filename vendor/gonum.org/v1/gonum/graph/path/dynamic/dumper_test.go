// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dynamic

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"gonum.org/v1/gonum/graph/path/internal/testgraphs"
	"gonum.org/v1/gonum/graph/simple"
)

// dumper implements a grid D* Lite statistics dump.
type dumper struct {
	step int

	dStarLite *DStarLite
	grid      *testgraphs.LimitedVisionGrid

	w io.Writer
}

// dump writes a single step of a D* Lite path search to the dumper's io.Writer.
func (d *dumper) dump(withpath bool) {
	if d == nil {
		return
	}
	var pathStep map[int64]int
	if withpath {
		pathStep = make(map[int64]int)
		path, _ := d.dStarLite.Path()
		for i, n := range path {
			pathStep[n.ID()] = i
		}
	}
	fmt.Fprintf(d.w, "Step:%d kₘ=%v\n", d.step, d.dStarLite.keyModifier)
	d.step++
	w := tabwriter.NewWriter(d.w, 0, 0, 0, ' ', tabwriter.Debug)
	rows, cols := d.grid.Grid.Dims()
	for r := 0; r < rows; r++ {
		if r == 0 {
			for c := 0; c < cols; c++ {
				if c != 0 {
					fmt.Fprint(w, "\t")
				}
				fmt.Fprint(w, "-------------------")
			}
			fmt.Fprintln(w)
		}
		for ln := 0; ln < 6; ln++ {
			for c := 0; c < cols; c++ {
				if c != 0 {
					fmt.Fprint(w, "\t")
				}
				n := d.dStarLite.model.Node(d.grid.NodeAt(r, c).ID()).(*dStarLiteNode)
				switch ln {
				case 0:
					if n.ID() == d.grid.Location.ID() {
						if d.grid.Grid.HasOpen(n.ID()) {
							fmt.Fprintf(w, "id:%2d  >@<", n.ID())
						} else {
							// Mark location as illegal.
							fmt.Fprintf(w, "id:%2d  >!<", n.ID())
						}
					} else if n.ID() == d.dStarLite.t.ID() {
						fmt.Fprintf(w, "id:%2d   G", n.ID())
						// Mark goal cell as illegal.
						if !d.grid.Grid.HasOpen(n.ID()) {
							fmt.Fprint(w, "!")
						}
					} else if pathStep[n.ID()] > 0 {
						fmt.Fprintf(w, "id:%2d  %2d", n.ID(), pathStep[n.ID()])
						// Mark path cells with an obstruction.
						if !d.grid.Grid.HasOpen(n.ID()) {
							fmt.Fprint(w, "!")
						}
					} else {
						fmt.Fprintf(w, "id:%2d", n.ID())
						// Mark cells with an obstruction.
						if !d.grid.Grid.HasOpen(n.ID()) {
							fmt.Fprint(w, "   *")
						}
					}
				case 1:
					fmt.Fprintf(w, "h:  %.4v", d.dStarLite.heuristic(n, d.dStarLite.Here()))
				case 2:
					fmt.Fprintf(w, "g:  %.4v", n.g)
				case 3:
					fmt.Fprintf(w, "rhs:%.4v", n.rhs)
				case 4:
					if n.g != n.rhs {
						fmt.Fprintf(w, "key:%.3f", n.key)
					}
					if n.key == n.key {
						// Mark keys for nodes in the priority queue.
						// We use NaN inequality for this check since all
						// keys not in the queue must have their key set
						// to badKey.
						//
						// This should always mark cells where key is
						// printed.
						fmt.Fprint(w, "*")
					}
					if n.g > n.rhs {
						fmt.Fprint(w, "^")
					}
					if n.g < n.rhs {
						fmt.Fprint(w, "v")
					}
				default:
					fmt.Fprint(w, "-------------------")
				}
			}
			fmt.Fprintln(w)
		}
	}
	w.Flush()
	fmt.Fprintln(d.w)
}

// printEdges pretty prints the given edges to the dumper's io.Writer using the provided
// format string. The edges are first formated to a string, so the format string must use
// the %s verb to indicate where the edges are to be printed.
func (d *dumper) printEdges(format string, edges []simple.WeightedEdge) {
	if d == nil {
		return
	}
	var buf bytes.Buffer
	sort.Sort(lexically(edges))
	for i, e := range edges {
		if i != 0 {
			fmt.Fprint(&buf, ", ")
		}
		fmt.Fprintf(&buf, "%d->%d:%.4v", e.From().ID(), e.To().ID(), e.Weight())
	}
	if len(edges) == 0 {
		fmt.Fprint(&buf, "none")
	}
	fmt.Fprintf(d.w, format, buf.Bytes())
}

type lexically []simple.WeightedEdge

func (l lexically) Len() int { return len(l) }
func (l lexically) Less(i, j int) bool {
	return l[i].From().ID() < l[j].From().ID() || (l[i].From().ID() == l[j].From().ID() && l[i].To().ID() < l[j].To().ID())
}
func (l lexically) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
