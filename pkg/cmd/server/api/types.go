package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
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

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig

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
	EtcdClientInfo EtcdConnectionInfo
	// KubeletClientInfo contains information about how to connect to kubelets
	KubeletClientInfo KubeletConnectionInfo

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

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig

	// PolicyConfig holds information about where to locate critical pieces of bootstrapping policy
	PolicyConfig PolicyConfig
}

type PolicyConfig struct {
	// BootstrapPolicyFile points to a template that contains roles and rolebindings that will be created if no policy object exists in the master namespace
	BootstrapPolicyFile string

	// MasterAuthorizationNamespace is the global namespace for Policy
	MasterAuthorizationNamespace string
	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string
}

type ImageConfig struct {
	// Format describes how to determine image names for system components
	Format string
	// Latest indicates whether to attempt to use the latest system component images as opposed to latest release
	Latest bool
}

type RemoteConnectionInfo struct {
	// URL is the remote URL to connect to
	URL string
	// CA is the CA for verifying TLS connections
	CA string
	// CertInfo is the TLS client cert information to present
	ClientCert CertInfo
}

type KubeletConnectionInfo struct {
	// Port is the port to connect to kubelets on
	Port uint
	// CA is the CA for verifying TLS connections to kubelets
	CA string
	// CertInfo is the TLS client cert information for securing communication to kubelets
	ClientCert CertInfo
}

type EtcdConnectionInfo struct {
	// URLs are the URLs for etcd
	URLs []string
	// CA is a file containing trusted roots for the etcd server certificates
	CA string
	// ClientCert is the TLS client cert information for securing communication to etcd
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

	// LogoutURL is an optional, absolute URL to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURL string

	// MasterPublicURL is how the web console can access the OpenShift api server
	MasterPublicURL string
}

type OAuthConfig struct {
	// MasterURL is used for building valid client redirect URLs for internal access
	MasterURL string

	// MasterPublicURL is used for building valid client redirect URLs for external access
	MasterPublicURL string

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string

	//IdentityProviders is an ordered list of ways for a user to identify themselves
	IdentityProviders []IdentityProvider

	// GrantConfig describes how to handle grants
	GrantConfig GrantConfig

	// SessionConfig hold information about configuring sessions.
	SessionConfig *SessionConfig

	TokenConfig TokenConfig
}

type TokenConfig struct {
	// Max age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32
	// Max age of access tokens
	AccessTokenMaxAgeSeconds int32
}

type SessionConfig struct {
	// SessionSecrets list the secret(s) to use to encrypt created sessions. Used by AuthRequestHandlerSession
	SessionSecrets []string
	// SessionMaxAgeSeconds specifies how long created sessions last. Used by AuthRequestHandlerSession
	SessionMaxAgeSeconds int32
	// SessionName is the cookie name used to store the session
	SessionName string
}

type IdentityProvider struct {
	// Name is used to qualify the identities returned by this provider
	Name string
	// UseAsChallenger indicates whether to issue WWW-Authenticate challenges for this provider
	UseAsChallenger bool
	// UseAsLogin indicates whether to use this identity provider for unauthenticated browsers to login against
	UseAsLogin bool
	// Provider contains the information about how to set up a specific identity provider
	Provider runtime.EmbeddedObject
}

type BasicAuthPasswordIdentityProvider struct {
	api.TypeMeta

	// RemoteConnectionInfo contains information about how to connect to the external basic auth server
	RemoteConnectionInfo RemoteConnectionInfo
}

type AllowAllPasswordIdentityProvider struct {
	api.TypeMeta
}

type DenyAllPasswordIdentityProvider struct {
	api.TypeMeta
}

type HTPasswdPasswordIdentityProvider struct {
	api.TypeMeta

	// File is a reference to your htpasswd file
	File string
}

type RequestHeaderIdentityProvider struct {
	api.TypeMeta

	// ClientCA is a file with the trusted signer certs.  If empty, no request verification is done, and any direct request to the OAuth server can impersonate any identity from this provider, merely by setting a request header.
	ClientCA string
	// Headers is the set of headers to check for identity information
	Headers []string
}

type OAuthRedirectingIdentityProvider struct {
	api.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret string

	// Provider contains the information about exactly which kind of oauth you're identifying with
	Provider runtime.EmbeddedObject
}

type GoogleOAuthProvider struct {
	api.TypeMeta
}
type GitHubOAuthProvider struct {
	api.TypeMeta
}

type GrantConfig struct {
	// Method: allow, deny, prompt
	Method GrantHandlerType
}

type GrantHandlerType string

const (
	// GrantHandlerAuto auto-approves client authorization grant requests
	GrantHandlerAuto GrantHandlerType = "auto"
	// GrantHandlerPrompt prompts the user to approve new client authorization grant requests
	GrantHandlerPrompt GrantHandlerType = "prompt"
	// GrantHandlerDeny auto-denies client authorization grant requests
	GrantHandlerDeny GrantHandlerType = "deny"
)

var ValidGrantHandlerTypes = util.NewStringSet(string(GrantHandlerAuto), string(GrantHandlerPrompt), string(GrantHandlerDeny))

type EtcdConfig struct {
	// ServingInfo describes how to start serving the etcd master
	ServingInfo ServingInfo
	// Address is the advertised host:port for client connections to etcd
	Address string
	// PeerServingInfo describes how to start serving the etcd peer
	PeerServingInfo ServingInfo
	// PeerAddress is the advertised host:port for peer connections to etcd
	PeerAddress string
	// StorageDir indicates where to save the etcd data
	StorageDir string
}

type KubernetesMasterConfig struct {
	// MasterIP is the public IP address of kubernetes stuff.  If empty, the first result from net.InterfaceAddrs will be used.
	MasterIP string
	// ServicesSubnet is the subnet to use for assigning service IPs
	ServicesSubnet string
	// StaticNodeNames is the list of nodes that are statically known
	StaticNodeNames []string
	// SchedulerConfigFile points to a file that describes how to set up the scheduler.  If empty, you get the default scheduling rules.
	SchedulerConfigFile string
}

type CertInfo struct {
	// CertFile is a file containing a PEM-encoded certificate
	CertFile string
	// KeyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	KeyFile string
}
