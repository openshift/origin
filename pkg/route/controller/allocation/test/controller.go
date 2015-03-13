package test

import (
	"fmt"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/controller/allocation"
)

type TestAllocationPlugin struct {
	Name string
}

func (p *TestAllocationPlugin) Allocate(route *routeapi.Route) (*routeapi.RouterShard, error) {

	return &routeapi.RouterShard{ShardName: "test", DNSSuffix: "openshift.test"}, nil
}

func (p *TestAllocationPlugin) GenerateHostname(route *routeapi.Route, shard *routeapi.RouterShard) string {
	if len(route.ServiceName) > 0 && len(route.Namespace) > 0 {
		return fmt.Sprintf("%s-%s.%s", route.ServiceName, route.Namespace, shard.DNSSuffix)
	}

	return "test-test-test.openshift.test"
}

func NewTestRouteAllocationController() *allocation.RouteAllocationController {
	plugin := &TestAllocationPlugin{"test route allocation plugin"}
	return &allocation.RouteAllocationController{Plugin: plugin}
}
