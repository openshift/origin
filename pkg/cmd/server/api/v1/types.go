package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

type ExtendedArguments map[string][]string

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	v1.TypeMeta `json:",inline"`

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

	// NetworkPluginName is a string specifying the networking plugin
	NetworkPluginName string `json:"networkPluginName"`

	// VolumeDir is the directory that volumes will be stored under
	VolumeDirectory string `json:"volumeDirectory"`

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig `json:"imageConfig"`

	// AllowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool `json:"allowDisabledDocker"`

	// PodManifestConfig holds the configuration for enabling the Kubelet to
	// create pods based from a manifest file(s) placed locally on the node
	PodManifestConfig *PodManifestConfig `json:"podManifestConfig"`

	// DockerConfig holds Docker related configuration options.
	DockerConfig DockerConfig `json:"dockerConfig"`

	// KubeletArguments are key value pairs that will be passed directly to the Kubelet that match the Kubelet's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	KubeletArguments ExtendedArguments `json:"kubeletArguments,omitempty"`
}

// DockerConfig holds Docker related configuration options.
type DockerConfig struct {
	// ExecHandlerName is the name of the handler to use for executing
	// commands in Docker containers.
	ExecHandlerName DockerExecHandlerType `json:"execHandlerName"`
}

type DockerExecHandlerType string

const (
	// DockerExecHandlerNative uses Docker's exec API for executing commands in containers.
	DockerExecHandlerNative DockerExecHandlerType = "native"
	// DockerExecHandlerNative uses nsenter for executing commands in containers.
	DockerExecHandlerNsenter DockerExecHandlerType = "nsenter"
)

type MasterConfig struct {
	v1.TypeMeta `json:",inline"`

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo `json:"servingInfo"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins"`

	// APILevels is a list of API levels that should be enabled on startup: v1beta3 and v1 as examples
	APILevels []string `json:"apiLevels"`

	// MasterPublicURL is how clients can access the OpenShift API server
	MasterPublicURL string `json:"masterPublicURL"`

	// EtcdStorageConfig contains information about how API resources are
	// stored in Etcd. These values are only relevant when etcd is the
	// backing store for the cluster.
	EtcdStorageConfig EtcdStorageConfig `json:"etcdStorageConfig"`

	// EtcdClientInfo contains information about how to connect to etcd
	EtcdClientInfo EtcdConnectionInfo `json:"etcdClientInfo"`
	// KubeletClientInfo contains information about how to connect to kubelets
	KubeletClientInfo KubeletConnectionInfo `json:"kubeletClientInfo"`

	// KubernetesMasterConfig, if present start the kubernetes master in this process
	KubernetesMasterConfig *KubernetesMasterConfig `json:"kubernetesMasterConfig"`
	// EtcdConfig, if present start etcd in this process
	EtcdConfig *EtcdConfig `json:"etcdConfig"`
	// OAuthConfig, if present start the /oauth endpoint in this process
	OAuthConfig *OAuthConfig `json:"oauthConfig"`
	// AssetConfig, if present start the asset server in this process
	AssetConfig *AssetConfig `json:"assetConfig"`
	// DNSConfig, if present start the DNS server in this process
	DNSConfig *DNSConfig `json:"dnsConfig"`

	// ServiceAccountConfig holds options related to service accounts
	ServiceAccountConfig ServiceAccountConfig `json:"serviceAccountConfig"`

	// MasterClients holds all the client connection information for controllers and other system components
	MasterClients MasterClients `json:"masterClients"`

	ImageConfig ImageConfig `json:"imageConfig"`

	PolicyConfig PolicyConfig `json:"policyConfig"`

	// ProjectConfig holds information about project creation and defaults
	ProjectConfig ProjectConfig `json:"projectConfig"`

	// NetworkConfig to be passed to the compiled in network plugin
	NetworkConfig NetworkConfig `json:"networkConfig"`
}

type ProjectConfig struct {
	// DefaultNodeSelector holds default project node label selector
	DefaultNodeSelector string `json:"defaultNodeSelector"`

	// ProjectRequestMessage is the string presented to a user if they are unable to request a project via the projectrequest api endpoint
	ProjectRequestMessage string `json:"projectRequestMessage"`

	// ProjectRequestTemplate is the template to use for creating projects in response to projectrequest.
	// It is in the format namespace/template and it is optional.
	// If it is not specified, a default template is used.
	ProjectRequestTemplate string `json:"projectRequestTemplate"`

	// SecurityAllocator controls the automatic allocation of UIDs and MCS labels to a project. If nil, allocation is disabled.
	SecurityAllocator *SecurityAllocator `json:"securityAllocator"`
}

type SecurityAllocator struct {
	// UIDAllocatorRange defines the total set of Unix user IDs (UIDs) that will be allocated to projects automatically, and the size of the
	// block each namespace gets. For example, 1000-1999/10 will allocate ten UIDs per namespace, and will be able to allocate up to 100 blocks
	// before running out of space. The default is to allocate from 1 billion to 2 billion in 10k blocks (which is the expected size of the
	// ranges Docker images will use once user namespaces are started).
	UIDAllocatorRange string `json:"uidAllocatorRange"`
	// MCSAllocatorRange defines the range of MCS categories that will be assigned to namespaces. The format is
	// "<prefix>/<numberOfLabels>[,<maxCategory>]". The default is "s0/2" and will allocate from c0 -> c1023, which means a total of 535k labels
	// are available (1024 choose 2 ~ 535k). If this value is changed after startup, new projects may receive labels that are already allocated
	// to other projects. Prefix may be any valid SELinux set of terms (including user, role, and type), although leaving them as the default
	// will allow the server to set them automatically.
	//
	// Examples:
	// * s0:/2     - Allocate labels from s0:c0,c0 to s0:c511,c511
	// * s0:/2,512 - Allocate labels from s0:c0,c0,c0 to s0:c511,c511,511
	//
	MCSAllocatorRange string `json:"mcsAllocatorRange"`
	// MCSLabelsPerProject defines the number of labels that should be reserved per project. The default is 5 to match the default UID and MCS
	// ranges (100k namespaces, 535k/5 labels).
	MCSLabelsPerProject int `json:"mcsLabelsPerProject"`
}

type PolicyConfig struct {
	// BootstrapPolicyFile points to a template that contains roles and rolebindings that will be created if no policy object exists in the master namespace
	BootstrapPolicyFile string `json:"bootstrapPolicyFile"`

	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string `json:"openshiftSharedResourcesNamespace"`
}

// NetworkConfig to be passed to the compiled in network plugin
type NetworkConfig struct {
	NetworkPluginName  string `json:"networkPluginName"`
	ClusterNetworkCIDR string `json:"clusterNetworkCIDR"`
	HostSubnetLength   uint   `json:"hostSubnetLength"`
}

type ImageConfig struct {
	Format string `json:"format"`
	Latest bool   `json:"latest"`
}

type RemoteConnectionInfo struct {
	// URL is the remote URL to connect to
	URL string `json:"url"`
	// CA is the CA for verifying TLS connections
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information to present
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

type KubeletConnectionInfo struct {
	// Port is the port to connect to kubelets on
	Port uint `json:"port"`
	// CA is the CA for verifying TLS connections to kubelets
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to kubelets
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

type EtcdConnectionInfo struct {
	// URLs are the URLs for etcd
	URLs []string `json:"urls"`
	// CA is a file containing trusted roots for the etcd server certificates
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to etcd
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

type EtcdStorageConfig struct {
	// KubernetesStorageVersion is the API version that Kube resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	KubernetesStorageVersion string `json:"kubernetesStorageVersion"`
	// KubernetesStoragePrefix is the path within etcd that the Kubernetes resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located. The default value is 'kubernetes.io'.
	KubernetesStoragePrefix string `json:"kubernetesStoragePrefix"`
	// OpenShiftStorageVersion is the API version that OS resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	OpenShiftStorageVersion string `json:"openShiftStorageVersion"`
	// OpenShiftStoragePrefix is the path within etcd that the OpenShift resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located. The default value is 'openshift.io'.
	OpenShiftStoragePrefix string `json:"openShiftStoragePrefix"`
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
	// OpenShiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string `json:"openshiftLoopbackKubeConfig"`
	// ExternalKubernetesKubeConfig is a .kubeconfig filename for proxying to kubernetes
	ExternalKubernetesKubeConfig string `json:"externalKubernetesKubeConfig"`
}

type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string `json:"bindAddress"`
}

type AssetConfig struct {
	ServingInfo ServingInfo `json:"servingInfo"`

	// PublicURL is where you can find the asset server (TODO do we really need this?)
	PublicURL string `json:"publicURL"`

	// LogoutURL is an optional, absolute URL to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURL string `json:"logoutURL"`

	// MasterPublicURL is how the web console can access the OpenShift v1 server
	MasterPublicURL string `json:"masterPublicURL"`
}

type OAuthConfig struct {
	// MasterURL is used for building valid client redirect URLs for external access
	MasterURL string `json:"masterURL"`

	// MasterPublicURL is used for building valid client redirect URLs for external access
	MasterPublicURL string `json:"masterPublicURL"`

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string `json:"assetPublicURL"`

	IdentityProviders []IdentityProvider `json:"identityProviders"`

	GrantConfig GrantConfig `json:"grantConfig"`

	SessionConfig *SessionConfig `json:"sessionConfig"`

	TokenConfig TokenConfig `json:"tokenConfig"`
}

type ServiceAccountConfig struct {
	// ManagedNames is a list of service account names that will be auto-created in every namespace.
	// If no names are specified, the ServiceAccountsController will not be started.
	ManagedNames []string `json:"managedNames"`

	// PrivateKeyFile is a file containing a PEM-encoded private RSA key, used to sign service account tokens.
	// If no private key is specified, the service account TokensController will not be started.
	PrivateKeyFile string `json:"privateKeyFile"`

	// PublicKeyFiles is a list of files, each containing a PEM-encoded public RSA key.
	// (If any file contains a private key, the public portion of the key is used)
	// The list of public keys is used to verify presented service account tokens.
	// Each key is tried in order until the list is exhausted or verification succeeds.
	// If no keys are specified, no service account authentication will be available.
	PublicKeyFiles []string `json:"publicKeyFiles"`
}

type TokenConfig struct {
	// Max age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32 `json:"authorizeTokenMaxAgeSeconds"`
	// Max age of access tokens
	AccessTokenMaxAgeSeconds int32 `json:"accessTokenMaxAgeSeconds"`
}

// SessionConfig specifies options for cookie-based sessions. Used by AuthRequestHandlerSession
type SessionConfig struct {
	// SessionSecretsFile is a reference to a file containing a serialized SessionSecrets object
	// If no file is specified, a random signing and encryption key are generated at each server start
	SessionSecretsFile string `json:"sessionSecretsFile"`
	// SessionMaxAgeSeconds specifies how long created sessions last. Used by AuthRequestHandlerSession
	SessionMaxAgeSeconds int32 `json:"sessionMaxAgeSeconds"`
	// SessionName is the cookie name used to store the session
	SessionName string `json:"sessionName"`
}

// SessionSecrets list the secrets to use to sign/encrypt and authenticate/decrypt created sessions.
type SessionSecrets struct {
	v1.TypeMeta `json:",inline"`

	// New sessions are signed and encrypted using the first secret.
	// Existing sessions are decrypted/authenticated by each secret until one succeeds. This allows rotating secrets.
	Secrets []SessionSecret `json:"secrets"`
}

type SessionSecret struct {
	// Signing secret, used to authenticate sessions using HMAC. Recommended to use a secret with 32 or 64 bytes.
	Authentication string `json:"authentication"`
	// Encrypting secret, used to encrypt sessions. Must be 16, 24, or 32 characters long, to select AES-128, AES-
	Encryption string `json:"encryption"`
}

type IdentityProvider struct {
	// Name is used to qualify the identities returned by this provider
	Name string `json:"name"`
	// UseAsChallenger indicates whether to issue WWW-Authenticate challenges for this provider
	UseAsChallenger bool `json:"challenge"`
	// UseAsLogin indicates whether to use this identity provider for unauthenticated browsers to login against
	UseAsLogin bool `json:"login"`
	// Provider contains the information about how to set up a specific identity provider
	Provider runtime.RawExtension `json:"provider"`
}

type BasicAuthPasswordIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	RemoteConnectionInfo `json:",inline"`
}

type AllowAllPasswordIdentityProvider struct {
	v1.TypeMeta `json:",inline"`
}

type DenyAllPasswordIdentityProvider struct {
	v1.TypeMeta `json:",inline"`
}

type HTPasswdPasswordIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	File string `json:"file"`
}

type RequestHeaderIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	ClientCA string   `json:"clientCA"`
	Headers  []string `json:"headers"`
}

type GitHubIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret string `json:"clientSecret"`
}

type GoogleIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret string `json:"clientSecret"`

	// HostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string `json:"hostedDomain"`
}

type OpenIDIdentityProvider struct {
	v1.TypeMeta `json:",inline"`

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret string `json:"clientSecret"`

	// ExtraScopes are any scopes to request in addition to the standard "openid" scope.
	ExtraScopes []string `json:"extraScopes"`

	// ExtraAuthorizeParameters are any custom parameters to add to the authorize request.
	ExtraAuthorizeParameters map[string]string `json:"extraAuthorizeParameters"`

	// URLs to use to authenticate
	URLs OpenIDURLs `json:"urls"`

	// Claims mappings
	Claims OpenIDClaims `json:"claims"`
}

type OpenIDURLs struct {
	// Authorize is the oauth authorization URL
	Authorize string `json:"authorize"`
	// Token is the oauth token granting URL
	Token string `json:"token"`
	// UserInfo is the optional userinfo URL.
	// If present, a granted access_token is used to request claims
	// If empty, a granted id_token is parsed for claims
	UserInfo string `json:"userInfo"`
}

type OpenIDClaims struct {
	// ID is the list of claims whose values should be used as the user ID. Required.
	// OpenID standard identity claim is "sub"
	ID []string `json:"id"`
	// PreferredUsername is the list of claims whose values should be used as the preferred username.
	// If unspecified, the preferred username is determined from the value of the id claim
	PreferredUsername []string `json:"preferredUsername"`
	// Name is the list of claims whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	Name []string `json:"name"`
	// Email is the list of claims whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string `json:"email"`
}

type GrantConfig struct {
	// Method: allow, deny, prompt
	Method GrantHandlerType `json:"method"`
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

type EtcdConfig struct {
	// ServingInfo describes how to start serving the etcd master
	ServingInfo ServingInfo `json:"servingInfo"`
	// Address is the advertised host:port for client connections to etcd
	Address string `json:"address"`
	// PeerServingInfo describes how to start serving the etcd peer
	PeerServingInfo ServingInfo `json:"peerServingInfo"`
	// PeerAddress is the advertised host:port for peer connections to etcd
	PeerAddress string `json:"peerAddress"`

	StorageDir string `json:"storageDirectory"`
}

type KubernetesMasterConfig struct {
	// APILevels is a list of API levels that should be enabled on startup: v1beta3 and v1 as examples
	APILevels []string `json:"apiLevels"`
	MasterIP  string   `json:"masterIP"`
	// MasterCount is the number of expected masters that should be running. This value defaults to 1 and may be set to a positive integer.
	MasterCount         int      `json:"masterCount"`
	ServicesSubnet      string   `json:"servicesSubnet"`
	StaticNodeNames     []string `json:"staticNodeNames"`
	SchedulerConfigFile string   `json:"schedulerConfigFile"`
	PodEvictionTimeout  string   `json:"podEvictionTimeout"`
}

type CertInfo struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type PodManifestConfig struct {
	// Path specifies the path for the pod manifest file or directory
	// If its a directory, its expected to contain on or more manifest files
	// This is used by the Kubelet to create pods on the node
	Path string `json:"path"`
	// FileCheckIntervalSeconds is the interval in seconds for checking the manifest file(s) for new data
	// The interval needs to be a positive value
	FileCheckIntervalSeconds int64 `json:"fileCheckIntervalSeconds"`
}
