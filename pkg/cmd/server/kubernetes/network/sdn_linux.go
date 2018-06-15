package network

import (
	"strings"

	kclientv1 "k8s.io/api/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	sdnnode "github.com/openshift/origin/pkg/network/node"
	sdnproxy "github.com/openshift/origin/pkg/network/proxy"
)

func NewSDNInterfaces(options configapi.NodeConfig, networkClient networkclient.Interface,
	kubeClientset kclientset.Interface, kubeClient kinternalclientset.Interface,
	internalKubeInformers kinternalinformers.SharedInformerFactory,
	internalNetworkInformers networkinformers.SharedInformerFactory,
	proxyconfig *kubeproxyconfig.KubeProxyConfiguration) (NodeInterface, ProxyInterface, error) {

	runtimeEndpoint := options.DockerConfig.DockerShimSocket
	runtime, ok := options.KubeletArguments["container-runtime"]
	if ok && len(runtime) == 1 && runtime[0] == "remote" {
		endpoint, ok := options.KubeletArguments["container-runtime-endpoint"]
		if ok && len(endpoint) == 1 {
			runtimeEndpoint = endpoint[0]
		}
	}

	cniBinDir := kcni.DefaultCNIDir
	if val, ok := options.KubeletArguments["cni-bin-dir"]; ok && len(val) == 1 {
		cniBinDir = val[0]
	}
	cniConfDir := kcni.DefaultNetDir
	if val, ok := options.KubeletArguments["cni-conf-dir"]; ok && len(val) == 1 {
		cniConfDir = val[0]
	}

	// dockershim + kube CNI driver delegates hostport handling to plugins,
	// while CRI-O handles hostports itself. Thus we need to disable the
	// SDN's hostport handling when run under CRI-O.
	enableHostports := !strings.Contains(runtimeEndpoint, "crio")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, kclientv1.EventSource{Component: "openshift-sdn", Host: options.NodeName})

	node, err := sdnnode.New(&sdnnode.OsdnNodeConfig{
		PluginName:         options.NetworkConfig.NetworkPluginName,
		Hostname:           options.NodeName,
		SelfIP:             options.NodeIP,
		RuntimeEndpoint:    runtimeEndpoint,
		CNIBinDir:          cniBinDir,
		CNIConfDir:         cniConfDir,
		MTU:                options.NetworkConfig.MTU,
		NetworkClient:      networkClient,
		KClient:            kubeClient,
		KubeInformers:      internalKubeInformers,
		NetworkInformers:   internalNetworkInformers,
		IPTablesSyncPeriod: proxyconfig.IPTables.SyncPeriod.Duration,
		MasqueradeBit:      proxyconfig.IPTables.MasqueradeBit,
		ProxyMode:          proxyconfig.Mode,
		EnableHostports:    enableHostports,
		Recorder:           recorder,
	})
	if err != nil {
		return nil, nil, err
	}

	proxy, err := sdnproxy.New(options.NetworkConfig.NetworkPluginName, networkClient, kubeClient, internalNetworkInformers)
	if err != nil {
		return nil, nil, err
	}

	return node, proxy, nil
}
