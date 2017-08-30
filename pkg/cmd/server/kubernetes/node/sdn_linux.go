package node

import (
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/sdn"
	sdnnode "github.com/openshift/origin/pkg/sdn/node"
	sdnproxy "github.com/openshift/origin/pkg/sdn/proxy"
)

func NewSDNInterfaces(options configapi.NodeConfig, originClient *osclient.Client, kubeClient kclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (sdn.NodeInterface, sdn.ProxyInterface, error) {
	node, err := sdnnode.New(&sdnnode.OsdnNodeConfig{
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

	proxy, err := sdnproxy.New(options.NetworkConfig.NetworkPluginName, originClient, kubeClient)
	if err != nil {
		return nil, nil, err
	}

	return node, proxy, nil
}
