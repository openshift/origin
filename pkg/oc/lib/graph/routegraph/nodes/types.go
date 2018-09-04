package nodes

import (
	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

var (
	RouteNodeKind = reflect.TypeOf(routev1.Route{}).Name()
)

func RouteNodeName(o *routev1.Route) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(RouteNodeKind, o)
}

type RouteNode struct {
	osgraph.Node
	*routev1.Route
}

func (n RouteNode) Object() interface{} {
	return n.Route
}

func (n RouteNode) String() string {
	return string(RouteNodeName(n.Route))
}

func (*RouteNode) Kind() string {
	return RouteNodeKind
}
