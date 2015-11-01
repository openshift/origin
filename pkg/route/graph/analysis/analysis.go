package analysis

import (
	"fmt"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	routeedges "github.com/openshift/origin/pkg/route/graph"
	routegraph "github.com/openshift/origin/pkg/route/graph/nodes"
)

const (
	// MissingRoutePortWarning is returned when a route has no route port specified
	// and the service it routes to has multiple ports.
	MissingRoutePortWarning = "MissingRoutePort"
	// MissingServiceWarning is returned when there is no service for the specific route.
	MissingServiceWarning = "MissingService"
)

// FindMissingPortMapping checks all routes and reports those that don't specify a port while
// the service they are routing to, has multiple ports. Also if a service for a route doesn't
// exist, will be reported.
func FindMissingPortMapping(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

route:
	for _, uncastRouteNode := range g.NodesByKind(routegraph.RouteNodeKind) {
		for _, uncastServiceNode := range g.SuccessorNodesByEdgeKind(uncastRouteNode, routeedges.ExposedThroughRouteEdgeKind) {
			routeNode := uncastRouteNode.(*routegraph.RouteNode)
			svcNode := uncastServiceNode.(*kubegraph.ServiceNode)

			if !svcNode.Found() {
				markers = append(markers, osgraph.Marker{
					Node:         routeNode,
					RelatedNodes: []graph.Node{svcNode},

					Severity: osgraph.WarningSeverity,
					Key:      MissingServiceWarning,
					Message: fmt.Sprintf("%s is supposed to route traffic to %s but %s doesn't exist.",
						routeNode.ResourceString(), svcNode.ResourceString(), svcNode.ResourceString()),
				})

				continue route
			}

			if len(svcNode.Spec.Ports) > 1 && (routeNode.Spec.Port == nil || len(routeNode.Spec.Port.TargetPort.String()) == 0) {
				markers = append(markers, osgraph.Marker{
					Node:         routeNode,
					RelatedNodes: []graph.Node{svcNode},

					Severity: osgraph.WarningSeverity,
					Key:      MissingRoutePortWarning,
					Message: fmt.Sprintf("%s doesn't have a port specified and is routing traffic to %s which uses multiple ports.",
						routeNode.ResourceString(), svcNode.ResourceString()),
				})

				continue route
			}
		}
	}

	return markers
}
