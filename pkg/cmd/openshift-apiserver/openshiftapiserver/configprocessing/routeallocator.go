package configprocessing

import (
	"github.com/openshift/origin/pkg/route/apiserver/routeallocationcontroller"
	routeplugin "github.com/openshift/origin/pkg/route/apiserver/simplerouteallocation"
)

func RouteAllocator(routingSubdomain string) (*routeallocationcontroller.RouteAllocationController, error) {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(routingSubdomain)
	if err != nil {
		return nil, err
	}

	return factory.Create(plugin), nil
}
