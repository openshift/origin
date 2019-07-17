package routeallocationcontroller

import (
	"k8s.io/klog"

	routeapi "github.com/openshift/openshift-apiserver/pkg/route/apis/route"
	"github.com/openshift/openshift-apiserver/pkg/route/apiserver/routeinterfaces"
)

// RouteAllocationController abstracts the details of how routes are
// allocated to router shards.
type RouteAllocationController struct {
	Plugin routeinterfaces.AllocationPlugin
}

// AllocateRouterShard allocates a router shard for the given route.
func (c *RouteAllocationController) AllocateRouterShard(route *routeapi.Route) (*routeapi.RouterShard, error) {

	klog.V(4).Infof("Allocating router shard for Route: %s [alias=%s]",
		route.Name, route.Spec.Host)

	shard, err := c.Plugin.Allocate(route)

	if err != nil {
		klog.Errorf("unable to allocate router shard: %v", err)
		return shard, err
	}

	klog.V(4).Infof("Route %s allocated to shard %s [suffix=%s]",
		route.Name, shard.ShardName, shard.DNSSuffix)

	return shard, err
}

// GenerateHostname generates a host name for the given route and router shard combination.
func (c *RouteAllocationController) GenerateHostname(route *routeapi.Route, shard *routeapi.RouterShard) string {
	klog.V(4).Infof("Generating host name for Route: %s", route.Name)

	s := c.Plugin.GenerateHostname(route, shard)

	klog.V(4).Infof("Route: %s, generated host name/alias=%s",
		route.Name, s)

	return s
}
