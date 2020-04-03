// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/spatial/r2"
	"gonum.org/v1/gonum/stat/mds"
)

// IsomapR2 implements a graph layout algorithm based on the Isomap
// non-linear dimensionality reduction method. Coordinates of nodes
// are computed by finding a Torgerson multidimensional scaling of
// the shortest path distances between all pairs of node in the graph.
// The all pair shortest path distances are calculated using the
// Floyd-Warshall algorithm and so IsomapR2 will not scale to large
// graphs. Graphs with more than one connected component cannot be
// laid out by IsomapR2.
type IsomapR2 struct{}

// Update is the IsomapR2 spatial graph update function.
func (IsomapR2) Update(g graph.Graph, layout LayoutR2) bool {
	nodes := graph.NodesOf(g.Nodes())
	v := isomap(g, nodes, 2)
	if v == nil {
		return false
	}

	// FIXME(kortschak): The Layout types do not have the capacity to
	// be cleared in the current API. Is this a problem? I don't know
	// at this stage. It might be if the layout is reused between graphs.
	// Someone may do this.

	for i, n := range nodes {
		layout.SetCoord2(n.ID(), r2.Vec{X: v.At(i, 0), Y: v.At(i, 1)})
	}
	return false
}

func isomap(g graph.Graph, nodes []graph.Node, dims int) *mat.Dense {
	p, ok := path.FloydWarshall(g)
	if !ok {
		return nil
	}

	dist := mat.NewSymDense(len(nodes), nil)
	for i, u := range nodes {
		for j, v := range nodes {
			dist.SetSym(i, j, p.Weight(u.ID(), v.ID()))
		}
	}
	var v mat.Dense
	k, _ := mds.TorgersonScaling(&v, nil, dist)
	if k < dims {
		return nil
	}

	return &v
}
