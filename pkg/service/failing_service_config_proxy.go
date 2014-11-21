package service

import (
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
)

type FailingServiceConfigProxy struct {
}

// OnUpdate implements method for kubernetes/pkg/proxy/config/ServiceConfigHandler
func (proxy *FailingServiceConfigProxy) OnUpdate(services []api.Service) {
	names := []string{}
	for i := range services {
		names = append(names, services[i].Name)
	}
	glog.V(4).Infof("Failed to properly wire up services.  This can happen if you forget to launch with permissions to iptables.  Access to the following services will be impaired: %#v\n", strings.Join(names, ", "))
}
