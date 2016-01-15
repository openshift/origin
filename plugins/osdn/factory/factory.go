package factory

import (
	"strings"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	osclient "github.com/openshift/origin/pkg/client"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/openshift-sdn/plugins/osdn/ovs"
)

func newPlugin(pluginType string, isMaster bool, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string) (api.OsdnPlugin, api.FilteringEndpointsConfigHandler, error) {
	registry, err := osdn.NewRegistry(isMaster, osClient, kClient)
	if err != nil {
		return nil, nil, err
	}

	switch strings.ToLower(pluginType) {
	case ovs.SingleTenantPluginName():
		return ovs.CreatePlugin(registry, false, hostname, selfIP)
	case ovs.MultiTenantPluginName():
		return ovs.CreatePlugin(registry, true, hostname, selfIP)
	}

	return nil, nil, nil
}

// Call by higher layers to create the plugin instance
func NewMasterPlugin(pluginType string, osClient *osclient.Client, kClient *kclient.Client) (api.OsdnPlugin, api.FilteringEndpointsConfigHandler, error) {
	return newPlugin(pluginType, true, osClient, kClient, "", "")
}

// Call by higher layers to create the plugin instance
func NewClientPlugin(pluginType string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string) (api.OsdnPlugin, api.FilteringEndpointsConfigHandler, error) {
	return newPlugin(pluginType, false, osClient, kClient, hostname, selfIP)
}
