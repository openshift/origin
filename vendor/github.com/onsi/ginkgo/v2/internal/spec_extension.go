package internal

import (
	"github.com/onsi/ginkgo/v2/types"
)

func (s Spec) CodeLocation() []types.CodeLocation {
	locations := []types.CodeLocation{}
	for _, node := range s.Nodes {
		locations = append(locations, node.CodeLocation)
	}
	return locations
}

func (s Spec) AppendText(text string) {
	s.Nodes[len(s.Nodes)-1].Text += text
}
