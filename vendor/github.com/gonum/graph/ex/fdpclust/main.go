package main

import (
	"fmt"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"
)

func main() {
	// graph G {
	G := NewGraphNode(0)
	// e
	e := NewGraphNode(1)

	// subgraph clusterA {
	clusterA := NewGraphNode(2)

	// a -- b
	a := NewGraphNode(3)
	b := NewGraphNode(4)
	a.AddNeighbor(b)
	b.AddNeighbor(a)
	clusterA.AddRoot(a)
	clusterA.AddRoot(b)

	// subgraph clusterC {
	clusterC := NewGraphNode(5)
	// C -- D
	C := NewGraphNode(6)
	D := NewGraphNode(7)
	C.AddNeighbor(D)
	D.AddNeighbor(C)

	clusterC.AddRoot(C)
	clusterC.AddRoot(D)
	// }
	clusterA.AddRoot(clusterC)
	// }

	// subgraph clusterB {
	clusterB := NewGraphNode(8)

	// d -- f
	d := NewGraphNode(9)
	f := NewGraphNode(10)
	d.AddNeighbor(f)
	f.AddNeighbor(d)
	clusterB.AddRoot(d)
	clusterB.AddRoot(f)
	// }

	// d -- D
	d.AddNeighbor(D)
	D.AddNeighbor(d)

	// e -- clusterB
	e.AddNeighbor(clusterB)
	clusterB.AddNeighbor(e)

	// clusterC -- clusterB
	clusterC.AddNeighbor(clusterB)
	clusterB.AddNeighbor(clusterC)

	G.AddRoot(e)
	G.AddRoot(clusterA)
	G.AddRoot(clusterB)
	// }

	if !topo.IsPathIn(G, []graph.Node{C, D, d, f}) {
		fmt.Println("Not working!")
	} else {
		fmt.Println("Working!")
	}
}
