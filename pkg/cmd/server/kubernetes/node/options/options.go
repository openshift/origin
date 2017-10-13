package node

import (
	"fmt"
	"net"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/features"
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
func Build(options configapi.NodeConfig) (*kubeletoptions.KubeletServer, error) {
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
		return nil, fmt.Errorf("cannot parse node address: %v", err)
	}
	kubePort, err := strconv.Atoi(kubePortStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node port: %v", err)
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
		return nil, err
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
		return nil, err
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
		return nil, kerrors.NewAggregate(err)
	}

	// terminate early if feature gate is incorrect on the node
	if len(server.FeatureGates) > 0 {
		if err := utilfeature.DefaultFeatureGate.Set(server.FeatureGates); err != nil {
			return nil, err
		}
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.RotateKubeletServerCertificate) {
		// Server cert rotation is ineffective if a cert is hardcoded.
		if len(server.CertDirectory) > 0 {
			server.TLSCertFile = ""
			server.TLSPrivateKeyFile = ""
		}
	}

	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		// SDN plugin pod setup/teardown is implemented as a CNI plugin
		server.NetworkPluginName = kubeletcni.CNIPluginName
		server.NetworkPluginDir = kubeletcni.DefaultNetDir
		server.CNIConfDir = kubeletcni.DefaultNetDir
		server.CNIBinDir = kubeletcni.DefaultCNIDir
		server.HairpinMode = componentconfig.HairpinNone
	}

	return server, nil
}

func ToFlags(config *kubeletoptions.KubeletServer) []string {
	return cmdflags.AsArgs(config.AddFlags, kubeletoptions.NewKubeletServer().AddFlags)
}
