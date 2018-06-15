package nodes

import (
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// EnsureRouteNode adds a graph node for the specific route if it does not exist
func EnsureRouteNode(g osgraph.MutableUniqueGraph, route *routeapi.Route) *RouteNode {
	return osgraph.EnsureUnique(
		g,
		RouteNodeName(route),
		func(node osgraph.Node) graph.Node {
			return &RouteNode{
				Node:  node,
				Route: route,
			}
		},
	).(*RouteNode)
}
