package configprocessing

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
)

func RouteAllocator(routingConfig configapi.RoutingConfig) (*routeallocationcontroller.RouteAllocationController, error) {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(routingConfig.Subdomain)
	if err != nil {
		return nil, err
	}

	return factory.Create(plugin), nil
}
