package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

var (
	KnownKubernetesAPILevels   = []string{"v1beta1", "v1beta2", "v1beta3", "v1"}
	KnownOpenShiftAPILevels    = []string{"v1beta1", "v1beta3", "v1"}
	DefaultKubernetesAPILevels = []string{"v1beta1", "v1beta2", "v1beta3", "v1"}
	DefaultOpenShiftAPILevels  = []string{"v1beta3", "v1"}
	DeadKubernetesAPILevels    = []string{}
	DeadOpenShiftAPILevels     = []string{"v1beta1"}
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

	// NetworkPluginName is a string specifying the networking plugin
	NetworkPluginName string

	// VolumeDir is the directory that volumes will be stored under
	VolumeDirectory string

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig

	// AllowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool

	// PodManifestConfig holds the configuration for enabling the Kubelet to
	// create pods based from a manifest file(s) placed locally on the node
	PodManifestConfig *PodManifestConfig

	// DockerConfig holds Docker related configuration options.
	DockerConfig DockerConfig
}

// DockerConfig holds Docker related configuration options.
type DockerConfig struct {
	// ExecHandlerName is the name of the handler to use for executing
	// commands in Docker containers.
	ExecHandlerName DockerExecHandlerType
}

type DockerExecHandlerType string

const (
	// DockerExecHandlerNative uses Docker's exec API for executing commands in containers.
	DockerExecHandlerNative DockerExecHandlerType = "native"
	// DockerExecHandlerNative uses nsenter for executing commands in containers.
	DockerExecHandlerNsenter DockerExecHandlerType = "nsenter"
)

type MasterConfig struct {
	api.TypeMeta

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo

	// CORSAllowedOrigins
	CORSAllowedOrigins []string

	// APILevels is a list of API levels that should be enabled on startup: v1beta1, v1beta3, v1 as examples
	APILevels []string

	// MasterPublicURL is how clients can access the OpenShift API server
	MasterPublicURL string

	// EtcdStorageConfig contains information about how API resources are
	// stored in Etcd. These values are only relevant when etcd is the
	// backing store for the cluster.
	EtcdStorageConfig EtcdStorageConfig

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
	// AssetConfig, if present start the asset server in this process
	AssetConfig *AssetConfig
	// DNSConfig, if present start the DNS server in this process
	DNSConfig *DNSConfig

	// ServiceAccountConfig holds options related to service accounts
	ServiceAccountConfig ServiceAccountConfig

	// MasterClients holds all the client connection information for controllers and other system components
	MasterClients MasterClients

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig

	// PolicyConfig holds information about where to locate critical pieces of bootstrapping policy
	PolicyConfig PolicyConfig

	// ProjectConfig holds information about project creation and defaults
	ProjectConfig ProjectConfig

	// NetworkConfig to be passed to the compiled in network plugin
	NetworkConfig NetworkConfig
}

type ProjectConfig struct {
	// DefaultNodeSelector holds default project node label selector
	DefaultNodeSelector string

	// ProjectRequestMessage is the string presented to a user if they are unable to request a project via the projectrequest api endpoint
	ProjectRequestMessage string

	// ProjectRequestTemplate is the template to use for creating projects in response to projectrequest.
	// It is in the format namespace/template and it is optional.
	// If it is not specified, a default template is used.
	ProjectRequestTemplate string
}

type PolicyConfig struct {
	// BootstrapPolicyFile points to a template that contains roles and rolebindings that will be created if no policy object exists in the master namespace
	BootstrapPolicyFile string

	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string
}

// NetworkConfig to be passed to the compiled in network plugin
type NetworkConfig struct {
	NetworkPluginName  string `json:"networkPluginName"`
	ClusterNetworkCIDR string `json:"clusterNetworkCIDR"`
	HostSubnetLength   uint   `json:"hostSubnetLength"`
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

type EtcdStorageConfig struct {
	// KubernetesStorageVersion is the API version that Kube resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	KubernetesStorageVersion string
	// KubernetesStoragePrefix is the path within etcd that the Kubernetes resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located.
	KubernetesStoragePrefix string
	// OpenShiftStorageVersion is the API version that OS resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	OpenShiftStorageVersion string
	// OpenShiftStoragePrefix is the path within etcd that the OpenShift resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located.
	OpenShiftStoragePrefix string
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
	// ExternalKubernetesKubeConfig is a .kubeconfig filename for proxying to kubernetes
	ExternalKubernetesKubeConfig string
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

type ServiceAccountConfig struct {
	// ManagedNames is a list of service account names that will be auto-created in every namespace.
	// If no names are specified, the ServiceAccountsController will not be started.
	ManagedNames []string

	// PrivateKeyFile is a file containing a PEM-encoded private RSA key, used to sign service account tokens.
	// If no private key is specified, the service account TokensController will not be started.
	PrivateKeyFile string

	// PublicKeyFiles is a list of files, each containing a PEM-encoded public RSA key.
	// (If any file contains a private key, the public portion of the key is used)
	// The list of public keys is used to verify presented service account tokens.
	// Each key is tried in order until the list is exhausted or verification succeeds.
	// If no keys are specified, no service account authentication will be available.
	PublicKeyFiles []string
}

type TokenConfig struct {
	// Max age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32
	// Max age of access tokens
	AccessTokenMaxAgeSeconds int32
}

// SessionConfig specifies options for cookie-based sessions. Used by AuthRequestHandlerSession
type SessionConfig struct {
	// SessionSecretsFile is a reference to a file containing a serialized SessionSecrets object
	// If no file is specified, a random signing and encryption key are generated at each server start
	SessionSecretsFile string
	// SessionMaxAgeSeconds specifies how long created sessions last. Used by AuthRequestHandlerSession
	SessionMaxAgeSeconds int32
	// SessionName is the cookie name used to store the session
	SessionName string
}

// SessionSecrets list the secrets to use to sign/encrypt and authenticate/decrypt created sessions.
type SessionSecrets struct {
	api.TypeMeta

	// New sessions are signed and encrypted using the first secret.
	// Existing sessions are decrypted/authenticated by each secret until one succeeds. This allows rotating secrets.
	Secrets []SessionSecret
}

type SessionSecret struct {
	// Signing secret, used to authenticate sessions using HMAC. Recommended to use a secret with 32 or 64 bytes.
	Authentication string
	// Encrypting secret, used to encrypt sessions. Must be 16, 24, or 32 characters long, to select AES-128, AES-192, or AES-256.
	Encryption string
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

type GitHubIdentityProvider struct {
	api.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret string
}

type GoogleIdentityProvider struct {
	api.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret string

	// HostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string
}

type OpenIDIdentityProvider struct {
	api.TypeMeta

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret string

	// ExtraScopes are any scopes to request in addition to the standard "openid" scope.
	ExtraScopes []string

	// ExtraAuthorizeParameters are any custom parameters to add to the authorize request.
	ExtraAuthorizeParameters map[string]string

	// URLs to use to authenticate
	URLs OpenIDURLs

	// Claims mappings
	Claims OpenIDClaims
}

type OpenIDURLs struct {
	// Authorize is the oauth authorization URL
	Authorize string
	// Token is the oauth token granting URL
	Token string
	// UserInfo is the optional userinfo URL.
	// If present, a granted access_token is used to request claims
	// If empty, a granted id_token is parsed for claims
	UserInfo string
}

type OpenIDClaims struct {
	// ID is the list of claims whose values should be used as the user ID. Required.
	// OpenID standard identity claim is "sub"
	ID []string
	// PreferredUsername is the list of claims whose values should be used as the preferred username.
	// If unspecified, the preferred username is determined from the value of the id claim
	PreferredUsername []string
	// Name is the list of claims whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	Name []string
	// Email is the list of claims whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string
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
	// APILevels is a list of API levels that should be enabled on startup: v1beta1, v1beta2, v1beta3, v1 as examples
	APILevels []string
	// MasterIP is the public IP address of kubernetes stuff.  If empty, the first result from net.InterfaceAddrs will be used.
	MasterIP string
	// MasterCount is the number of expected masters that should be running. This value defaults to 1 and may be set to a positive integer.
	MasterCount int
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

type PodManifestConfig struct {
	// Path specifies the path for the pod manifest file or directory
	// If its a directory, its expected to contain on or more manifest files
	// This is used by the Kubelet to create pods on the node
	Path string
	// FileCheckIntervalSeconds is the interval in seconds for checking the manifest file(s) for new data
	// The interval needs to be a positive value
	FileCheckIntervalSeconds int64
}
