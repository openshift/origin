package analysis

import (
	"fmt"
	"strings"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

const (
	DuelingRepliationControllerWarning = "DuelingReplicationControllers"
)

func FindDuelingReplicationControllers(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastRCNode := range g.NodesByKind(kubegraph.ReplicationControllerNodeKind) {
		rcNode := uncastRCNode.(*kubegraph.ReplicationControllerNode)

		for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rcNode, kubeedges.ManagedByRCEdgeKind) {
			podNode := uncastPodNode.(*kubegraph.PodNode)

			// check to see if this pod is managed by more than one RC
			uncastOwningRCs := g.SuccessorNodesByEdgeKind(podNode, kubeedges.ManagedByRCEdgeKind)
			if len(uncastOwningRCs) > 1 {
				involvedRCNames := []string{}
				relatedNodes := []graph.Node{uncastPodNode}

				for _, uncastOwningRC := range uncastOwningRCs {
					if uncastOwningRC.ID() == rcNode.ID() {
						continue
					}
					owningRC := uncastOwningRC.(*kubegraph.ReplicationControllerNode)
					involvedRCNames = append(involvedRCNames, owningRC.ResourceString())

					relatedNodes = append(relatedNodes, uncastOwningRC)
				}

				markers = append(markers, osgraph.Marker{
					Node:         rcNode,
					RelatedNodes: relatedNodes,

					Severity: osgraph.WarningSeverity,
					Key:      DuelingRepliationControllerWarning,
					Message:  fmt.Sprintf("%s is competing for %s with %s", rcNode.ResourceString(), podNode.ResourceString(), strings.Join(involvedRCNames, ", ")),
				})
			}
		}
	}

	return markers
}
