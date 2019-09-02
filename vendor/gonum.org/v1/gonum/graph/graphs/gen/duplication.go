// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"fmt"
	"math"
	"sort"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
)

// UndirectedMutator is an undirected graph builder that can remove edges.
type UndirectedMutator interface {
	graph.UndirectedBuilder
	graph.EdgeRemover
}

// Duplication constructs a graph in the destination, dst, of order n. New nodes
// are created by duplicating an existing node and all its edges. Each new edge is
// deleted with probability delta. Additional edges are added between the new node
// and existing nodes with probability alpha/|V|. An exception to this addition
// rule is made for the parent node when sigma is not NaN; in this case an edge is
// created with probability sigma. With the exception of the sigma parameter, this
// corresponds to the completely correlated case in doi:10.1016/S0022-5193(03)00028-6.
// If src is not nil it is used as the random source, otherwise rand.Float64 is used.
func Duplication(dst UndirectedMutator, n int, delta, alpha, sigma float64, src rand.Source) error {
	// As described in doi:10.1016/S0022-5193(03)00028-6 but
	// also clarified in doi:10.1186/gb-2007-8-4-r51.

	if delta < 0 || delta > 1 {
		return fmt.Errorf("gen: bad delta: delta=%v", delta)
	}
	if alpha <= 0 || alpha > 1 {
		return fmt.Errorf("gen: bad alpha: alpha=%v", alpha)
	}
	if sigma < 0 || sigma > 1 {
		return fmt.Errorf("gen: bad sigma: sigma=%v", sigma)
	}

	var (
		rnd  func() float64
		rndN func(int) int
	)
	if src == nil {
		rnd = rand.Float64
		rndN = rand.Intn
	} else {
		r := rand.New(src)
		rnd = r.Float64
		rndN = r.Intn
	}

	nodes := graph.NodesOf(dst.Nodes())
	sort.Sort(ordered.ByID(nodes))
	if len(nodes) == 0 {
		n--
		u := dst.NewNode()
		dst.AddNode(u)
		nodes = append(nodes, u)
	}
	for i := 0; i < n; i++ {
		u := nodes[rndN(len(nodes))]
		d := dst.NewNode()
		did := d.ID()

		// Add the duplicate node.
		dst.AddNode(d)

		// Loop until we have connectivity
		// into the rest of the graph.
		for {
			// Add edges to parent's neighbours.
			to := graph.NodesOf(dst.From(u.ID()))
			sort.Sort(ordered.ByID(to))
			for _, v := range to {
				vid := v.ID()
				if rnd() < delta || dst.HasEdgeBetween(vid, did) {
					continue
				}
				if vid < did {
					dst.SetEdge(dst.NewEdge(v, d))
				} else {
					dst.SetEdge(dst.NewEdge(d, v))
				}
			}

			// Add edges to old nodes.
			scaledAlpha := alpha / float64(len(nodes))
			for _, v := range nodes {
				uid := u.ID()
				vid := v.ID()
				switch vid {
				case uid:
					if !math.IsNaN(sigma) {
						if i == 0 || rnd() < sigma {
							if vid < did {
								dst.SetEdge(dst.NewEdge(v, d))
							} else {
								dst.SetEdge(dst.NewEdge(d, v))
							}
						}
						continue
					}
					fallthrough
				default:
					if rnd() < scaledAlpha && !dst.HasEdgeBetween(vid, did) {
						if vid < did {
							dst.SetEdge(dst.NewEdge(v, d))
						} else {
							dst.SetEdge(dst.NewEdge(d, v))
						}
					}
				}
			}

			if dst.From(did).Len() != 0 {
				break
			}
		}

		nodes = append(nodes, d)
	}

	return nil
}
