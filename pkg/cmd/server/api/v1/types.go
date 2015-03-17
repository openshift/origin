package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
)

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	v1beta3.TypeMeta `json:",inline"`

	// NodeName is the value used to identify this particular node in the cluster.  If possible, this should be your fully qualified hostname.
	// If you're describing a set of static nodes to the master, this value must match one of the values in the list
	NodeName string `json:"nodeName"`

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo `json:"servingInfo"`

	// MasterKubeConfig is a filename for the .kubeconfig file that describes how to connect this node to the master
	MasterKubeConfig string `json:"masterKubeConfig"`

	// domain suffix
	DNSDomain string `json:"dnsDomain"`
	// ip
	DNSIP string `json:"dnsIP"`

	// VolumeDir is the directory that volumes will be stored under
	VolumeDirectory string `json:"volumeDirectory"`

	// NetworkContainerImage is the image used as the Kubelet network namespace and volume container.
	NetworkContainerImage string `json:"networkContainerImage"`

	// AllowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool `json:"allowDisabledDocker"`

	// RecordEvents indicates whether or not to record events from the master
	RecordEvents bool `json:"recordEvents"`
}

type MasterConfig struct {
	v1beta3.TypeMeta `json:",inline"`

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo `json:"servingInfo"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins"`

	// EtcdClientInfo contains information about how to connect to etcd
	EtcdClientInfo RemoteConnectionInfo `json:"etcdClientInfo"`

	// KubernetesMasterConfig, if present start the kubernetes master in this process
	KubernetesMasterConfig *KubernetesMasterConfig `json:"kubernetesMasterConfig"`
	// EtcdConfig, if present start etcd in this process
	EtcdConfig *EtcdConfig `json:"etcdConfig"`
	// OAuthConfig, if present start the /oauth endpoint in this process
	OAuthConfig *OAuthConfig `json:"oauthConfig"`
	// AssetConfig, if present start the asset serverin this process
	AssetConfig *AssetConfig `json:"assetConfig"`
	// DNSConfig, if present start the DNS server in this process
	DNSConfig *DNSConfig `json:"dnsConfig"`

	// MasterClients holds all the client connection information for controllers and other system components
	MasterClients MasterClients `json:"masterClients"`

	ImageConfig ImageConfig `json:"imageConfig"`

	// MasterAuthorizationNamespace is the global namespace for Policy
	MasterAuthorizationNamespace string `json:"masterAuthorizationNamespace"`
	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string `json:"openshiftSharedResourcesNamespace"`
}

type ImageConfig struct {
	Format string `json:"format"`
	Latest bool   `json:"latest"`
}

type RemoteConnectionInfo struct {
	// URL is the URL for etcd
	URL string `json:"url"`
	// CA is the CA for confirming that the server at the etcdURL is the actual server
	CA string `json:"ca"`
	// EtcdClientCertInfo is the TLS client cert information for securing communication to  etcd
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

type ServingInfo struct {
	// BindAddress is the ip:port to serve on
	BindAddress string `json:"bindAddress"`
	// ServerCert is the TLS cert info for serving secure traffic.
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string `json:"clientCA"`
}

type MasterClients struct {
	// DeployerKubeConfig is a .kubeconfig filename for depoyment pods to use
	DeployerKubeConfig string `json:"deployerKubeConfig"`
	// OpenShiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string `json:"openshiftLoopbackKubeConfig"`
	// KubernetesKubeConfig is a .kubeconfig filename for system components to communicate to kubernetes for building the proxy
	KubernetesKubeConfig string `json:"kubernetesKubeConfig"`
}

type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string `json:"bindAddress"`
}

type AssetConfig struct {
	ServingInfo ServingInfo `json:"servingInfo"`

	// PublicURL is where you can find the asset server (TODO do we really need this?)
	PublicURL string `json:"publicURL"`

	// LogoutURI is an optional, absolute URI to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURI string `json:"logoutURI"`

	// MasterPublicURL is how the web console can access the OpenShift api server
	MasterPublicURL string `json:"masterPublicURL"`

	// TODO: we probably don't need this since we have a proxy
	// KubernetesPublicURL is how the web console can access the Kubernetes api server
	KubernetesPublicURL string `json:"kubernetesPublicURL"`
}

type OAuthConfig struct {
	// ProxyCA is the certificate bundle for confirming the identity of front proxy forwards to the oauth server
	ProxyCA string `json:"proxyCA"`

	// MasterURL is used for building valid client redirect URLs for external access
	MasterURL string `json:"masterURL"`

	// MasterPublicURL is used for building valid client redirect URLs for external access
	MasterPublicURL string `json:"masterPublicURL"`

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string `json:"assetPublicURL"`

	// all the handlers here
}

type EtcdConfig struct {
	ServingInfo ServingInfo `json:"servingInfo"`

	PeerAddress   string `json:"peerAddress"`
	MasterAddress string `json:"masterAddress"`
	StorageDir    string `json:"storageDirectory"`
}

type KubernetesMasterConfig struct {
	ServicesSubnet  string   `json:"servicesSubnet"`
	StaticNodeNames []string `json:"staticNodeNames"`
}

type CertInfo struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}
