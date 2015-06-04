package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	kapp "github.com/GoogleCloudPlatform/kubernetes/cmd/kubelet/app"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// NodeConfig represents the required parameters to start the OpenShift node
// through Kubernetes. All fields are required.
type NodeConfig struct {
	// The address to bind to
	BindAddress string
	// The directory that volumes will be stored under
	VolumeDir string
	// If true, the Kubelet will ignore errors from Docker
	AllowDisabledDocker bool
	// A client to connect to the master.
	Client *client.Client
	// A client to connect to Docker
	DockerClient dockertools.DockerInterface

	// The KubeletServer configuration
	KubeletServer *kapp.KubeletServer
	// The configuration for the kubelet, fully initialized
	KubeletConfig *kapp.KubeletConfig
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

	kubeAddress, kubePortStr, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node address: %v", err)
	}
	kubePort, err := strconv.Atoi(kubePortStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node port: %v", err)
	}

	address := util.IP{}
	if err := address.Set(kubeAddress); err != nil {
		return nil, err
	}

	// declare the OpenShift defaults from config
	server := kapp.NewKubeletServer()
	server.Config = path
	server.RootDirectory = options.VolumeDirectory
	server.HostnameOverride = options.NodeName
	server.AllowPrivileged = true
	server.RegisterNode = true
	server.Address = address
	server.Port = uint(kubePort)
	server.ReadOnlyPort = 0 // no read only access
	server.ClusterDNS = util.IP(dnsIP)
	server.ClusterDomain = options.DNSDomain
	server.NetworkPluginName = options.NetworkPluginName
	server.HostNetworkSources = strings.Join([]string{kubelet.ApiserverSource, kubelet.FileSource}, ",")
	server.HTTPCheckFrequency = 0 // no remote HTTP pod creation access
	server.FileCheckFrequency = time.Duration(fileCheckInterval) * time.Second
	server.PodInfraContainerImage = imageTemplate.ExpandOrDie("pod")

	// prevents kube from generating certs
	server.TLSCertFile = options.ServingInfo.ServerCert.CertFile
	server.TLSPrivateKeyFile = options.ServingInfo.ServerCert.KeyFile

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubeletArguments, server.AddFlags); len(err) > 0 {
		return nil, errors.NewAggregate(err)
	}

	cfg, err := server.KubeletConfig()
	if err != nil {
		return nil, err
	}

	// provide any config overrides
	cfg.StreamingConnectionIdleTimeout = 5 * time.Minute // TODO: should be set
	cfg.KubeClient = kubeClient
	cfg.DockerExecHandler = dockerExecHandler

	// TODO: could be cleaner
	if configapi.UseTLS(options.ServingInfo) {
		cfg.TLSOptions = &kubelet.TLSOptions{
			Config: &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// RequireAndVerifyClientCert lets us limit requests to ones with a valid client certificate
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  clientCAs,
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
	}

	return config, nil
}
