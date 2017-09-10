package node

import (
	"fmt"
	"net"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kubeproxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeletcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/network"
)

// Build creates the core Kubernetes component configs for a given NodeConfig, or returns
// an error
func Build(options configapi.NodeConfig) (*kubeletoptions.KubeletServer, *componentconfig.KubeProxyConfiguration, error) {
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	var path string
	var fileCheckInterval int64
	if options.PodManifestConfig != nil {
		path = options.PodManifestConfig.Path
		fileCheckInterval = options.PodManifestConfig.FileCheckIntervalSeconds
	}

	kubeAddressStr, kubePortStr, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse node address: %v", err)
	}
	kubePort, err := strconv.Atoi(kubePortStr)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse node port: %v", err)
	}

	// Defaults are tested in TestKubeletDefaults
	server := kubeletoptions.NewKubeletServer()
	// Adjust defaults
	server.RequireKubeConfig = true
	server.KubeConfig.Default(options.MasterKubeConfig)
	server.PodManifestPath = path
	server.RootDirectory = options.VolumeDirectory
	server.NodeIP = options.NodeIP
	server.HostnameOverride = options.NodeName
	server.AllowPrivileged = true
	server.RegisterNode = true
	server.Address = kubeAddressStr
	server.Port = int32(kubePort)
	server.ReadOnlyPort = 0        // no read only access
	server.CAdvisorPort = 0        // no unsecured cadvisor access
	server.HealthzPort = 0         // no unsecured healthz access
	server.HealthzBindAddress = "" // no unsecured healthz access
	server.ClusterDNS = []string{options.DNSIP}
	server.ClusterDomain = options.DNSDomain
	server.NetworkPluginName = options.NetworkConfig.NetworkPluginName
	server.HostNetworkSources = []string{kubelettypes.ApiserverSource, kubelettypes.FileSource}
	server.HostPIDSources = []string{kubelettypes.ApiserverSource, kubelettypes.FileSource}
	server.HostIPCSources = []string{kubelettypes.ApiserverSource, kubelettypes.FileSource}
	server.HTTPCheckFrequency = metav1.Duration{Duration: time.Duration(0)} // no remote HTTP pod creation access
	server.FileCheckFrequency = metav1.Duration{Duration: time.Duration(fileCheckInterval) * time.Second}
	server.KubeletFlags.ContainerRuntimeOptions.PodSandboxImage = imageTemplate.ExpandOrDie("pod")
	server.LowDiskSpaceThresholdMB = 256 // this the previous default
	server.CPUCFSQuota = true            // enable cpu cfs quota enforcement by default
	server.MaxPods = 250
	server.PodsPerCore = 10
	server.CgroupDriver = "systemd"
	server.DockerExecHandlerName = string(options.DockerConfig.ExecHandlerName)
	server.RemoteRuntimeEndpoint = options.DockerConfig.DockerShimSocket
	server.RemoteImageEndpoint = options.DockerConfig.DockerShimSocket
	server.DockershimRootDirectory = options.DockerConfig.DockershimRootDirectory

	// prevents kube from generating certs
	server.TLSCertFile = options.ServingInfo.ServerCert.CertFile
	server.TLSPrivateKeyFile = options.ServingInfo.ServerCert.KeyFile

	containerized := cmdutil.Env("OPENSHIFT_CONTAINERIZED", "") == "true"
	server.Containerized = containerized

	// force the authentication and authorization
	// Setup auth
	authnTTL, err := time.ParseDuration(options.AuthConfig.AuthenticationCacheTTL)
	if err != nil {
		return nil, nil, err
	}
	server.Authentication = componentconfig.KubeletAuthentication{
		X509: componentconfig.KubeletX509Authentication{
			ClientCAFile: options.ServingInfo.ClientCA,
		},
		Webhook: componentconfig.KubeletWebhookAuthentication{
			Enabled:  true,
			CacheTTL: metav1.Duration{Duration: authnTTL},
		},
		Anonymous: componentconfig.KubeletAnonymousAuthentication{
			Enabled: true,
		},
	}
	authzTTL, err := time.ParseDuration(options.AuthConfig.AuthorizationCacheTTL)
	if err != nil {
		return nil, nil, err
	}
	server.Authorization = componentconfig.KubeletAuthorization{
		Mode: componentconfig.KubeletAuthorizationModeWebhook,
		Webhook: componentconfig.KubeletWebhookAuthorization{
			CacheAuthorizedTTL:   metav1.Duration{Duration: authzTTL},
			CacheUnauthorizedTTL: metav1.Duration{Duration: authzTTL},
		},
	}

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubeletArguments, server.AddFlags); len(err) > 0 {
		return nil, nil, kerrors.NewAggregate(err)
	}

	proxyconfig, err := buildKubeProxyConfig(options)
	if err != nil {
		return nil, nil, err
	}

	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		// SDN plugin pod setup/teardown is implemented as a CNI plugin
		server.NetworkPluginName = kubeletcni.CNIPluginName
		server.NetworkPluginDir = kubeletcni.DefaultNetDir
		server.CNIConfDir = kubeletcni.DefaultNetDir
		server.CNIBinDir = kubeletcni.DefaultCNIDir
		server.HairpinMode = componentconfig.HairpinNone
	}

	return server, proxyconfig, nil
}

func buildKubeProxyConfig(options configapi.NodeConfig) (*componentconfig.KubeProxyConfiguration, error) {
	proxyOptions, err := kubeproxyoptions.NewOptions()
	if err != nil {
		return nil, err
	}
	// get default config
	proxyconfig := proxyOptions.GetConfig()

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
	proxyconfig.HealthzBindAddress = ""
	proxyconfig.MetricsBindAddress = ""

	// OOMScoreAdj, ResourceContainer - clear, we don't run in a container
	oomScoreAdj := int32(0)
	proxyconfig.OOMScoreAdj = &oomScoreAdj
	proxyconfig.ResourceContainer = ""

	// use the same client as the node
	proxyconfig.ClientConnection.KubeConfigFile = options.MasterKubeConfig

	// ProxyMode, set to iptables
	proxyconfig.Mode = "iptables"

	// IptablesSyncPeriod, set to our config value
	syncPeriod, err := time.ParseDuration(options.IPTablesSyncPeriod)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse the provided ip-tables sync period (%s) : %v", options.IPTablesSyncPeriod, err)
	}
	proxyconfig.IPTables.SyncPeriod = metav1.Duration{
		Duration: syncPeriod,
	}
	masqueradeBit := int32(0)
	proxyconfig.IPTables.MasqueradeBit = &masqueradeBit

	// PortRange, use default
	// HostnameOverride, use default
	// ConfigSyncPeriod, use default
	// MasqueradeAll, use default
	// CleanupAndExit, use default
	// KubeAPIQPS, use default, doesn't apply until we build a separate client
	// KubeAPIBurst, use default, doesn't apply until we build a separate client
	// UDPIdleTimeout, use default

	// Resolve cmd flags to add any user overrides
	if err := cmdflags.Resolve(options.ProxyArguments, proxyOptions.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	return proxyconfig, nil
}
