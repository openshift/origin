package node

import (
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/sdn"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
)

func NewSDNInterfaces(options configapi.NodeConfig, originClient *osclient.Client, kubeClient kclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (sdn.NodeInterface, sdn.ProxyInterface, error) {
	node, err := sdnplugin.NewNodePlugin(&sdnplugin.OsdnNodeConfig{
		PluginName:         options.NetworkConfig.NetworkPluginName,
		Hostname:           options.NodeName,
		SelfIP:             options.NodeIP,
		RuntimeEndpoint:    options.DockerConfig.DockerShimSocket,
		MTU:                options.NetworkConfig.MTU,
		OSClient:           originClient,
		KClient:            kubeClient,
		KubeInformers:      internalKubeInformers,
		IPTablesSyncPeriod: proxyconfig.IPTables.SyncPeriod.Duration,
		ProxyMode:          proxyconfig.Mode,
	})
	if err != nil {
		return nil, nil, err
	}

	proxy, err := sdnplugin.NewProxyPlugin(options.NetworkConfig.NetworkPluginName, originClient, kubeClient)
	if err != nil {
		return nil, nil, err
	}

	return node, proxy, nil
}
