package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	proxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app/options"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	kubeletserver "k8s.io/kubernetes/pkg/kubelet/server"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	kcrypto "k8s.io/kubernetes/pkg/util/crypto"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/oom"

	osdnapi "github.com/openshift/openshift-sdn/plugins/osdn/api"
	"github.com/openshift/openshift-sdn/plugins/osdn/factory"
	"github.com/openshift/openshift-sdn/plugins/osdn/ovs"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/variable"
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
	Client *client.Client
	// DockerClient is a client to connect to Docker
	DockerClient dockertools.DockerInterface
	// KubeletServer contains the KubeletServer configuration
	KubeletServer *kubeletoptions.KubeletServer
	// KubeletConfig is the configuration for the kubelet, fully initialized
	KubeletConfig *kubeletapp.KubeletConfig
	// ProxyConfig is the configuration for the kube-proxy, fully initialized
	ProxyConfig *proxyoptions.ProxyServerConfig
	// IPTablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string

	// Maximum transmission unit for the network packets
	MTU uint
	// SDNPlugin is an optional SDN plugin
	SDNPlugin osdnapi.OsdnPlugin
	// EndpointsFilterer is an optional endpoints filterer
	FilteringEndpointsHandler osdnapi.FilteringEndpointsConfigHandler
}

func BuildKubernetesNodeConfig(options configapi.NodeConfig) (*NodeConfig, error) {
	originClient, _, err := configapi.GetOpenShiftClient(options.MasterKubeConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, _, err := configapi.GetKubeClient(options.MasterKubeConfig)
	if err != nil {
		return nil, err
	}
	// Make a separate client for event reporting, to avoid event QPS blocking node calls
	eventClient, _, err := configapi.GetKubeClient(options.MasterKubeConfig)
	if err != nil {
		return nil, err
	}

	if options.NodeName == "localhost" {
		glog.Warningf(`Using "localhost" as node name will not resolve from all locations`)
	}

	clientCAs, err := kcrypto.CertPoolFromFile(options.ServingInfo.ClientCA)
	if err != nil {
		return nil, err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	var path string
	var fileCheckInterval int64
	if options.PodManifestConfig != nil {
		path = options.PodManifestConfig.Path
		fileCheckInterval = options.PodManifestConfig.FileCheckIntervalSeconds
	}

	var dockerExecHandler dockertools.ExecHandler

	switch options.DockerConfig.ExecHandlerName {
	case configapi.DockerExecHandlerNative:
		dockerExecHandler = &dockertools.NativeExecHandler{}
	case configapi.DockerExecHandlerNsenter:
		dockerExecHandler = &dockertools.NsenterExecHandler{}
	}

	kubeAddressStr, kubePortStr, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node address: %v", err)
	}
	kubePort, err := strconv.Atoi(kubePortStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node port: %v", err)
	}

	// Defaults are tested in TestKubeletDefaults
	server := kubeletoptions.NewKubeletServer()
	// Adjust defaults
	server.Config = path
	server.RootDirectory = options.VolumeDirectory
	server.NodeIP = options.NodeIP
	server.HostnameOverride = options.NodeName
	server.AllowPrivileged = true
	server.RegisterNode = true
	server.Address = kubeAddressStr
	server.Port = uint(kubePort)
	server.ReadOnlyPort = 0        // no read only access
	server.CAdvisorPort = 0        // no unsecured cadvisor access
	server.HealthzPort = 0         // no unsecured healthz access
	server.HealthzBindAddress = "" // no unsecured healthz access
	server.ClusterDNS = options.DNSIP
	server.ClusterDomain = options.DNSDomain
	server.NetworkPluginName = options.NetworkConfig.NetworkPluginName
	server.HostNetworkSources = strings.Join([]string{kubelettypes.ApiserverSource, kubelettypes.FileSource}, ",")
	server.HostPIDSources = strings.Join([]string{kubelettypes.ApiserverSource, kubelettypes.FileSource}, ",")
	server.HostIPCSources = strings.Join([]string{kubelettypes.ApiserverSource, kubelettypes.FileSource}, ",")
	server.HTTPCheckFrequency = unversioned.Duration{Duration: time.Duration(0)} // no remote HTTP pod creation access
	server.FileCheckFrequency = unversioned.Duration{Duration: time.Duration(fileCheckInterval) * time.Second}
	server.PodInfraContainerImage = imageTemplate.ExpandOrDie("pod")
	server.CPUCFSQuota = true // enable cpu cfs quota enforcement by default
	server.MaxPods = 110

	switch server.NetworkPluginName {
	case ovs.SingleTenantPluginName(), ovs.MultiTenantPluginName():
		// set defaults for openshift-sdn
		server.HairpinMode = componentconfig.HairpinNone
		server.ConfigureCBR0 = false
	}

	// prevents kube from generating certs
	server.TLSCertFile = options.ServingInfo.ServerCert.CertFile
	server.TLSPrivateKeyFile = options.ServingInfo.ServerCert.KeyFile

	containerized := cmdutil.Env("OPENSHIFT_CONTAINERIZED", "") == "true"
	server.Containerized = containerized

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubeletArguments, server.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	proxyconfig, err := buildKubeProxyConfig(options)
	if err != nil {
		return nil, err
	}

	cfg, err := kubeletapp.UnsecuredKubeletConfig(server)
	if err != nil {
		return nil, err
	}

	// provide any config overrides
	cfg.NodeName = options.NodeName
	cfg.KubeClient = clientadapter.FromUnversionedClient(kubeClient)
	cfg.EventClient = clientadapter.FromUnversionedClient(eventClient)
	cfg.DockerExecHandler = dockerExecHandler

	// docker-in-docker (dind) deployments are used for testing
	// networking plugins.  Running openshift under dind won't work
	// with the real oom adjuster due to the state of the cgroups path
	// in a dind container that uses systemd for init.  Similarly,
	// cgroup manipulation of the nested docker daemon doesn't work
	// properly under centos/rhel and should be disabled by setting
	// the name of the container to an empty string.
	//
	// This workaround should become unnecessary once user namespaces
	if value := cmdutil.Env("OPENSHIFT_DIND", ""); value == "true" {
		glog.Warningf("Using FakeOOMAdjuster for docker-in-docker compatibility")
		cfg.OOMAdjuster = oom.NewFakeOOMAdjuster()
	}

	// Setup auth
	osClient, osClientConfig, err := configapi.GetOpenShiftClient(options.MasterKubeConfig)
	if err != nil {
		return nil, err
	}
	authnTTL, err := time.ParseDuration(options.AuthConfig.AuthenticationCacheTTL)
	if err != nil {
		return nil, err
	}
	authn, err := newAuthenticator(clientCAs, clientcmd.AnonymousClientConfig(osClientConfig), authnTTL, options.AuthConfig.AuthenticationCacheSize)
	if err != nil {
		return nil, err
	}

	authzAttr, err := newAuthorizerAttributesGetter(options.NodeName)
	if err != nil {
		return nil, err
	}

	authzTTL, err := time.ParseDuration(options.AuthConfig.AuthorizationCacheTTL)
	if err != nil {
		return nil, err
	}
	authz, err := newAuthorizer(osClient, authzTTL, options.AuthConfig.AuthorizationCacheSize)
	if err != nil {
		return nil, err
	}

	cfg.Auth = kubeletserver.NewKubeletAuth(authn, authzAttr, authz)

	// Make sure the node doesn't think it is in standalone mode
	// This is required for the node to enforce nodeSelectors on pods, to set hostIP on pod status updates, etc
	cfg.StandaloneMode = false

	// TODO: could be cleaner
	if configapi.UseTLS(options.ServingInfo) {
		extraCerts, err := configapi.GetNamedCertificateMap(options.ServingInfo.NamedCertificates)
		if err != nil {
			return nil, err
		}
		cfg.TLSOptions = &kubeletserver.TLSOptions{
			Config: crypto.SecureTLSConfig(&tls.Config{
				// RequestClientCert lets us request certs, but allow requests without client certs
				// Verification is done by the authn layer
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  clientCAs,
				// Set SNI certificate func
				// Do not use NameToCertificate, since that requires certificates be included in the server's tlsConfig.Certificates list,
				// which we do not control when running with http.Server#ListenAndServeTLS
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
			}),
			CertFile: options.ServingInfo.ServerCert.CertFile,
			KeyFile:  options.ServingInfo.ServerCert.KeyFile,
		}
	} else {
		cfg.TLSOptions = nil
	}

	// Prepare cloud provider
	cloud, err := cloudprovider.InitCloudProvider(server.CloudProvider, server.CloudConfigFile)
	if err != nil {
		return nil, err
	}
	if cloud != nil {
		glog.V(2).Infof("Successfully initialized cloud provider: %q from the config file: %q\n", server.CloudProvider, server.CloudConfigFile)
	}
	cfg.Cloud = cloud

	sdnPlugin, endpointFilter, err := factory.NewPlugin(options.NetworkConfig.NetworkPluginName, originClient, kubeClient, options.NodeName, options.NodeIP)
	if err != nil {
		return nil, fmt.Errorf("SDN initialization failed: %v", err)
	}
	if sdnPlugin != nil {
		cfg.NetworkPlugins = append(cfg.NetworkPlugins, sdnPlugin)
	}

	config := &NodeConfig{
		BindAddress: options.ServingInfo.BindAddress,

		AllowDisabledDocker: options.AllowDisabledDocker,
		Containerized:       containerized,

		Client: kubeClient,

		VolumeDir: options.VolumeDirectory,

		KubeletServer: server,
		KubeletConfig: cfg,

		ProxyConfig: proxyconfig,

		MTU: options.NetworkConfig.MTU,

		SDNPlugin:                 sdnPlugin,
		FilteringEndpointsHandler: endpointFilter,
	}

	return config, nil
}

func buildKubeProxyConfig(options configapi.NodeConfig) (*proxyoptions.ProxyServerConfig, error) {
	// get default config
	proxyconfig := proxyoptions.NewProxyConfig()

	// BindAddress - Override default bind address from our config
	addr := options.ServingInfo.BindAddress
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("The provided value to bind to must be an ip:port %q", addr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("The provided value to bind to must be an ip:port: %q", addr)
	}
	proxyconfig.BindAddress = ip.String()

	// HealthzPort, HealthzBindAddress - disable
	proxyconfig.HealthzPort = 0
	proxyconfig.HealthzBindAddress = ""

	// OOMScoreAdj, ResourceContainer - clear, we don't run in a container
	oomScoreAdj := 0
	proxyconfig.OOMScoreAdj = &oomScoreAdj
	proxyconfig.ResourceContainer = ""

	// use the same client as the node
	proxyconfig.Master = ""
	proxyconfig.Kubeconfig = options.MasterKubeConfig

	// PortRange, use default
	// HostnameOverride, use default

	// ProxyMode, set to iptables
	proxyconfig.Mode = "iptables"

	// IptablesSyncPeriod, set to our config value
	syncPeriod, err := time.ParseDuration(options.IPTablesSyncPeriod)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse the provided ip-tables sync period (%s) : %v", options.IPTablesSyncPeriod, err)
	}
	proxyconfig.IPTablesSyncPeriod = unversioned.Duration{
		Duration: syncPeriod,
	}

	// ConfigSyncPeriod, use default

	// NodeRef, build from config
	proxyconfig.NodeRef = &kapi.ObjectReference{
		Kind: "Node",
		Name: options.NodeName,
	}

	// MasqueradeAll, use default

	// CleanupAndExit, use default

	// KubeAPIQPS, use default, doesn't apply until we build a separate client
	// KubeAPIBurst, use default, doesn't apply until we build a separate client

	// UDPIdleTimeout, use default

	// Resolve cmd flags to add any user overrides
	if err := cmdflags.Resolve(options.ProxyArguments, proxyconfig.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	return proxyconfig, nil
}
