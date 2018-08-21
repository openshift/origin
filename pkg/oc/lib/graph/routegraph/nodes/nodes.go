package nodes

import (
	"github.com/gonum/graph"

	routev1 "github.com/openshift/api/route/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

// EnsureRouteNode adds a graph node for the specific route if it does not exist
func EnsureRouteNode(g osgraph.MutableUniqueGraph, route *routev1.Route) *RouteNode {
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
