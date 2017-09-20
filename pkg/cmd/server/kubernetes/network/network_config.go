package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/golang/glog"

	miekgdns "github.com/miekg/dns"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/dns"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
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

	// ProxyConfig is the configuration for the kube-proxy, fully initialized
	ProxyConfig *componentconfig.KubeProxyConfiguration
	// EnableUnidling indicates whether or not the unidling hybrid proxy should be used
	EnableUnidling bool

	// DNSConfig controls the DNS configuration.
	DNSServer *dns.Server

	// SDNNode is an optional SDN node interface
	SDNNode network.NodeInterface
	// SDNProxy is an optional service endpoints filterer
	SDNProxy network.ProxyInterface
}

// New creates a new network config object for running the networking components of the OpenShift node.
func New(options configapi.NodeConfig, clusterDomain string, proxyConfig *componentconfig.KubeProxyConfiguration, enableProxy, enableDNS bool) (*NetworkConfig, error) {
	originClient, _, err := configapi.GetOpenShiftClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	internalKubeClient, kubeConfig, err := configapi.GetInternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	externalKubeClient, _, err := configapi.GetExternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	if err = validateNetworkPluginName(originClient, options.NetworkConfig.NetworkPluginName); err != nil {
		return nil, err
	}

	internalKubeInformers := kinternalinformers.NewSharedInformerFactory(internalKubeClient, proxyConfig.ConfigSyncPeriod.Duration)

	var sdnNode network.NodeInterface
	var sdnProxy network.ProxyInterface
	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		sdnNode, sdnProxy, err = NewSDNInterfaces(options, originClient, internalKubeClient, internalKubeInformers, proxyConfig)
		if err != nil {
			return nil, fmt.Errorf("SDN initialization failed: %v", err)
		}
	}

	config := &NetworkConfig{
		KubeClientset:         kubeClient,
		ExternalKubeClientset: externalKubeClient,
		InternalKubeInformers: internalKubeInformers,

		ProxyConfig:    proxyConfig,
		EnableUnidling: options.EnableUnidling,

		SDNNode:  sdnNode,
		SDNProxy: sdnProxy,
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
		glog.Infof("DNS Bind to %s", options.DNSBindAddress)
		config.DNSServer = dns.NewServer(
			dnsConfig,
			services,
			endpoints,
			"node",
		)
	}

	return config, nil
}

func validateNetworkPluginName(originClient *osclient.Client, pluginName string) error {
	if network.IsOpenShiftNetworkPlugin(pluginName) {
		// Detect any plugin mismatches between node and master
		clusterNetwork, err := originClient.ClusterNetwork().Get(networkapi.ClusterNetworkDefault, metav1.GetOptions{})
		if kerrs.IsNotFound(err) {
			return fmt.Errorf("master has not created a default cluster network, network plugin %q can not start", pluginName)
		} else if err != nil {
			return fmt.Errorf("cannot fetch %q cluster network: %v", networkapi.ClusterNetworkDefault, err)
		}

		if clusterNetwork.PluginName != strings.ToLower(pluginName) {
			if len(clusterNetwork.PluginName) != 0 {
				return fmt.Errorf("detected network plugin mismatch between OpenShift node(%q) and master(%q)", pluginName, clusterNetwork.PluginName)
			} else {
				// Do not return error in this case
				glog.Warningf(`either there is network plugin mismatch between OpenShift node(%q) and master or OpenShift master is running an older version where we did not persist plugin name`, pluginName)
			}
		}
	}
	return nil
}
