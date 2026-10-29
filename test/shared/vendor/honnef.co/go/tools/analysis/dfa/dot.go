package dfa

import (
	"fmt"
	"strings"
)

// Dot returns a directed graph in [Graphviz] format that represents the finite
// join-semilattice ⟨S, ≤⟩. Vertices represent elements in S and edges
// represent the ≤ relation between elements. We map from ⟨S, ∨⟩ to ⟨S, ≤⟩ by
// computing x ∨ y for all elements in [S]², where x ≤ y iff x ∨ y == y.
//
// The resulting graph can be filtered through [tred] to compute the transitive
// reduction of the graph, the visualisation of which corresponds to the Hasse
// diagram of the semilattice.
//
// [Graphviz]: https://graphviz.org/
// [tred]: https://graphviz.org/docs/cli/tred/
func Dot[L Semilattice[Elem], Elem any](states []Elem) string {
	var sb strings.Builder
	sb.WriteString("digraph{\n")
	sb.WriteString("rankdir=\"BT\"\n")

	for i, v := range states {
		if vs, ok := any(v).(fmt.Stringer); ok {
			fmt.Fprintf(&sb, "n%d [label=%q]\n", i, vs)
		} else {
			fmt.Fprintf(&sb, "n%d [label=%q]\n", i, fmt.Sprintf("%v", v))
		}
	}

	var l L

	for dx, x := range states {
		for dy, y := range states {
			if dx == dy {
				continue
			}

			if l.Equals(l.Merge(x, y), y) {
				fmt.Fprintf(&sb, "n%d -> n%d\n", dx, dy)
			}
		}
	}

	sb.WriteString("}")
	return sb.String()
}
