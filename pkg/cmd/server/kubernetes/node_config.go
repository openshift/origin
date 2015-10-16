package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	kapp "k8s.io/kubernetes/cmd/kubelet/app"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubelet"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/errors"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
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
	KubeletServer *kapp.KubeletServer
	// KubeletConfig is the configuration for the kubelet, fully initialized
	KubeletConfig *kapp.KubeletConfig
	// IPTablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string
}

func BuildKubernetesNodeConfig(options configapi.NodeConfig) (*NodeConfig, error) {
	kubeClient, _, err := configapi.GetKubeClient(options.MasterKubeConfig)
	if err != nil {
		return nil, err
	}

	if options.NodeName == "localhost" {
		glog.Warningf(`Using "localhost" as node name will not resolve from all locations`)
	}

	var dnsIP net.IP
	if len(options.DNSIP) > 0 {
		dnsIP = net.ParseIP(options.DNSIP)
		if dnsIP == nil {
			return nil, fmt.Errorf("Invalid DNS IP: %s", options.DNSIP)
		}
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
	kubeAddress := net.ParseIP(kubeAddressStr)
	if kubeAddress == nil {
		return nil, fmt.Errorf("Invalid DNS IP: %s", kubeAddressStr)
	}

	// declare the OpenShift defaults from config
	server := kapp.NewKubeletServer()
	server.Config = path
	server.RootDirectory = options.VolumeDirectory

	// kubelet finds the node IP address by doing net.ParseIP(hostname) and if that fails,
	// it does net.LookupIP(NodeName) and picks the first non-loopback address.
	// Pass node IP as hostname to make kubelet use the desired IP address.
	if len(options.NodeIP) > 0 {
		server.HostnameOverride = options.NodeIP
	} else {
		server.HostnameOverride = options.NodeName
	}
	server.AllowPrivileged = true
	server.RegisterNode = true
	server.Address = kubeAddress
	server.Port = uint(kubePort)
	server.ReadOnlyPort = 0 // no read only access
	server.CadvisorPort = 0 // no unsecured cadvisor access
	server.HealthzPort = 0  // no unsecured healthz access
	server.ClusterDNS = dnsIP
	server.ClusterDomain = options.DNSDomain
	server.NetworkPluginName = options.NetworkConfig.NetworkPluginName
	server.HostNetworkSources = strings.Join([]string{kubelet.ApiserverSource, kubelet.FileSource}, ",")
	server.HostPIDSources = strings.Join([]string{kubelet.ApiserverSource, kubelet.FileSource}, ",")
	server.HostIPCSources = strings.Join([]string{kubelet.ApiserverSource, kubelet.FileSource}, ",")
	server.HTTPCheckFrequency = 0 // no remote HTTP pod creation access
	server.FileCheckFrequency = time.Duration(fileCheckInterval) * time.Second
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

	cfg, err := server.UnsecuredKubeletConfig()
	if err != nil {
		return nil, err
	}

	// provide any config overrides
	cfg.NodeName = options.NodeName
	cfg.StreamingConnectionIdleTimeout = 5 * time.Minute // TODO: should be set
	cfg.KubeClient = kubeClient
	cfg.DockerExecHandler = dockerExecHandler

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

	cfg.Auth = kubelet.NewKubeletAuth(authn, authzAttr, authz)

	// Make sure the node doesn't think it is in standalone mode
	// This is required for the node to enforce nodeSelectors on pods, to set hostIP on pod status updates, etc
	cfg.StandaloneMode = false

	// TODO: could be cleaner
	if configapi.UseTLS(options.ServingInfo) {
		extraCerts, err := configapi.GetNamedCertificateMap(options.ServingInfo.NamedCertificates)
		if err != nil {
			return nil, err
		}
		cfg.TLSOptions = &kubelet.TLSOptions{
			Config: &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// RequestClientCert lets us request certs, but allow requests without client certs
				// Verification is done by the authn layer
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  clientCAs,
				// Set SNI certificate func
				// Do not use NameToCertificate, since that requires certificates be included in the server's tlsConfig.Certificates list,
				// which we do not control when running with http.Server#ListenAndServeTLS
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
			},
			CertFile: options.ServingInfo.ServerCert.CertFile,
			KeyFile:  options.ServingInfo.ServerCert.KeyFile,
		}
	} else {
		cfg.TLSOptions = nil
	}

	config := &NodeConfig{
		BindAddress: options.ServingInfo.BindAddress,

		AllowDisabledDocker: options.AllowDisabledDocker,

		Client: kubeClient,

		VolumeDir: options.VolumeDirectory,

		KubeletServer: server,
		KubeletConfig: cfg,

		IPTablesSyncPeriod: options.IPTablesSyncPeriod,
	}

	return config, nil
}
