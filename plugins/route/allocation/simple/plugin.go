package simple

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/pborman/uuid"
	"k8s.io/kubernetes/pkg/util"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// Default DNS suffix to use if no configuration is passed to this plugin.
const defaultDNSSuffix = "router.default.svc.cluster.local"

// SimpleAllocationPlugin implements the route.AllocationPlugin interface
// to provide a simple unsharded (or single sharded) allocation plugin.
type SimpleAllocationPlugin struct {
	DNSSuffix string
}

// NewSimpleAllocationPlugin creates a new SimpleAllocationPlugin.
func NewSimpleAllocationPlugin(suffix string) (*SimpleAllocationPlugin, error) {
	if len(suffix) == 0 {
		suffix = defaultDNSSuffix
	}

	glog.V(4).Infof("Route plugin initialized with suffix=%s", suffix)

	// Check that the DNS suffix is valid.
	if !util.IsDNS1123Subdomain(suffix) {
		return nil, fmt.Errorf("invalid DNS suffix: %s", suffix)
	}

	return &SimpleAllocationPlugin{DNSSuffix: suffix}, nil
}

// Allocate a router shard for the given route. This plugin always returns
// the "global" router shard.
func (p *SimpleAllocationPlugin) Allocate(route *routeapi.Route) (*routeapi.RouterShard, error) {

	glog.V(4).Infof("Allocating global shard *.%s to Route: %s", p.DNSSuffix, route.ServiceName)

	return &routeapi.RouterShard{ShardName: "global", DNSSuffix: p.DNSSuffix}, nil
}

// GenerateHostname generates a host name for a route - using the service name,
// namespace (if provided) and the router shard dns suffix.
func (p *SimpleAllocationPlugin) GenerateHostname(route *routeapi.Route, shard *routeapi.RouterShard) string {

	name := route.Name
	if len(name) == 0 {
		name = route.ServiceName
	}
	if len(name) == 0 {
		name = uuid.NewUUID().String()
		glog.V(4).Infof("No service name passed, using generated name: %s", name)
	}

	s := ""
	if len(route.Namespace) == 0 {
		s = fmt.Sprintf("%s.%s", name, shard.DNSSuffix)
	} else {
		s = fmt.Sprintf("%s.%s.%s", name, route.Namespace, shard.DNSSuffix)
	}

	glog.V(4).Infof("Generated hostname=%s for Route: %s", s, route.Name)

	return s
}
