package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	api.TypeMeta

	// NodeName is the value used to identify this particular node in the cluster.  If possible, this should be your fully qualified hostname.
	// If you're describing a set of static nodes to the master, this value must match one of the values in the list
	NodeName string

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo

	// MasterKubeConfig is a filename for the .kubeconfig file that describes how to connect this node to the master
	MasterKubeConfig string

	// domain suffix
	DNSDomain string
	// ip
	DNSIP string

	// VolumeDir is the directory that volumes will be stored under
	VolumeDirectory string

	// NetworkContainerImage is the image used as the Kubelet network namespace and volume container.
	NetworkContainerImage string

	// AllowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool

	// RecordEvents indicates whether or not to record events from the master
	RecordEvents bool
}

type MasterConfig struct {
	api.TypeMeta

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo

	// CORSAllowedOrigins
	CORSAllowedOrigins []string

	// EtcdClientInfo contains information about how to connect to etcd
	EtcdClientInfo RemoteConnectionInfo

	// KubernetesMasterConfig, if present start the kubernetes master in this process
	KubernetesMasterConfig *KubernetesMasterConfig
	// EtcdConfig, if present start etcd in this process
	EtcdConfig *EtcdConfig
	// OAuthConfig, if present start the /oauth endpoint in this process
	OAuthConfig *OAuthConfig
	// AssetConfig, if present start the asset serverin this process
	AssetConfig *AssetConfig
	// DNSConfig, if present start the DNS server in this process
	DNSConfig *DNSConfig

	// MasterClients holds all the client connection information for controllers and other system components
	MasterClients MasterClients

	ImageConfig ImageConfig

	// MasterAuthorizationNamespace is the global namespace for Policy
	MasterAuthorizationNamespace string
	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string
}

type ImageConfig struct {
	Format string
	Latest bool
}

type RemoteConnectionInfo struct {
	// URL is the URL for etcd
	URL string
	// CA is the CA for confirming that the server at the etcdURL is the actual server
	CA string
	// EtcdClientCertInfo is the TLS client cert information for securing communication to  etcd
	// this is anonymous so that we can inline it for serialization
	ClientCert CertInfo
}

type ServingInfo struct {
	// BindAddress is the ip:port to serve on
	BindAddress string
	// ServerCert is the TLS cert info for serving secure traffic
	ServerCert CertInfo
	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string
}

type MasterClients struct {
	// DeployerKubeConfig is a .kubeconfig filename for depoyment pods to use
	DeployerKubeConfig string
	// OpenShiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string
	// KubernetesKubeConfig is a .kubeconfig filename for system components to communicate to kubernetes for building the proxy
	KubernetesKubeConfig string
}

type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string
}

type AssetConfig struct {
	ServingInfo ServingInfo

	// PublicURL is where you can find the asset server (TODO do we really need this?)
	PublicURL string

	// LogoutURI is an optional, absolute URI to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURI string

	// MasterPublicURL is how the web console can access the OpenShift api server
	MasterPublicURL string

	// TODO: we probably don't need this since we have a proxy
	// KubernetesPublicURL is how the web console can access the Kubernetes api server
	KubernetesPublicURL string
}

type OAuthConfig struct {
	// ProxyCA is the certificate bundle for confirming the identity of front proxy forwards to the oauth server
	ProxyCA string

	// MasterURL is used for building valid client redirect URLs for external access
	MasterURL string

	// MasterPublicURL is used for building valid client redirect URLs for external access
	MasterPublicURL string

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string

	// all the handlers here
}

type EtcdConfig struct {
	ServingInfo ServingInfo

	PeerAddress   string
	MasterAddress string
	StorageDir    string
}

type KubernetesMasterConfig struct {
	ServicesSubnet  string
	StaticNodeNames []string
}

type CertInfo struct {
	CertFile string
	KeyFile  string
}
