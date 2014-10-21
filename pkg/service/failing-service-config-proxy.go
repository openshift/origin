package service

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
)

type FailingServiceConfigProxy struct {
}

// OnUpdate implements method for kubernetes/pkg/proxy/config/ServiceConfigHandler
func (proxy *FailingServiceConfigProxy) OnUpdate(services []api.Service) {
	glog.Errorf("Failed to properly wire up service.  This can happen if you forget to launch with permissions to iptables.  Access to the following services will be impaired: %#v\n", services)
}
