package factory

import (
	"fmt"
	"strings"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	osclient "github.com/openshift/origin/pkg/client"
	oskserver "github.com/openshift/origin/pkg/cmd/server/kubernetes"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/openshift-sdn/plugins/osdn/flatsdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/multitenant"
)

// Call by higher layers to create the plugin instance
func NewPlugin(pluginType string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, ready chan struct{}) (api.OsdnPlugin, oskserver.FilteringEndpointsConfigHandler, error) {
	switch strings.ToLower(pluginType) {
	case flatsdn.NetworkPluginName():
		return flatsdn.CreatePlugin(osdn.NewRegistry(osClient, kClient), hostname, selfIP, ready)
	case multitenant.NetworkPluginName():
		return multitenant.CreatePlugin(osdn.NewRegistry(osClient, kClient), hostname, selfIP, ready)
	}

	return nil, nil, nil
}
