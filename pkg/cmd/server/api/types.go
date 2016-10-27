package api

import (
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
)

// A new entry shall be added to FeatureAliases for every change to following values.
const (
	FeatureBuilder    = `Builder`
	FeatureS2I        = `S2IBuilder`
	FeatureWebConsole = `WebConsole`

	AllVersions = "*"
)

var (
	KnownKubernetesAPILevels   = []string{"v1beta1", "v1beta2", "v1beta3", "v1"}
	KnownOpenShiftAPILevels    = []string{"v1beta1", "v1beta3", "v1"}
	DefaultKubernetesAPILevels = []string{"v1"}
	DefaultOpenShiftAPILevels  = []string{"v1"}
	DeadKubernetesAPILevels    = []string{"v1beta1", "v1beta2", "v1beta3"}
	DeadOpenShiftAPILevels     = []string{"v1beta1", "v1beta3"}
	// KnownKubernetesStorageVersionLevels are storage versions that can be
	// dealt with internally.
	KnownKubernetesStorageVersionLevels = []string{"v1", "v1beta3"}
	// KnownOpenShiftStorageVersionLevels are storage versions that can be dealt
	// with internally
	KnownOpenShiftStorageVersionLevels = []string{"v1", "v1beta3"}
	// DefaultOpenShiftStorageVersionLevel is the default storage version for
	// resources.
	DefaultOpenShiftStorageVersionLevel = "v1"
	// DeadKubernetesStorageVersionLevels are storage versions which shouldn't
	// be exposed externally.
	DeadKubernetesStorageVersionLevels = []string{"v1beta3"}
	// DeadOpenShiftStorageVersionLevels are storage versions which shouldn't be
	// exposed externally.
	DeadOpenShiftStorageVersionLevels = []string{"v1beta1", "v1beta3"}

	APIGroupKube           = ""
	APIGroupExtensions     = "extensions"
	APIGroupApps           = "apps"
	APIGroupAuthentication = "authentication.k8s.io"
	APIGroupAutoscaling    = "autoscaling"
	APIGroupBatch          = "batch"
	APIGroupCertificates   = "certificates.k8s.io"
	APIGroupFederation     = "federation"
	APIGroupPolicy         = "policy"
	APIGroupStorage        = "storage.k8s.io"

	// Map of group names to allowed REST API versions
	KubeAPIGroupsToAllowedVersions = map[string][]string{
		APIGroupKube:           {"v1"},
		APIGroupExtensions:     {"v1beta1"},
		APIGroupApps:           {"v1alpha1"},
		APIGroupAuthentication: {"v1beta1"},
		APIGroupAutoscaling:    {"v1"},
		APIGroupBatch:          {"v1", "v2alpha1"},
		APIGroupCertificates:   {"v1alpha1"},
		APIGroupPolicy:         {"v1alpha1"},
		APIGroupStorage:        {"v1beta1"},
		// TODO: enable as part of a separate binary
		//APIGroupFederation:  {"v1beta1"},
	}
	// Map of group names to known, but disallowed REST API versions
	KubeAPIGroupsToDeadVersions = map[string][]string{
		APIGroupKube:        {"v1beta3"},
		APIGroupExtensions:  {},
		APIGroupAutoscaling: {},
		APIGroupBatch:       {},
		APIGroupPolicy:      {},
		APIGroupApps:        {},
	}
	KnownKubeAPIGroups = sets.StringKeySet(KubeAPIGroupsToAllowedVersions)

	// FeatureAliases maps deprecated names of feature flag to their canonical
	// names. Aliases must be lower-cased for O(1) lookup.
	FeatureAliases = map[string]string{
		"s2i builder": FeatureS2I,
		"web console": FeatureWebConsole,
	}
	KnownOpenShiftFeatures = []string{FeatureBuilder, FeatureS2I, FeatureWebConsole}
	AtomicDisabledFeatures = []string{FeatureBuilder, FeatureS2I, FeatureWebConsole}
)

type ExtendedArguments map[string][]string

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	unversioned.TypeMeta

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

	// MasterClientConnectionOverrides provides overrides to the client connection used to connect to the master.
	MasterClientConnectionOverrides *ClientConnectionOverrides

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

	// AuthConfig holds authn/authz configuration options
	AuthConfig NodeAuthConfig

	// DockerConfig holds Docker related configuration options.
	DockerConfig DockerConfig

	// KubeletArguments are key value pairs that will be passed directly to the Kubelet that match the Kubelet's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	KubeletArguments ExtendedArguments

	// ProxyArguments are key value pairs that will be passed directly to the Proxy that match the Proxy's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	ProxyArguments ExtendedArguments

	// IPTablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string

	// EnableUnidling controls whether or not the hybrid unidling proxy will be set up
	EnableUnidling bool

	// VolumeConfig contains options for configuring volumes on the node.
	VolumeConfig NodeVolumeConfig
}

// NodeVolumeConfig contains options for configuring volumes on the node.
type NodeVolumeConfig struct {
	// LocalQuota contains options for controlling local volume quota on the node.
	LocalQuota LocalQuota
}

// MasterVolumeConfig contains options for configuring volume plugins in the master node.
type MasterVolumeConfig struct {
	// DynamicProvisioningEnabled is a boolean that toggles dynamic provisioning off when false, defaults to true
	DynamicProvisioningEnabled bool
}

// LocalQuota contains options for controlling local volume quota on the node.
type LocalQuota struct {
	// PerFSGroup can be specified to enable a quota on local storage use per unique FSGroup ID.
	// At present this is only implemented for emptyDir volumes, and if the underlying
	// volumeDirectory is on an XFS filesystem.
	PerFSGroup *resource.Quantity
}

// NodeNetworkConfig provides network options for the node
type NodeNetworkConfig struct {
	// NetworkPluginName is a string specifying the networking plugin
	NetworkPluginName string
	// Maximum transmission unit for the network packets
	MTU uint32
}

// NodeAuthConfig holds authn/authz configuration options
type NodeAuthConfig struct {
	// AuthenticationCacheTTL indicates how long an authentication result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthenticationCacheTTL string

	// AuthenticationCacheSize indicates how many authentication results should be cached. If 0, the default cache size is used.
	AuthenticationCacheSize int

	// AuthorizationCacheTTL indicates how long an authorization result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthorizationCacheTTL string

	// AuthorizationCacheSize indicates how many authorization results should be cached. If 0, the default cache size is used.
	AuthorizationCacheSize int
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
	unversioned.TypeMeta

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

	// AdmissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig

	ControllerConfig ControllerConfig

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

	// ImagePolicyConfig controls limits and behavior for importing images
	ImagePolicyConfig ImagePolicyConfig

	// PolicyConfig holds information about where to locate critical pieces of bootstrapping policy
	PolicyConfig PolicyConfig

	// ProjectConfig holds information about project creation and defaults
	ProjectConfig ProjectConfig

	// RoutingConfig holds information about routing and route generation
	RoutingConfig RoutingConfig

	// NetworkConfig to be passed to the compiled in network plugin
	NetworkConfig MasterNetworkConfig

	// VolumeConfig contains options for configuring volumes on the node.
	VolumeConfig MasterVolumeConfig

	// JenkinsPipelineConfig holds information about the default Jenkins template
	// used for JenkinsPipeline build strategy.
	JenkinsPipelineConfig JenkinsPipelineConfig

	// AuditConfig holds information related to auditing capabilities.
	AuditConfig AuditConfig
}

// AuditConfig holds configuration for the audit capabilities
type AuditConfig struct {
	// If this flag is set, audit log will be printed in the logs.
	// The logs contains, method, user and a requested URL.
	Enabled bool
	// All requests coming to the apiserver will be logged to this file.
	AuditFilePath string
	// Maximum number of days to retain old log files based on the timestamp encoded in their filename.
	MaximumFileRetentionDays int
	// Maximum number of old log files to retain.
	MaximumRetainedFiles int
	// Maximum size in megabytes of the log file before it gets rotated. Defaults to 100MB.
	MaximumFileSizeMegabytes int
}

// JenkinsPipelineConfig holds configuration for the Jenkins pipeline strategy
type JenkinsPipelineConfig struct {
	// AutoProvisionEnabled determines whether a Jenkins server will be spawned from the provided
	// template when the first build config in the project with type JenkinsPipeline
	// is created. When not specified this option defaults to true.
	AutoProvisionEnabled *bool
	// TemplateNamespace contains the namespace name where the Jenkins template is stored
	TemplateNamespace string
	// TemplateName is the name of the default Jenkins template
	TemplateName string
	// ServiceName is the name of the Jenkins service OpenShift uses to detect
	// whether a Jenkins pipeline handler has already been installed in a project.
	// This value *must* match a service name in the provided template.
	ServiceName string
	// Parameters specifies a set of optional parameters to the Jenkins template.
	Parameters map[string]string
}

type ImagePolicyConfig struct {
	// MaxImagesBulkImportedPerRepository controls the number of images that are imported when a user
	// does a bulk import of a Docker repository. This number is set low to prevent users from
	// importing large numbers of images accidentally. Set -1 for no limit.
	MaxImagesBulkImportedPerRepository int
	// DisableScheduledImport allows scheduled background import of images to be disabled.
	DisableScheduledImport bool
	// ScheduledImageImportMinimumIntervalSeconds is the minimum number of seconds that can elapse between when image streams
	// scheduled for background import are checked against the upstream repository. The default value is 15 minutes.
	ScheduledImageImportMinimumIntervalSeconds int
	// MaxScheduledImageImportsPerMinute is the maximum number of image streams that will be imported in the background per minute.
	// The default value is 60. Set to -1 for unlimited.
	MaxScheduledImageImportsPerMinute int
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

	// UserAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
	UserAgentMatchingConfig UserAgentMatchingConfig
}

// UserAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
type UserAgentMatchingConfig struct {
	// If this list is non-empty, then a User-Agent must match one of the UserAgentRegexes to be allowed
	RequiredClients []UserAgentMatchRule

	// If this list is non-empty, then a User-Agent must not match any of the UserAgentRegexes
	DeniedClients []UserAgentDenyRule

	// DefaultRejectionMessage is the message shown when rejecting a client.  If it is not a set, a generic message is given.
	DefaultRejectionMessage string
}

// UserAgentMatchRule describes how to match a given request based on User-Agent and HTTPVerb
type UserAgentMatchRule struct {
	// UserAgentRegex is a regex that is checked against the User-Agent.
	Regex string

	// HTTPVerbs specifies which HTTP verbs should be matched.  An empty list means "match all verbs".
	HTTPVerbs []string
}

// UserAgentDenyRule adds a rejection message that can be used to help a user figure out how to get an approved client
type UserAgentDenyRule struct {
	UserAgentMatchRule

	// RejectionMessage is the message shown when rejecting a client.  If it is not a set, the default message is used.
	RejectionMessage string
}

// MasterNetworkConfig to be passed to the compiled in network plugin
type MasterNetworkConfig struct {
	NetworkPluginName  string
	ClusterNetworkCIDR string
	HostSubnetLength   uint32
	ServiceNetworkCIDR string
	// ExternalIPNetworkCIDRs controls what values are acceptable for the service external IP field. If empty, no externalIP
	// may be set. It may contain a list of CIDRs which are checked for access. If a CIDR is prefixed with !, IPs in that
	// CIDR will be rejected. Rejections will be applied first, then the IP checked against one of the allowed CIDRs. You
	// should ensure this range does not overlap with your nodes, pods, or service CIDRs for security reasons.
	ExternalIPNetworkCIDRs []string
	// IngressIPNetworkCIDR controls the range to assign ingress ips from for services of type LoadBalancer on bare
	// metal. If empty, ingress ips will not be assigned. It may contain a single CIDR that will be allocated from.
	// For security reasons, you should ensure that this range does not overlap with the CIDRs reserved for external ips,
	// nodes, pods, or services.
	IngressIPNetworkCIDR string
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
	// NamedCertificates is a list of certificates to use to secure requests to specific hostnames
	NamedCertificates []NamedCertificate
}

// NamedCertificate specifies a certificate/key, and the names it should be served for
type NamedCertificate struct {
	// Names is a list of DNS names this certificate should be used to secure
	// A name can be a normal DNS name, or can contain leading wildcard segments.
	Names []string
	// CertInfo is the TLS cert info for serving secure traffic
	CertInfo
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

	// OpenShiftLoopbackClientConnectionOverrides specifies client overrides for system components to loop back to this master.
	OpenShiftLoopbackClientConnectionOverrides *ClientConnectionOverrides
	// ExternalKubernetesClientConnectionOverrides specifies client overrides for proxying to Kubernetes.
	ExternalKubernetesClientConnectionOverrides *ClientConnectionOverrides
}

type ClientConnectionOverrides struct {
	// AcceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string
	// ContentType is the content type used when sending data to the server from this client.
	ContentType string

	// QPS controls the number of queries per second allowed for this connection.
	QPS float32
	// Burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32
}

type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string
	// AllowRecursiveQueries allows the DNS server on the master to answer queries recursively. Note that open
	// resolvers can be used for DNS amplification attacks and the master DNS should not be made accessible
	// to public networks.
	AllowRecursiveQueries bool
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

	// LoggingPublicURL is the public endpoint for logging (optional)
	LoggingPublicURL string

	// MetricsPublicURL is the public endpoint for metrics (optional)
	MetricsPublicURL string

	// ExtensionScripts are file paths on the asset server files to load as scripts when the Web
	// Console loads
	ExtensionScripts []string

	// ExtensionProperties are key(string) and value(string) pairs that will be injected into the console under
	// the global variable OPENSHIFT_EXTENSION_PROPERTIES
	ExtensionProperties map[string]string

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
	// MasterCA is the CA for verifying the TLS connection back to the MasterURL.
	// "" to use system roots, set to use custom roots, never nil (guaranteed by conversion defaults)
	MasterCA *string

	// MasterURL is used for making server-to-server calls to exchange authorization codes for access tokens
	MasterURL string

	// MasterPublicURL is used for building valid client redirect URLs for internal and external access
	MasterPublicURL string

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string

	// AlwaysShowProviderSelection will force the provider selection page to render even when there is only a single provider
	AlwaysShowProviderSelection bool

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

	// ProviderSelection is a path to a file containing a go template used to render the provider selection page.
	// If unspecified, the default provider selection page is used.
	ProviderSelection string

	// Error is a path to a file containing a go template used to render error pages during the authentication or grant flow
	// If unspecified, the default error page is used.
	Error string
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
	unversioned.TypeMeta

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
	// MappingMethod determines how identities from this provider are mapped to users
	MappingMethod string
	// Provider contains the information about how to set up a specific identity provider
	Provider runtime.Object
}

type BasicAuthPasswordIdentityProvider struct {
	unversioned.TypeMeta

	// RemoteConnectionInfo contains information about how to connect to the external basic auth server
	RemoteConnectionInfo RemoteConnectionInfo
}

type AllowAllPasswordIdentityProvider struct {
	unversioned.TypeMeta
}

type DenyAllPasswordIdentityProvider struct {
	unversioned.TypeMeta
}

type HTPasswdPasswordIdentityProvider struct {
	unversioned.TypeMeta

	// File is a reference to your htpasswd file
	File string
}

type LDAPPasswordIdentityProvider struct {
	unversioned.TypeMeta
	// URL is an RFC 2255 URL which specifies the LDAP search parameters to use. The syntax of the URL is
	//    ldap://host:port/basedn?attribute?scope?filter
	URL string
	// BindDN is an optional DN to bind with during the search phase.
	BindDN string
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool
	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string
	// Attributes maps LDAP attributes to identities
	Attributes LDAPAttributeMapping
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

type KeystonePasswordIdentityProvider struct {
	unversioned.TypeMeta
	// RemoteConnectionInfo contains information about how to connect to the keystone server
	RemoteConnectionInfo RemoteConnectionInfo
	// Domain Name is required for keystone v3
	DomainName string
}

type RequestHeaderIdentityProvider struct {
	unversioned.TypeMeta

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
	// ClientCommonNames is an optional list of common names to require a match from. If empty, any client certificate validated against the clientCA bundle is considered authoritative.
	ClientCommonNames []string

	// Headers is the set of headers to check for identity information
	Headers []string
	// PreferredUsernameHeaders is the set of headers to check for the preferred username
	PreferredUsernameHeaders []string
	// NameHeaders is the set of headers to check for the display name
	NameHeaders []string
	// EmailHeaders is the set of headers to check for the email address
	EmailHeaders []string
}

type GitHubIdentityProvider struct {
	unversioned.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource
	// Organizations optionally restricts which organizations are allowed to log in
	Organizations []string
}

type GitLabIdentityProvider struct {
	unversioned.TypeMeta

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string
	// URL is the oauth server base URL
	URL string
	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource
}

type GoogleIdentityProvider struct {
	unversioned.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource

	// HostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string
}

type OpenIDIdentityProvider struct {
	unversioned.TypeMeta

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource

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

	// ServiceAccountMethod is used for determining client authorization for service account oauth client.
	// It must be either: deny, prompt
	ServiceAccountMethod GrantHandlerType
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

var ValidGrantHandlerTypes = sets.NewString(string(GrantHandlerAuto), string(GrantHandlerPrompt), string(GrantHandlerDeny))
var ValidServiceAccountGrantHandlerTypes = sets.NewString(string(GrantHandlerPrompt), string(GrantHandlerDeny))

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
	// DisabledAPIGroupVersions is a map of groups to the versions (or *) that should be disabled.
	DisabledAPIGroupVersions map[string][]string

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

	// ProxyClientInfo specifies the client cert/key to use when proxying to pods
	ProxyClientInfo CertInfo

	// AdmissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig

	// APIServerArguments are key value pairs that will be passed directly to the Kube apiserver that match the apiservers's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	APIServerArguments ExtendedArguments
	// ControllerArguments are key value pairs that will be passed directly to the Kube controller manager that match the
	// controller manager's command line arguments.  These are not migrated, but if you reference a value that does not exist
	// the server will not start. These values may override other settings in KubernetesMasterConfig which may cause invalid
	// configurations.
	ControllerArguments ExtendedArguments
	// SchedulerArguments are key value pairs that will be passed directly to the Kube scheduler that match the scheduler's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	SchedulerArguments ExtendedArguments
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

const (
	// StringSourceEncryptedBlockType is the PEM block type used to store an encrypted string
	StringSourceEncryptedBlockType = "ENCRYPTED STRING"
	// StringSourceKeyBlockType is the PEM block type used to store an encrypting key
	StringSourceKeyBlockType = "ENCRYPTING KEY"
)

// StringSource allows specifying a string inline, or externally via env var or file.
// When it contains only a string value, it marshals to a simple JSON string.
type StringSource struct {
	// StringSourceSpec specifies the string value, or external location
	StringSourceSpec
}

// StringSourceSpec specifies a string value, or external location
type StringSourceSpec struct {
	// Value specifies the cleartext value, or an encrypted value if keyFile is specified.
	Value string

	// Env specifies an envvar containing the cleartext value, or an encrypted value if the keyFile is specified.
	Env string

	// File references a file containing the cleartext value, or an encrypted value if a keyFile is specified.
	File string

	// KeyFile references a file containing the key to use to decrypt the value.
	KeyFile string
}

type LDAPSyncConfig struct {
	unversioned.TypeMeta

	// URL is the scheme, host and port of the LDAP server to connect to: scheme://host:port
	URL string
	// BindDN is an optional DN to bind with during the search phase.
	BindDN string
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool
	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string

	// LDAPGroupUIDToOpenShiftGroupNameMapping is an optional direct mapping of LDAP group UIDs to
	// OpenShift Group names
	LDAPGroupUIDToOpenShiftGroupNameMapping map[string]string

	// RFC2307Config holds the configuration for extracting data from an LDAP server set up in a fashion
	// similar to RFC2307: first-class group and user entries, with group membership determined by a
	// multi-valued attribute on the group entry listing its members
	RFC2307Config *RFC2307Config

	// ActiveDirectoryConfig holds the configuration for extracting data from an LDAP server set up in a
	// fashion similar to that used in Active Directory: first-class user entries, with group membership
	// determined by a multi-valued attribute on members listing groups they are a member of
	ActiveDirectoryConfig *ActiveDirectoryConfig

	// AugmentedActiveDirectoryConfig holds the configuration for extracting data from an LDAP server
	// set up in a fashion similar to that used in Active Directory as described above, with one addition:
	// first-class group entries exist and are used to hold metadata but not group membership
	AugmentedActiveDirectoryConfig *AugmentedActiveDirectoryConfig
}

type RFC2307Config struct {
	// AllGroupsQuery holds the template for an LDAP query that returns group entries.
	AllGroupsQuery LDAPQuery

	// GroupUIDAttributes defines which attribute on an LDAP group entry will be interpreted as its unique identifier.
	// (ldapGroupUID)
	GroupUIDAttribute string

	// GroupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
	// an OpenShift group
	GroupNameAttributes []string

	// GroupMembershipAttributes defines which attributes on an LDAP group entry will be interpreted  as its members.
	// The values contained in those attributes must be queryable by your UserUIDAttribute
	GroupMembershipAttributes []string

	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery

	// UserUIDAttribute defines which attribute on an LDAP user entry will be interpreted as its unique identifier.
	// It must correspond to values that will be found from the GroupMembershipAttributes
	UserUIDAttribute string

	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	// This should match your PreferredUsername setting for your LDAPPasswordIdentityProvider
	UserNameAttributes []string

	// TolerateMemberNotFoundErrors determines the behavior of the LDAP sync job when missing user entries are
	// encountered. If 'true', an LDAP query for users that doesn't find any will be tolerated and an only
	// and error will be logged. If 'false', the LDAP sync job will fail if a query for users doesn't find
	// any. The default value is 'false'. Misconfigured LDAP sync jobs with this flag set to 'true' can cause
	// group membership to be removed, so it is recommended to use this flag with caution.
	TolerateMemberNotFoundErrors bool

	// TolerateMemberOutOfScopeErrors determines the behavior of the LDAP sync job when out-of-scope user entries
	// are encountered. If 'true', an LDAP query for a user that falls outside of the base DN given for the all
	// user query will be tolerated and only an error will be logged. If 'false', the LDAP sync job will fail
	// if a user query would search outside of the base DN specified by the all user query. Misconfigured LDAP
	// sync jobs with this flag set to 'true' can result in groups missing users, so it is recommended to use
	// this flag with caution.
	TolerateMemberOutOfScopeErrors bool
}

type ActiveDirectoryConfig struct {
	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery

	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string

	// GroupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string
}

type AugmentedActiveDirectoryConfig struct {
	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery

	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string

	// GroupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string

	// AllGroupsQuery holds the template for an LDAP query that returns group entries.
	AllGroupsQuery LDAPQuery

	// GroupUIDAttributes defines which attribute on an LDAP group entry will be interpreted as its unique identifier.
	// (ldapGroupUID)
	GroupUIDAttribute string

	// GroupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
	// an OpenShift group
	GroupNameAttributes []string
}

type LDAPQuery struct {
	// The DN of the branch of the directory where all searches should start from
	BaseDN string

	// The (optional) scope of the search. Can be:
	// base: only the base object,
	// one:  all object on the base level,
	// sub:  the entire subtree
	// Defaults to the entire subtree if not set
	Scope string

	// The (optional) behavior of the search with regards to alisases. Can be:
	// never:  never dereference aliases,
	// search: only dereference in searching,
	// base:   only dereference in finding the base object,
	// always: always dereference
	// Defaults to always dereferencing if not set
	DerefAliases string

	// TimeLimit holds the limit of time in seconds that any request to the server can remain outstanding
	// before the wait for a response is given up. If this is 0, no client-side limit is imposed
	TimeLimit int

	// Filter is a valid LDAP search filter that retrieves all relevant entries from the LDAP server with the base DN
	Filter string

	// PageSize is the maximum preferred page size, measured in LDAP entries. A page size of 0 means no paging will be done.
	PageSize int
}

type AdmissionPluginConfig struct {
	// Location is the path to a configuration file that contains the plugin's
	// configuration
	Location string

	// Configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.Object
}

type AdmissionConfig struct {
	// PluginConfig allows specifying a configuration file per admission control plugin
	PluginConfig map[string]AdmissionPluginConfig

	// PluginOrderOverride is a list of admission control plugin names that will be installed
	// on the master. Order is significant. If empty, a default list of plugins is used.
	PluginOrderOverride []string
}

// ControllerConfig holds configuration values for controllers
type ControllerConfig struct {
	// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
	// pods fulfilling a service to serve with.
	ServiceServingCert ServiceServingCert
}

// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
// pods fulfilling a service to serve with.
type ServiceServingCert struct {
	// Signer holds the signing information used to automatically sign serving certificates.
	// If this value is nil, then certs are not signed automatically.
	Signer *CertInfo
}

// DefaultAdmissionConfig can be used to enable or disable various admission plugins.
// When this type is present as the `configuration` object under `pluginConfig` and *if* the admission plugin supports it,
// this will cause an "off by default" admission plugin to be enabled
type DefaultAdmissionConfig struct {
	unversioned.TypeMeta

	// Disable turns off an admission plugin that is enabled by default.
	Disable bool
}
