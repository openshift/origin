package analysis

import (
	"fmt"
	"strings"

	"github.com/gonum/graph"

	"github.com/openshift/origin/pkg/oc/lib/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

const (
	DuelingReplicationControllerWarning = "DuelingReplicationControllers"
)

func FindDuelingReplicationControllers(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastRCNode := range g.NodesByKind(kubegraph.ReplicationControllerNodeKind) {
		rcNode := uncastRCNode.(*kubegraph.ReplicationControllerNode)

		for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(rcNode, appsgraph.ManagedByControllerEdgeKind) {
			podNode := uncastPodNode.(*kubegraph.PodNode)

			// check to see if this pod is managed by more than one RC
			uncastOwningRCs := g.SuccessorNodesByEdgeKind(podNode, appsgraph.ManagedByControllerEdgeKind)
			if len(uncastOwningRCs) > 1 {
				involvedRCNames := []string{}
				relatedNodes := []graph.Node{uncastPodNode}

				for _, uncastOwningRC := range uncastOwningRCs {
					if uncastOwningRC.ID() == rcNode.ID() {
						continue
					}
					owningRC := uncastOwningRC.(*kubegraph.ReplicationControllerNode)
					involvedRCNames = append(involvedRCNames, f.ResourceName(owningRC))

					relatedNodes = append(relatedNodes, uncastOwningRC)
				}

				markers = append(markers, osgraph.Marker{
					Node:         rcNode,
					RelatedNodes: relatedNodes,

					Severity: osgraph.WarningSeverity,
					Key:      DuelingReplicationControllerWarning,
					Message:  fmt.Sprintf("%s is competing for %s with %s", f.ResourceName(rcNode), f.ResourceName(podNode), strings.Join(involvedRCNames, ", ")),
				})
			}
		}
	}

	return markers
}
