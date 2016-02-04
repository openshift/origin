package analysis

import (
	"fmt"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	routeapi "github.com/openshift/origin/pkg/route/api"
	routeedges "github.com/openshift/origin/pkg/route/graph"
	routegraph "github.com/openshift/origin/pkg/route/graph/nodes"
)

const (
	// MissingRoutePortWarning is returned when a route has no route port specified
	// and the service it routes to has multiple ports.
	MissingRoutePortWarning = "MissingRoutePort"
	// MissingServiceWarning is returned when there is no service for the specific route.
	MissingServiceWarning = "MissingService"
	// MissingTLSTerminationTypeErr is returned when a route with a tls config doesn't
	// specify a tls termination type.
	MissingTLSTerminationTypeErr = "MissingTLSTermination"
	// PathBasedPassthroughErr is returned when a path based route is passthrough
	// terminated.
	PathBasedPassthroughErr = "PathBasedPassthrough"
)

// FindMissingPortMapping checks all routes and reports those that don't specify a port while
// the service they are routing to, has multiple ports. Also if a service for a route doesn't
// exist, will be reported.
func FindMissingPortMapping(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
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
						f.ResourceName(routeNode), f.ResourceName(svcNode), f.ResourceName(svcNode)),
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
						f.ResourceName(routeNode), f.ResourceName(svcNode)),
				})

				continue route
			}
		}
	}

	return markers
}

func FindMissingTLSTerminationType(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastRouteNode := range g.NodesByKind(routegraph.RouteNodeKind) {
		routeNode := uncastRouteNode.(*routegraph.RouteNode)

		if routeNode.Spec.TLS != nil && len(routeNode.Spec.TLS.Termination) == 0 {
			markers = append(markers, osgraph.Marker{
				Node: routeNode,

				Severity:   osgraph.ErrorSeverity,
				Key:        MissingTLSTerminationTypeErr,
				Message:    fmt.Sprintf("%s has a TLS configuration but no termination type specified.", f.ResourceName(routeNode)),
				Suggestion: osgraph.Suggestion(fmt.Sprintf("oc patch %s -p '{\"spec\":{\"tls\":{\"termination\":\"<type>\"}}}' (replace <type> with a valid termination type: edge, passthrough, reencrypt)", f.ResourceName(routeNode)))})
		}
	}

	return markers
}

func FindPathBasedPassthroughRoutes(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastRouteNode := range g.NodesByKind(routegraph.RouteNodeKind) {
		routeNode := uncastRouteNode.(*routegraph.RouteNode)

		if len(routeNode.Spec.Path) > 0 && routeNode.Spec.TLS != nil && routeNode.Spec.TLS.Termination == routeapi.TLSTerminationPassthrough {
			markers = append(markers, osgraph.Marker{
				Node: routeNode,

				Severity:   osgraph.ErrorSeverity,
				Key:        PathBasedPassthroughErr,
				Message:    fmt.Sprintf("%s is path-based and uses passthrough termination, which is an invalid combination.", f.ResourceName(routeNode)),
				Suggestion: osgraph.Suggestion(fmt.Sprintf("1. use spec.tls.termination=edge or 2. use spec.tls.termination=reencrypt and specify spec.tls.destinationCACertificate or 3. remove spec.path")),
			})
		}
	}

	return markers
}
