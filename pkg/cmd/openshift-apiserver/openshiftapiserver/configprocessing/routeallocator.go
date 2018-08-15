package configprocessing

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
)

func RouteAllocator(openshiftAPIServerConfig configapi.MasterConfig) (*routeallocationcontroller.RouteAllocationController, error) {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(openshiftAPIServerConfig.RoutingConfig.Subdomain)
	if err != nil {
		return nil, err
	}

	return factory.Create(plugin), nil
}
