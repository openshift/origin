package node

import (
	"crypto/tls"

	"github.com/golang/glog"

	"k8s.io/client-go/util/cert"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/kubelet"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	kubeletserver "k8s.io/kubernetes/pkg/kubelet/server"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
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
	// DNSClient is a client that is only used to lookup default DNS IP addresses on
	// the cluster. It should not be passed into the Kubelet.
	DNSClient kclientsetexternal.Interface

	// DockerClient is a client to connect to Docker
	DockerClient dockertools.Interface
	// KubeletServer contains the KubeletServer configuration
	KubeletServer *kubeletoptions.KubeletServer
	// KubeletDeps are the injected code dependencies for the kubelet, fully initialized
	KubeletDeps *kubelet.KubeletDeps
}

func New(options configapi.NodeConfig, server *kubeletoptions.KubeletServer) (*NodeConfig, error) {
	if options.NodeName == "localhost" {
		glog.Warningf(`Using "localhost" as node name will not resolve from all locations`)
	}

	clientCAs, err := cert.NewPool(options.ServingInfo.ClientCA)
	if err != nil {
		return nil, err
	}

	externalKubeClient, _, err := configapi.GetExternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides)
	if err != nil {
		return nil, err
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
		DNSClient:           externalKubeClient,

		VolumeDir: options.VolumeDirectory,

		KubeletServer: server,
		KubeletDeps:   deps,
	}

	return config, nil
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
