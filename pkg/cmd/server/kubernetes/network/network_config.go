package network

import (
	"fmt"
	"net"
	"time"

	miekgdns "github.com/miekg/dns"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/certificate"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes/network/transport"
	"github.com/openshift/origin/pkg/dns"
	"github.com/openshift/origin/pkg/network"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

// NetworkConfig represents the required parameters to start OpenShift networking
// through Kubernetes. All fields are required.
type NetworkConfig struct {
	// External kube client
	KubeClientset kclientset.Interface
	// External kube client
	ExternalKubeClientset kclientsetexternal.Interface
	// Internal kubernetes shared informer factory.
	InternalKubeInformers kinternalinformers.SharedInformerFactory
	// Internal network shared informer factory.
	InternalNetworkInformers networkinformers.SharedInformerFactory

	// ProxyConfig is the configuration for the kube-proxy, fully initialized
	ProxyConfig *kubeproxyconfig.KubeProxyConfiguration
	// EnableUnidling indicates whether or not the unidling hybrid proxy should be used
	EnableUnidling bool

	// DNSConfig controls the DNS configuration.
	DNSServer *dns.Server

	// SDNNode is an optional SDN node interface
	SDNNode NodeInterface
	// SDNProxy is an optional service endpoints filterer
	SDNProxy ProxyInterface
}

type ProxyInterface interface {
	proxyconfig.EndpointsHandler

	Start(proxyconfig.EndpointsHandler) error
}

type NodeInterface interface {
	Start() error
}

// configureKubeConfigForClientCertRotation attempts to watch for client certificate rotation on the kubelet's cert
// dir, if configured. This allows the network component to participate in client cert rotation when it is in the
// same process (since it can't share a client with the Kubelet). This code path will be removed or altered when
// the network process is split into a daemonset.
func configureKubeConfigForClientCertRotation(options configapi.NodeConfig, kubeConfig *rest.Config) error {
	v, ok := options.KubeletArguments["cert-dir"]
	if !ok || len(v) == 0 {
		return nil
	}
	certDir := v[0]
	// equivalent to values in pkg/kubelet/certificate/kubelet.go
	store, err := certificate.NewFileStore("kubelet-client", certDir, certDir, kubeConfig.TLSClientConfig.CertFile, kubeConfig.TLSClientConfig.KeyFile)
	if err != nil {
		return err
	}
	return transport.RefreshCertificateAfterExpiry(utilwait.NeverStop, 10*time.Second, kubeConfig, store)
}

// New creates a new network config object for running the networking components of the OpenShift node.
func New(options configapi.NodeConfig, clusterDomain string, proxyConfig *kubeproxyconfig.KubeProxyConfiguration, enableProxy, enableDNS bool) (*NetworkConfig, error) {
	kubeConfig, err := configapi.GetKubeConfigOrInClusterConfig(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	if err := configureKubeConfigForClientCertRotation(options, kubeConfig); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to enable client certificate rotation for network components: %v", err))
	}
	internalKubeClient, err := kclientsetinternal.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	externalKubeClient, err := kclientsetexternal.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	networkClient, err := networkclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	internalKubeInformers := kinternalinformers.NewSharedInformerFactory(internalKubeClient, proxyConfig.ConfigSyncPeriod.Duration)

	config := &NetworkConfig{
		KubeClientset:         kubeClient,
		ExternalKubeClientset: externalKubeClient,
		InternalKubeInformers: internalKubeInformers,

		ProxyConfig:    proxyConfig,
		EnableUnidling: options.EnableUnidling,
	}

	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		config.InternalNetworkInformers = networkinformers.NewSharedInformerFactory(networkClient, network.DefaultInformerResyncPeriod)

		config.SDNNode, config.SDNProxy, err = NewSDNInterfaces(options, networkClient, kubeClient, internalKubeClient, internalKubeInformers, config.InternalNetworkInformers, proxyConfig)
		if err != nil {
			return nil, fmt.Errorf("SDN initialization failed: %v", err)
		}
	}

	if enableDNS {
		dnsConfig, err := dns.NewServerDefaults()
		if err != nil {
			return nil, fmt.Errorf("DNS configuration was not possible: %v", err)
		}
		if len(options.DNSBindAddress) > 0 {
			dnsConfig.DnsAddr = options.DNSBindAddress
		}
		dnsConfig.Domain = clusterDomain + "."
		dnsConfig.Local = "openshift.default.svc." + dnsConfig.Domain

		// identify override nameservers
		var nameservers []string
		for _, s := range options.DNSNameservers {
			nameservers = append(nameservers, s)
		}
		if len(options.DNSRecursiveResolvConf) > 0 {
			c, err := miekgdns.ClientConfigFromFile(options.DNSRecursiveResolvConf)
			if err != nil {
				return nil, fmt.Errorf("could not start DNS, unable to read config file: %v", err)
			}
			for _, s := range c.Servers {
				nameservers = append(nameservers, net.JoinHostPort(s, c.Port))
			}
		}

		if len(nameservers) > 0 {
			dnsConfig.Nameservers = nameservers
		}

		services, err := dns.NewCachedServiceAccessor(internalKubeInformers.Core().InternalVersion().Services())
		if err != nil {
			return nil, fmt.Errorf("could not start DNS: failed to add ClusterIP index: %v", err)
		}

		endpoints, err := dns.NewCachedEndpointsAccessor(internalKubeInformers.Core().InternalVersion().Endpoints())
		if err != nil {
			return nil, fmt.Errorf("could not start DNS: failed to add HostnameIP index: %v", err)
		}

		// TODO: use kubeletConfig.ResolverConfig as an argument to etcd in the event the
		//   user sets it, instead of passing it to the kubelet.
		config.DNSServer = dns.NewServer(
			dnsConfig,
			services,
			endpoints,
			"node",
		)
	}

	return config, nil
}
