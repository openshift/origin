package factory

import (
	"strings"
	"time"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	osclient "github.com/openshift/origin/pkg/client"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/openshift-sdn/plugins/osdn/ovs"
)

// Call by higher layers to create the plugin SDN master instance
func NewMasterPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (api.OsdnPlugin, error) {
	return newPlugin(pluginName, osClient, kClient, "", "", 0)
}

// Call by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration) (api.OsdnPlugin, error) {
	return newPlugin(pluginName, osClient, kClient, hostname, selfIP, iptablesSyncPeriod)
}

func newPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration) (api.OsdnPlugin, error) {
	switch strings.ToLower(pluginName) {
	case ovs.SingleTenantPluginName, ovs.MultiTenantPluginName:
		return ovs.CreatePlugin(osdn.NewRegistry(osClient, kClient), pluginName, hostname, selfIP, iptablesSyncPeriod)
	}

	return nil, nil
}

// Call by higher layers to create the proxy plugin instance; only used by nodes
func NewProxyPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (api.FilteringEndpointsConfigHandler, error) {
	switch strings.ToLower(pluginName) {
	case ovs.MultiTenantPluginName:
		return ovs.CreateProxyPlugin(osdn.NewRegistry(osClient, kClient))
	}

	return nil, nil
}
