package node

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/golang/glog"

	miekgdns "github.com/miekg/dns"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientgoclientset "k8s.io/client-go/kubernetes"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/cert"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/kubelet"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	kubeletserver "k8s.io/kubernetes/pkg/kubelet/server"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/dns"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

// NodeConfig represents the required parameters to start the OpenShift node
// through Kubernetes. All fields are required.
type NodeConfig struct {
	// BindAddress is the address to bind to
	BindAddress string
	// VolumeDir is the directory that volumes will be stored under
	VolumeDir string
	// AllowDisabledDocker if true, will make the Kubelet ignore errors from Docker
	AllowDisabledDocker bool
	// Containerized is true if we are expected to be running inside of a container
	Containerized bool

	// Client to connect to the master.
	Client kclientset.Interface
	// External kube client
	ExternalKubeClientset kclientsetexternal.Interface
	// Internal kubernetes shared informer factory.
	InternalKubeInformers kinternalinformers.SharedInformerFactory
	// DockerClient is a client to connect to Docker
	DockerClient dockertools.Interface
	// KubeletServer contains the KubeletServer configuration
	KubeletServer *kubeletoptions.KubeletServer
	// KubeletDeps are the injected code dependencies for the kubelet, fully initialized
	KubeletDeps *kubelet.KubeletDeps
	// ProxyConfig is the configuration for the kube-proxy, fully initialized
	ProxyConfig *componentconfig.KubeProxyConfiguration
	// IPTablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string
	// EnableUnidling indicates whether or not the unidling hybrid proxy should be used
	EnableUnidling bool

	// DNSConfig controls the DNS configuration.
	DNSServer *dns.Server

	// SDNNode is an optional SDN node interface
	SDNNode network.NodeInterface
	// SDNProxy is an optional service endpoints filterer
	SDNProxy network.ProxyInterface
}

func New(options configapi.NodeConfig, server *kubeletoptions.KubeletServer, proxyconfig *componentconfig.KubeProxyConfiguration, enableProxy, enableDNS bool) (*NodeConfig, error) {
	if options.NodeName == "localhost" {
		glog.Warningf(`Using "localhost" as node name will not resolve from all locations`)
	}

	clientCAs, err := cert.NewPool(options.ServingInfo.ClientCA)
	if err != nil {
		return nil, err
	}

	originClient, _, err := configapi.GetOpenShiftClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	kubeClient, privilegedKubeConfig, err := configapi.GetInternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	externalKubeClient, _, err := configapi.GetExternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	// Make a separate client for event reporting, to avoid event QPS blocking node calls
	eventClient, _, err := configapi.GetExternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	clientgoClientSet, err := clientgoclientset.NewForConfig(privilegedKubeConfig)
	if err != nil {
		return nil, err
	}

	if err = validateNetworkPluginName(originClient, options.NetworkConfig.NetworkPluginName); err != nil {
		return nil, err
	}

	internalKubeInformers := kinternalinformers.NewSharedInformerFactory(kubeClient, proxyconfig.ConfigSyncPeriod.Duration)

	// Initialize SDN before building kubelet config so it can modify option
	var sdnNode network.NodeInterface
	var sdnProxy network.ProxyInterface
	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		sdnNode, sdnProxy, err = NewSDNInterfaces(options, originClient, kubeClient, internalKubeInformers, proxyconfig)
		if err != nil {
			return nil, fmt.Errorf("SDN initialization failed: %v", err)
		}
	}

	deps, err := kubeletapp.UnsecuredKubeletDeps(server)
	if err != nil {
		return nil, err
	}

	// Initialize cloud provider
	cloud, err := buildCloudProvider(server)
	if err != nil {
		return nil, err
	}
	deps.Cloud = cloud

	// provide any config overrides
	//deps.NodeName = options.NodeName
	deps.KubeClient = externalKubeClient
	deps.EventClient = kv1core.New(eventClient.CoreV1().RESTClient())

	deps.Auth, err = kubeletapp.BuildAuth(types.NodeName(options.NodeName), clientgoClientSet, server.KubeletConfiguration)
	if err != nil {
		return nil, err
	}

	// TODO: could be cleaner
	extraCerts, err := configapi.GetNamedCertificateMap(options.ServingInfo.NamedCertificates)
	if err != nil {
		return nil, err
	}
	deps.TLSOptions = &kubeletserver.TLSOptions{
		Config: crypto.SecureTLSConfig(&tls.Config{
			// RequestClientCert lets us request certs, but allow requests without client certs
			// Verification is done by the authn layer
			ClientAuth: tls.RequestClientCert,
			ClientCAs:  clientCAs,
			// Set SNI certificate func
			// Do not use NameToCertificate, since that requires certificates be included in the server's tlsConfig.Certificates list,
			// which we do not control when running with http.Server#ListenAndServeTLS
			GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
			MinVersion:     crypto.TLSVersionOrDie(options.ServingInfo.MinTLSVersion),
			CipherSuites:   crypto.CipherSuitesOrDie(options.ServingInfo.CipherSuites),
		}),
		CertFile: options.ServingInfo.ServerCert.CertFile,
		KeyFile:  options.ServingInfo.ServerCert.KeyFile,
	}

	config := &NodeConfig{
		BindAddress: options.ServingInfo.BindAddress,

		AllowDisabledDocker: options.AllowDisabledDocker,
		Containerized:       server.Containerized,

		Client:                kubeClient,
		ExternalKubeClientset: externalKubeClient,
		InternalKubeInformers: internalKubeInformers,

		VolumeDir: options.VolumeDirectory,

		KubeletServer: server,
		KubeletDeps:   deps,

		ProxyConfig:    proxyconfig,
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
		dnsConfig.Domain = server.ClusterDomain + "."
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

func buildCloudProvider(server *kubeletoptions.KubeletServer) (cloudprovider.Interface, error) {
	if len(server.CloudProvider) == 0 || server.CloudProvider == v1alpha1.AutoDetectCloudProvider {
		return nil, nil
	}
	cloud, err := cloudprovider.InitCloudProvider(server.CloudProvider, server.CloudConfigFile)
	if err != nil {
		return nil, err
	}
	if cloud != nil {
		glog.V(2).Infof("Successfully initialized cloud provider: %q from the config file: %q", server.CloudProvider, server.CloudConfigFile)
	}
	return cloud, nil
}
