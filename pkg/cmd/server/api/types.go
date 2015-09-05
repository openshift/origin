package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

const (
	FeatureBuilder    = `Builder`
	FeatureS2I        = `S2I Builder`
	FeatureWebConsole = `Web Console`
)

var (
	KnownKubernetesAPILevels   = []string{"v1beta1", "v1beta2", "v1beta3", "v1"}
	KnownOpenShiftAPILevels    = []string{"v1beta1", "v1beta3", "v1"}
	DefaultKubernetesAPILevels = []string{"v1beta3", "v1"}
	DefaultOpenShiftAPILevels  = []string{"v1beta3", "v1"}
	DeadKubernetesAPILevels    = []string{"v1beta1", "v1beta2"}
	DeadOpenShiftAPILevels     = []string{"v1beta1"}

	KnownOpenShiftFeatures = []string{FeatureBuilder, FeatureS2I, FeatureWebConsole}
	AtomicDisabledFeatures = []string{FeatureBuilder, FeatureS2I, FeatureWebConsole}
)

type ExtendedArguments map[string][]string

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	api.TypeMeta

	// NodeName is the value used to identify this particular node in the cluster.  If possible, this should be your fully qualified hostname.
	// If you're describing a set of static nodes to the master, this value must match one of the values in the list
	NodeName string

	// Node may have multiple IPs, specify the IP to use for pod traffic routing
	// If not specified, network parse/lookup on the nodeName is performed and the first non-loopback address is used
	NodeIP string

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo

	// MasterKubeConfig is a filename for the .kubeconfig file that describes how to connect this node to the master
	MasterKubeConfig string

	// DNSDomain holds the domain suffix
	DNSDomain string

	// DNSIP holds the IP
	DNSIP string

	// NetworkConfig provides network options for the node
	NetworkConfig NodeNetworkConfig

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

	// KubeletArguments are key value pairs that will be passed directly to the Kubelet that match the Kubelet's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	KubeletArguments ExtendedArguments
}

// NodeNetworkConfig provides network options for the node
type NodeNetworkConfig struct {
	// NetworkPluginName is a string specifying the networking plugin
	NetworkPluginName string
	// Maximum transmission unit for the network packets
	MTU uint
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
	// DockerExecHandlerNsenter uses nsenter for executing commands in containers.
	DockerExecHandlerNsenter DockerExecHandlerType = "nsenter"

	// ControllersDisabled indicates no controllers should be enabled.
	ControllersDisabled = "none"
	// ControllersAll indicates all controllers should be started.
	ControllersAll = "*"
)

type FeatureList []string

type MasterConfig struct {
	api.TypeMeta

	// ServingInfo describes how to start serving
	ServingInfo HTTPServingInfo

	// CORSAllowedOrigins
	CORSAllowedOrigins []string

	// APILevels is a list of API levels that should be enabled on startup: v1beta3 and v1 as examples
	APILevels []string

	// MasterPublicURL is how clients can access the OpenShift API server
	MasterPublicURL string

	// Controllers is a list of the controllers that should be started. If set to "none", no controllers
	// will start automatically. The default value is "*" which will start all controllers. When
	// using "*", you may exclude controllers by prepending a "-" in front of their name. No other
	// values are recognized at this time.
	Controllers string
	// PauseControllers instructs the master to not automatically start controllers, but instead
	// to wait until a notification to the server is received before launching them.
	// TODO: will be disabled in function for 1.1.
	PauseControllers bool
	// ControllerLeaseTTL enables controller election, instructing the master to attempt to acquire
	// a lease before controllers start and renewing it within a number of seconds defined by this value.
	// Setting this value non-negative forces pauseControllers=true. This value defaults off (0, or
	// omitted) and controller election can be disabled with -1.
	ControllerLeaseTTL int
	// TODO: the next field added to controllers must be added to a new controllers struct

	// Allow to disable OpenShift components
	DisabledFeatures FeatureList

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

	// RoutingConfig holds information about routing and route generation
	RoutingConfig RoutingConfig

	// NetworkConfig to be passed to the compiled in network plugin
	NetworkConfig MasterNetworkConfig
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

	// SecurityAllocator controls the automatic allocation of UIDs and MCS labels to a project. If nil, allocation is disabled.
	SecurityAllocator *SecurityAllocator
}

type RoutingConfig struct {
	// Subdomain is the suffix appended to $service.$namespace. to form the default route hostname
	Subdomain string
}

type SecurityAllocator struct {
	// UIDAllocatorRange defines the total set of Unix user IDs (UIDs) that will be allocated to projects automatically, and the size of the
	// block each namespace gets. For example, 1000-1999/10 will allocate ten UIDs per namespace, and will be able to allocate up to 100 blocks
	// before running out of space. The default is to allocate from 1 billion to 2 billion in 10k blocks (which is the expected size of the
	// ranges Docker images will use once user namespaces are started).
	UIDAllocatorRange string
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
	MCSAllocatorRange string
	// MCSLabelsPerProject defines the number of labels that should be reserved per project. The default is 5 to match the default UID and MCS
	// ranges (100k namespaces, 535k/5 labels).
	MCSLabelsPerProject int
}

type PolicyConfig struct {
	// BootstrapPolicyFile points to a template that contains roles and rolebindings that will be created if no policy object exists in the master namespace
	BootstrapPolicyFile string

	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string

	// OpenShiftInfrastructureNamespace is the namespace where OpenShift infrastructure resources live (like controller service accounts)
	OpenShiftInfrastructureNamespace string
}

// MasterNetworkConfig to be passed to the compiled in network plugin
type MasterNetworkConfig struct {
	NetworkPluginName  string
	ClusterNetworkCIDR string
	HostSubnetLength   uint
	ServiceNetworkCIDR string
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
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string
	// ServerCert is the TLS cert info for serving secure traffic
	ServerCert CertInfo
	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string
}

type HTTPServingInfo struct {
	ServingInfo
	// MaxRequestsInFlight is the number of concurrent requests allowed to the server. If zero, no limit.
	MaxRequestsInFlight int
	// RequestTimeoutSeconds is the number of seconds before requests are timed out. The default is 60 minutes, if
	// -1 there is no limit on requests.
	RequestTimeoutSeconds int
}

type MasterClients struct {
	// OpenShiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string
	// ExternalKubernetesKubeConfig is a .kubeconfig filename for proxying to kubernetes
	ExternalKubernetesKubeConfig string
}

type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string
}

type AssetConfig struct {
	ServingInfo HTTPServingInfo

	// PublicURL is where you can find the asset server (TODO do we really need this?)
	PublicURL string

	// LogoutURL is an optional, absolute URL to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURL string

	// MasterPublicURL is how the web console can access the OpenShift api server
	MasterPublicURL string

	// ExtensionScripts are file paths on the asset server files to load as scripts when the Web
	// Console loads
	ExtensionScripts []string

	// ExtensionStylesheets are file paths on the asset server files to load as stylesheets when
	// the Web Console loads
	ExtensionStylesheets []string

	// Extensions are files to serve from the asset server filesystem under a subcontext
	Extensions []AssetExtensionsConfig

	// ExtensionDevelopment when true tells the asset server to reload extension scripts and
	// stylesheets for every request rather than only at startup. It lets you develop extensions
	// without having to restart the server for every change.
	ExtensionDevelopment bool
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

	// Templates allow you to customize pages like the login page.
	Templates *OAuthTemplates
}

type OAuthTemplates struct {
	// Login is a path to a file containing a go template used to render the login page.
	// If unspecified, the default login page is used.
	Login string
}

type ServiceAccountConfig struct {
	// ManagedNames is a list of service account names that will be auto-created in every namespace.
	// If no names are specified, the ServiceAccountsController will not be started.
	ManagedNames []string

	// LimitSecretReferences controls whether or not to allow a service account to reference any secret in a namespace
	// without explicitly referencing them
	LimitSecretReferences bool

	// PrivateKeyFile is a file containing a PEM-encoded private RSA key, used to sign service account tokens.
	// If no private key is specified, the service account TokensController will not be started.
	PrivateKeyFile string

	// PublicKeyFiles is a list of files, each containing a PEM-encoded public RSA key.
	// (If any file contains a private key, the public portion of the key is used)
	// The list of public keys is used to verify presented service account tokens.
	// Each key is tried in order until the list is exhausted or verification succeeds.
	// If no keys are specified, no service account authentication will be available.
	PublicKeyFiles []string

	// MasterCA is the CA for verifying the TLS connection back to the master.  The service account controller will automatically
	// inject the contents of this file into pods so they can verify connections to the master.
	MasterCA string
}

type TokenConfig struct {
	// AuthorizeTokenMaxAgeSeconds defines the maximum age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32
	// AccessTokenMaxAgeSeconds defines the maximum age of access tokens
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

	// Secrets is a list of secrets
	// New sessions are signed and encrypted using the first secret.
	// Existing sessions are decrypted/authenticated by each secret until one succeeds. This allows rotating secrets.
	Secrets []SessionSecret
}

type SessionSecret struct {
	// Authentication is used to authenticate sessions using HMAC. Recommended to use a secret with 32 or 64 bytes.
	Authentication string
	// Encryption is used to encrypt sessions. Must be 16, 24, or 32 characters long, to select AES-128, AES-192, or AES-256.
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

type LDAPPasswordIdentityProvider struct {
	api.TypeMeta
	// URL is an RFC 2255 URL which specifies the LDAP search parameters to use. The syntax of the URL is
	//    ldap://host:port/basedn?attribute?scope?filter
	URL string
	// BindDN is an optional DN to bind with during the search phase.
	BindDN string
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword string
	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool
	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string
	// LDAPEntryAttributeMapping maps LDAP attributes to identities
	LDAPEntryAttributeMapping LDAPAttributeMapping
}

type LDAPAttributeMapping struct {
	// ID is the list of attributes whose values should be used as the user ID. Required.
	// LDAP standard identity attribute is "dn"
	ID []string
	// PreferredUsername is the list of attributes whose values should be used as the preferred username.
	// LDAP standard login attribute is "uid"
	PreferredUsername []string
	// Name is the list of attributes whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	// LDAP standard display name attribute is "cn"
	Name []string
	// Email is the list of attributes whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string
}

type RequestHeaderIdentityProvider struct {
	api.TypeMeta

	// LoginURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect interactive logins will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	LoginURL string

	// ChallengeURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect WWW-Authenticate challenges will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	ChallengeURL string

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
	// APILevels is a list of API levels that should be enabled on startup: v1beta3 and v1 as examples
	APILevels []string
	// MasterIP is the public IP address of kubernetes stuff.  If empty, the first result from net.InterfaceAddrs will be used.
	MasterIP string
	// MasterCount is the number of expected masters that should be running. This value defaults to 1 and may be set to a positive integer,
	// or if set to -1, indicates this is part of a cluster.
	MasterCount int
	// ServicesSubnet is the subnet to use for assigning service IPs
	ServicesSubnet string
	// ServicesNodePortRange is the range to use for assigning service public ports on a host.
	ServicesNodePortRange string
	// StaticNodeNames is the list of nodes that are statically known
	StaticNodeNames []string
	// SchedulerConfigFile points to a file that describes how to set up the scheduler. If empty, you get the default scheduling rules.
	SchedulerConfigFile string
	// PodEvictionTimeout controls grace period for deleting pods on failed nodes.
	// It takes valid time duration string. If empty, you get the default pod eviction timeout.
	PodEvictionTimeout string
	// APIServerArguments are key value pairs that will be passed directly to the Kube apiserver that match the apiservers's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	APIServerArguments ExtendedArguments
	// ControllerArguments are key value pairs that will be passed directly to the Kube controller manager that match the
	// controller manager's command line arguments.  These are not migrated, but if you reference a value that does not exist
	// the server will not start. These values may override other settings in KubernetesMasterConfig which may cause invalid
	// configurations.
	ControllerArguments ExtendedArguments
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

type AssetExtensionsConfig struct {
	// Name is the path under /<context>/extensions/ to serve files from SourceDirectory
	Name string
	// SourceDirectory is a directory on the asset server to serve files under Name in the Web
	// Console. It may have nested folders.
	SourceDirectory string
	// HTML5Mode determines whether to redirect to the root index.html when a file is not found.
	// This is needed for apps that use the HTML5 history API like AngularJS apps with HTML5
	// mode enabled. If HTML5Mode is true, also rewrite the base element in index.html with the
	// Web Console's context root. Defaults to false.
	HTML5Mode bool
}
