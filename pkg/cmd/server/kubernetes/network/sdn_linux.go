package network

import (
	"strings"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
	sdnnode "github.com/openshift/origin/pkg/network/node"
	sdnproxy "github.com/openshift/origin/pkg/network/proxy"
)

func NewSDNInterfaces(options configapi.NodeConfig, originClient *osclient.Client, kubeClient kclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (network.NodeInterface, network.ProxyInterface, error) {
	runtimeEndpoint := options.DockerConfig.DockerShimSocket
	runtime, ok := options.KubeletArguments["container-runtime"]
	if ok && len(runtime) == 1 && runtime[0] == "remote" {
		endpoint, ok := options.KubeletArguments["container-runtime-endpoint"]
		if ok && len(endpoint) == 1 {
			runtimeEndpoint = endpoint[0]
		}
	}

	// dockershim + kube CNI driver delegates hostport handling to plugins,
	// while CRI-O handles hostports itself. Thus we need to disable the
	// SDN's hostport handling when run under CRI-O.
	enableHostports := !strings.Contains(runtimeEndpoint, "crio")

	node, err := sdnnode.New(&sdnnode.OsdnNodeConfig{
		PluginName:         options.NetworkConfig.NetworkPluginName,
		Hostname:           options.NodeName,
		SelfIP:             options.NodeIP,
		RuntimeEndpoint:    runtimeEndpoint,
		MTU:                options.NetworkConfig.MTU,
		OSClient:           originClient,
		KClient:            kubeClient,
		KubeInformers:      internalKubeInformers,
		IPTablesSyncPeriod: proxyconfig.IPTables.SyncPeriod.Duration,
		ProxyMode:          proxyconfig.Mode,
		EnableHostports:    enableHostports,
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
