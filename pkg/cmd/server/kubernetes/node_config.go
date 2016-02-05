package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	kubeletserver "k8s.io/kubernetes/pkg/kubelet/server"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/oom"

	osdnapi "github.com/openshift/openshift-sdn/plugins/osdn/api"
	"github.com/openshift/openshift-sdn/plugins/osdn/factory"
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
	// Client to connect to the master.
	Client *client.Client
	// DockerClient is a client to connect to Docker
	DockerClient dockertools.DockerInterface
	// KubeletServer contains the KubeletServer configuration
	KubeletServer *kubeletoptions.KubeletServer
	// KubeletConfig is the configuration for the kubelet, fully initialized
	KubeletConfig *kubeletapp.KubeletConfig
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

	clientCAs, err := util.CertPoolFromFile(options.ServingInfo.ClientCA)
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

	// declare the OpenShift defaults from config
	server := kubeletoptions.NewKubeletServer()
	server.Config = path
	server.RootDirectory = options.VolumeDirectory
	server.NodeIP = options.NodeIP
	server.HostnameOverride = options.NodeName
	server.AllowPrivileged = true
	server.RegisterNode = true
	server.Address = kubeAddressStr
	server.Port = uint(kubePort)
	server.ReadOnlyPort = 0 // no read only access
	server.CAdvisorPort = 0 // no unsecured cadvisor access
	server.HealthzPort = 0  // no unsecured healthz access
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

	// prevents kube from generating certs
	server.TLSCertFile = options.ServingInfo.ServerCert.CertFile
	server.TLSPrivateKeyFile = options.ServingInfo.ServerCert.KeyFile

	if value := cmdutil.Env("OPENSHIFT_CONTAINERIZED", ""); len(value) > 0 {
		server.Containerized = value == "true"
	}

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubeletArguments, server.AddFlags); len(err) > 0 {
		return nil, errors.NewAggregate(err)
	}

	cfg, err := kubeletapp.UnsecuredKubeletConfig(server)
	if err != nil {
		return nil, err
	}

	// provide any config overrides
	cfg.NodeName = options.NodeName
	cfg.KubeClient = kubeClient
	cfg.EventClient = eventClient
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
		glog.Warningf("Disabling cgroup manipulation of nested docker daemon for docker-in-docker compatibility")
		cfg.DockerDaemonContainer = ""
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
	authn, err := newAuthenticator(clientCAs, clientcmd.AnonymousClientConfig(*osClientConfig), authnTTL, options.AuthConfig.AuthenticationCacheSize)
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
	} else if sdnPlugin != nil {
		cfg.NetworkPlugins = append(cfg.NetworkPlugins, sdnPlugin)
	}

	config := &NodeConfig{
		BindAddress: options.ServingInfo.BindAddress,

		AllowDisabledDocker: options.AllowDisabledDocker,

		Client: kubeClient,

		VolumeDir: options.VolumeDirectory,

		KubeletServer: server,
		KubeletConfig: cfg,

		IPTablesSyncPeriod: options.IPTablesSyncPeriod,
		MTU:                options.NetworkConfig.MTU,

		SDNPlugin:                 sdnPlugin,
		FilteringEndpointsHandler: endpointFilter,
	}

	return config, nil
}
