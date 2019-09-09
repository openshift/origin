// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dot_test

import (
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

type edgeWithPorts struct {
	simple.Edge
	fromPort, toPort string
}

func (e edgeWithPorts) ReversedEdge() graph.Edge {
	e.F, e.T = e.T, e.F
	e.fromPort, e.toPort = e.toPort, e.fromPort
	return e
}

func (e edgeWithPorts) FromPort() (string, string) {
	return e.fromPort, ""
}

func (e edgeWithPorts) ToPort() (string, string) {
	return e.toPort, ""
}

func ExamplePorter() {
	g := simple.NewUndirectedGraph()
	g.SetEdge(edgeWithPorts{
		Edge:     simple.Edge{F: simple.Node(1), T: simple.Node(0)},
		fromPort: "p1",
		toPort:   "p2",
	})

	result, _ := dot.Marshal(g, "", "", "  ")
	fmt.Print(string(result))

	// Output:
	// strict graph {
	//   // Node definitions.
	//   0;
	//   1;
	//
	//   // Edge definitions.
	//   0:p2 -- 1:p1;
	// }
}
