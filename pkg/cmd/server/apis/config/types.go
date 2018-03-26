package config

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

// A new entry shall be added to FeatureAliases for every change to following values.
const (
	AllVersions = "*"
)

const (
	DefaultIngressIPNetworkCIDR = "172.29.0.0/16"
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

	APIGroupKube                  = ""
	APIGroupApps                  = "apps"
	APIGroupAdmissionRegistration = "admissionregistration.k8s.io"
	APIGroupAPIExtensions         = "apiextensions.k8s.io"
	APIGroupAPIRegistration       = "apiregistration.k8s.io"
	APIGroupAuthentication        = "authentication.k8s.io"
	APIGroupAuthorization         = "authorization.k8s.io"
	APIGroupExtensions            = "extensions"
	APIGroupEvents                = "events.k8s.io"
	APIGroupImagePolicy           = "imagepolicy.k8s.io"
	APIGroupAutoscaling           = "autoscaling"
	APIGroupBatch                 = "batch"
	APIGroupCertificates          = "certificates.k8s.io"
	APIGroupNetworking            = "networking.k8s.io"
	APIGroupPolicy                = "policy"
	APIGroupStorage               = "storage.k8s.io"
	APIGroupComponentConfig       = "componentconfig"
	APIGroupAuthorizationRbac     = "rbac.authorization.k8s.io"
	APIGroupSettings              = "settings.k8s.io"
	APIGroupScheduling            = "scheduling.k8s.io"

	OriginAPIGroupCore                = ""
	OriginAPIGroupAuthorization       = "authorization.openshift.io"
	OriginAPIGroupBuild               = "build.openshift.io"
	OriginAPIGroupDeploy              = "apps.openshift.io"
	OriginAPIGroupTemplate            = "template.openshift.io"
	OriginAPIGroupImage               = "image.openshift.io"
	OriginAPIGroupProject             = "project.openshift.io"
	OriginAPIGroupProjectRequestLimit = "requestlimit.project.openshift.io"
	OriginAPIGroupUser                = "user.openshift.io"
	OriginAPIGroupOAuth               = "oauth.openshift.io"
	OriginAPIGroupRoute               = "route.openshift.io"
	OriginAPIGroupNetwork             = "network.openshift.io"
	OriginAPIGroupQuota               = "quota.openshift.io"
	OriginAPIGroupSecurity            = "security.openshift.io"

	// Map of group names to allowed REST API versions
	KubeAPIGroupsToAllowedVersions = map[string][]string{
		APIGroupKube:                  {"v1"},
		APIGroupExtensions:            {"v1beta1"},
		APIGroupEvents:                {"v1beta1"},
		APIGroupApps:                  {"v1", "v1beta1", "v1beta2"},
		APIGroupAdmissionRegistration: {"v1beta1"},
		APIGroupAPIExtensions:         {"v1beta1"},
		APIGroupAPIRegistration:       {"v1beta1"},
		APIGroupAuthentication:        {"v1", "v1beta1"},
		APIGroupAuthorization:         {"v1", "v1beta1"},
		APIGroupAuthorizationRbac:     {"v1", "v1beta1"},
		APIGroupAutoscaling:           {"v1", "v2beta1"},
		APIGroupBatch:                 {"v1", "v1beta1", "v2alpha1"}, // v2alpha1 has to stay on to keep cronjobs on for backwards compatibility
		APIGroupCertificates:          {"v1beta1"},
		APIGroupNetworking:            {"v1"},
		APIGroupPolicy:                {"v1beta1"},
		APIGroupStorage:               {"v1", "v1beta1"},
		APIGroupSettings:              {}, // list the group, but don't enable any versions.  alpha disabled by default, but enablable via arg
		APIGroupScheduling:            {}, // alpha disabled by default
		// TODO: enable as part of a separate binary
		//APIGroupFederation:  {"v1beta1"},
	}

	OriginAPIGroupsToAllowedVersions = map[string][]string{
		OriginAPIGroupAuthorization: {"v1"},
		OriginAPIGroupBuild:         {"v1"},
		OriginAPIGroupDeploy:        {"v1"},
		OriginAPIGroupTemplate:      {"v1"},
		OriginAPIGroupImage:         {"v1"},
		OriginAPIGroupProject:       {"v1"},
		OriginAPIGroupUser:          {"v1"},
		OriginAPIGroupOAuth:         {"v1"},
		OriginAPIGroupNetwork:       {"v1"},
		OriginAPIGroupRoute:         {"v1"},
		OriginAPIGroupQuota:         {"v1"},
		OriginAPIGroupSecurity:      {"v1"},
	}

	// Map of group names to known, but disabled REST API versions
	KubeDefaultDisabledVersions = map[string][]string{
		APIGroupKube:                  {"v1beta3"},
		APIGroupExtensions:            {},
		APIGroupAutoscaling:           {"v2alpha1"},
		APIGroupBatch:                 {},
		APIGroupPolicy:                {},
		APIGroupApps:                  {},
		APIGroupAdmissionRegistration: {"v1alpha1"},
		APIGroupAuthorizationRbac:     {"v1alpha1"},
		APIGroupSettings:              {"v1alpha1"},
		APIGroupScheduling:            {"v1alpha1"},
		APIGroupStorage:               {"v1alpha1"},
	}
	KnownKubeAPIGroups   = sets.StringKeySet(KubeAPIGroupsToAllowedVersions)
	KnownOriginAPIGroups = sets.StringKeySet(OriginAPIGroupsToAllowedVersions)

	// List public registries that we are allowing to import images from by default.
	// By default all registries have set to be "secure", iow. the port for them is
	// defaulted to "443".
	// If the registry you are adding here is insecure, you can add 'Insecure: true' to
	// make it default to port '80'.
	// If the registry you are adding use custom port, you have to specify the port as
	// part of the domain name.
	DefaultAllowedRegistriesForImport = &AllowedRegistries{
		{DomainName: "docker.io"},
		{DomainName: "*.docker.io"},  // registry-1.docker.io
		{DomainName: "*.redhat.com"}, // registry.connect.redhat.com and registry.access.redhat.com
		{DomainName: "gcr.io"},
		{DomainName: "quay.io"},
		{DomainName: "registry.centos.org"},
		{DomainName: "registry.redhat.io"},
		// FIXME: Probably need to have more fine-tuned pattern defined
		{DomainName: "*.amazonaws.com"},
	}
)

type ExtendedArguments map[string][]string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	metav1.TypeMeta

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

	// DNSDomain holds the domain suffix that will be used for the DNS search path inside each container. Defaults to
	// 'cluster.local'.
	DNSDomain string

	// DNSIP is the IP address that pods will use to access cluster DNS. Defaults to the service IP of the Kubernetes
	// master. This IP must be listening on port 53 for compatibility with libc resolvers (which cannot be configured
	// to resolve names from any other port). When running more complex local DNS configurations, this is often set
	// to the local address of a DNS proxy like dnsmasq, which then will consult either the local DNS (see
	// dnsBindAddress) or the master DNS.
	DNSIP string

	// DNSBindAddress is the ip:port to serve DNS on. If this is not set, the DNS server will not be started.
	// Because most DNS resolvers will only listen on port 53, if you select an alternative port you will need
	// a DNS proxy like dnsmasq to answer queries for containers. A common configuration is dnsmasq configured
	// on a node IP listening on 53 and delegating queries for dnsDomain to this process, while sending other
	// queries to the host environments nameservers.
	DNSBindAddress string

	// DNSNameservers is a list of ip:port values of recursive nameservers to forward queries to when running
	// a local DNS server if dnsBindAddress is set. If this value is empty, the DNS server will default to
	// the nameservers listed in /etc/resolv.conf. If you have configured dnsmasq or another DNS proxy on the
	// system, this value should be set to the upstream nameservers dnsmasq resolves with.
	DNSNameservers []string

	// DNSRecursiveResolvConf is a path to a resolv.conf file that contains settings for an upstream server.
	// Only the nameservers and port fields are used. The file must exist and parse correctly. It adds extra
	// nameservers to DNSNameservers if set.
	DNSRecursiveResolvConf string

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
	// DockerShimSocket is the location of the dockershim socket the kubelet uses.
	DockerShimSocket string
	// DockershimRootDirectory is the dockershim root directory.
	DockershimRootDirectory string
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MasterConfig struct {
	metav1.TypeMeta

	// ServingInfo describes how to start serving
	ServingInfo HTTPServingInfo

	// AuthConfig configures authentication options in addition to the standard
	// oauth token and client certificate authenticators
	AuthConfig MasterAuthConfig

	// AggregatorConfig has options for configuring the aggregator component of the API server.
	AggregatorConfig AggregatorConfig

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
	// Deprecated: Will be removed in 3.7.
	PauseControllers bool
	// ControllerLeaseTTL enables controller election against etcd, instructing the master to attempt to
	// acquire a lease before controllers start and renewing it within a number of seconds defined by this
	// value. Setting this value non-negative forces pauseControllers=true. This value defaults off (0, or
	// omitted) and controller election can be disabled with -1.
	// Deprecated: use controllerConfig.lockServiceName to force leader election via config, and the
	//   appropriate leader election flags in controllerArguments. Will be removed in 3.9.
	ControllerLeaseTTL int
	// TODO: the next field added to controllers must be added to a new controllers struct

	// AdmissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig

	ControllerConfig ControllerConfig

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

	// DisableOpenAPI avoids starting the openapi endpoint because it is very expensive.
	// This option will be removed at a later time.  It is never serialized.
	DisableOpenAPI bool
}

// MasterAuthConfig configures authentication options in addition to the standard
// oauth token and client certificate authenticators
type MasterAuthConfig struct {
	// RequestHeader holds options for setting up a front proxy against the the API.  It is optional.
	RequestHeader *RequestHeaderAuthenticationOptions
	// WebhookTokenAuthnConfig, if present configures remote token reviewers
	WebhookTokenAuthenticators []WebhookTokenAuthenticator
}

// RequestHeaderAuthenticationOptions provides options for setting up a front proxy against the entire
// API instead of against the /oauth endpoint.
type RequestHeaderAuthenticationOptions struct {
	// ClientCA is a file with the trusted signer certs.  It is required.
	ClientCA string
	// ClientCommonNames is a required list of common names to require a match from.
	ClientCommonNames []string

	// UsernameHeaders is the list of headers to check for user information.  First hit wins.
	UsernameHeaders []string
	// GroupNameHeader is the set of headers to check for group information.  All are unioned.
	GroupHeaders []string
	// ExtraHeaderPrefixes is the set of request header prefixes to inspect for user extra. X-Remote-Extra- is suggested.
	ExtraHeaderPrefixes []string
}

// AggregatorConfig holds information required to make the aggregator function.
type AggregatorConfig struct {
	// ProxyClientInfo specifies the client cert/key to use when proxying to aggregated API servers
	ProxyClientInfo CertInfo
}

type LogFormatType string

type WebHookModeType string

const (
	// LogFormatLegacy saves event in 1-line text format.
	LogFormatLegacy LogFormatType = "legacy"
	// LogFormatJson saves event in structured json format.
	LogFormatJson LogFormatType = "json"

	// WebHookModeBatch indicates that the webhook should buffer audit events
	// internally, sending batch updates either once a certain number of
	// events have been received or a certain amount of time has passed.
	WebHookModeBatch WebHookModeType = "batch"
	// WebHookModeBlocking causes the webhook to block on every attempt to process
	// a set of events. This causes requests to the API server to wait for a
	// round trip to the external audit service before sending a response.
	WebHookModeBlocking WebHookModeType = "blocking"
)

// AuditConfig holds configuration for the audit capabilities
type AuditConfig struct {
	// If this flag is set, audit log will be printed in the logs.
	Enabled bool
	// All requests coming to the apiserver will be logged to this file.
	AuditFilePath string
	// This field is used to hold actual value of the AuditFilePath as entered
	// by the user for validation.
	InternalAuditFilePath string
	// Maximum number of days to retain old log files based on the timestamp encoded in their filename.
	MaximumFileRetentionDays int
	// Maximum number of old log files to retain.
	MaximumRetainedFiles int
	// Maximum size in megabytes of the log file before it gets rotated. Defaults to 100MB.
	MaximumFileSizeMegabytes int

	// PolicyFile is a path to the file that defines the audit policy configuration.
	PolicyFile string
	// PolicyConfiguration is an embedded policy configuration object to be used
	// as the audit policy configuration. If present, it will be used instead of
	// the path to the policy file.
	PolicyConfiguration runtime.Object

	// Format of saved audits (legacy or json).
	LogFormat LogFormatType

	// Path to a .kubeconfig formatted file that defines the audit webhook configuration.
	WebHookKubeConfig string
	// Strategy for sending audit events (block or batch).
	WebHookMode WebHookModeType
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
	// AllowedRegistriesForImport limits the docker registries that normal users may import
	// images from. Set this list to the registries that you trust to contain valid Docker
	// images and that you want applications to be able to import from. Users with
	// permission to create Images or ImageStreamMappings via the API are not affected by
	// this policy - typically only administrators or system integrations will have those
	// permissions.
	AllowedRegistriesForImport *AllowedRegistries
	// InternalRegistryHostname sets the hostname for the default internal image
	// registry. The value must be in "hostname[:port]" format.
	// For backward compatibility, users can still use OPENSHIFT_DEFAULT_REGISTRY
	// environment variable but this setting overrides the environment variable.
	InternalRegistryHostname string
	// ExternalRegistryHostname sets the hostname for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	ExternalRegistryHostname string
}

// AllowedRegistries represents a list of registries allowed for the image import.
type AllowedRegistries []RegistryLocation

// RegistryLocation contains a location of the registry specified by the registry domain
// name. The domain name might include wildcards, like '*' or '??'.
type RegistryLocation struct {
	// DomainName specifies a domain name for the registry
	// In case the registry use non-standard (80 or 443) port, the port should be included
	// in the domain name as well.
	DomainName string
	// Insecure indicates whether the registry is secure (https) or insecure (http)
	// By default (if not specified) the registry is assumed as secure.
	Insecure bool
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
	NetworkPluginName            string
	DeprecatedClusterNetworkCIDR string
	// ClusterNetworks contains a list of cluster networks that defines the global overlay networks L3 space.
	ClusterNetworks            []ClusterNetworkEntry
	DeprecatedHostSubnetLength uint32
	ServiceNetworkCIDR         string
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

// ClusterNetworkEntry defines an individual cluster network. The CIDRs cannot overlap with other cluster network CIDRs, CIDRs
// reserved for external ips, CIDRs reserved for service networks, and CIDRs reserved for ingress ips.
type ClusterNetworkEntry struct {
	// CIDR defines the total range of a cluster networks address space.
	CIDR string
	// HostSubnetLength gives the number of address bits reserved for pod IPs on each node.
	HostSubnetLength uint32
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
	// MinTLSVersion is the minimum TLS version supported.
	// Values must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants
	MinTLSVersion string
	// CipherSuites contains an overridden list of ciphers for the server to support.
	// Values must match cipher suite IDs from https://golang.org/pkg/crypto/tls/#pkg-constants
	CipherSuites []string
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

type WebhookTokenAuthenticator struct {
	// ConfigFile is a path to a Kubeconfig file with the webhook configuration
	ConfigFile string
	// CacheTTL indicates how long an authentication result should be cached.
	// It takes a valid time duration string (e.g. "5m").
	// If empty, you get a default timeout of 2 minutes.
	// If zero (e.g. "0m"), caching is disabled
	CacheTTL string
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
	// AccessTokenInactivityTimeoutSeconds defined the default token
	// inactivity timeout for tokens granted by any client.
	// Setting it to nil means the feature is completely disabled (default)
	// The default setting can be overriden on OAuthClient basis.
	// The value represents the maximum amount of time that can occur between
	// consecutive uses of the token. Tokens become invalid if they are not
	// used within this temporal window. The user will need to acquire a new
	// token to regain access once a token times out.
	// Valid values are:
	// - 0: Tokens never time out
	// - X: Tokens time out if there is no activity for X seconds
	// The current minimum allowed value for X is 300 (5 minutes)
	AccessTokenInactivityTimeoutSeconds *int32
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SessionSecrets list the secrets to use to sign/encrypt and authenticate/decrypt created sessions.
type SessionSecrets struct {
	metav1.TypeMeta

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BasicAuthPasswordIdentityProvider struct {
	metav1.TypeMeta

	// RemoteConnectionInfo contains information about how to connect to the external basic auth server
	RemoteConnectionInfo RemoteConnectionInfo
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AllowAllPasswordIdentityProvider struct {
	metav1.TypeMeta
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DenyAllPasswordIdentityProvider struct {
	metav1.TypeMeta
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HTPasswdPasswordIdentityProvider struct {
	metav1.TypeMeta

	// File is a reference to your htpasswd file
	File string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LDAPPasswordIdentityProvider struct {
	metav1.TypeMeta
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeystonePasswordIdentityProvider struct {
	metav1.TypeMeta
	// RemoteConnectionInfo contains information about how to connect to the keystone server
	RemoteConnectionInfo RemoteConnectionInfo
	// Domain Name is required for keystone v3
	DomainName string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RequestHeaderIdentityProvider struct {
	metav1.TypeMeta

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GitHubIdentityProvider struct {
	metav1.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource
	// Organizations optionally restricts which organizations are allowed to log in
	Organizations []string
	// Teams optionally restricts which teams are allowed to log in. Format is <org>/<team>.
	Teams []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GitLabIdentityProvider struct {
	metav1.TypeMeta

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GoogleIdentityProvider struct {
	metav1.TypeMeta

	// ClientID is the oauth client ID
	ClientID string
	// ClientSecret is the oauth client secret
	ClientSecret StringSource

	// HostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenIDIdentityProvider struct {
	metav1.TypeMeta

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
	// MasterEndpointReconcileTTL sets the time to live in seconds of an endpoint record recorded by each master. The endpoints are checked
	// at an interval that is 2/3 of this value and this value defaults to 15s if unset. In very large clusters, this value may be increased to
	// reduce the possibility that the master endpoint record expires (due to other load on the etcd server) and causes masters to drop in and
	// out of the kubernetes service record. It is not recommended to set this value below 15s.
	MasterEndpointReconcileTTL int
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LDAPSyncConfig struct {
	metav1.TypeMeta

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
	// configuration.
	Location string

	// Configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.Object
}

type AdmissionConfig struct {
	// PluginConfig allows specifying a configuration file per admission control plugin
	PluginConfig map[string]*AdmissionPluginConfig

	// PluginOrderOverride is a list of admission control plugin names that will be installed
	// on the master. Order is significant. If empty, a default list of plugins is used.
	PluginOrderOverride []string
}

// ControllerConfig holds configuration values for controllers
type ControllerConfig struct {
	// Controllers is a list of controllers to enable.  '*' enables all on-by-default controllers, 'foo' enables the controller "+
	// named 'foo', '-foo' disables the controller named 'foo'.
	// Defaults to "*".
	Controllers []string
	// Election defines the configuration for electing a controller instance to make changes to
	// the cluster. If unspecified, the ControllerTTL value is checked to determine whether the
	// legacy direct etcd election code will be used.
	Election *ControllerElectionConfig
	// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
	// pods fulfilling a service to serve with.
	ServiceServingCert ServiceServingCert
}

// ControllerElectionConfig contains configuration values for deciding how a controller
// will be elected to act as leader.
type ControllerElectionConfig struct {
	// LockName is the resource name used to act as the lock for determining which controller
	// instance should lead.
	LockName string
	// LockNamespace is the resource namespace used to act as the lock for determining which
	// controller instance should lead. It defaults to "kube-system"
	LockNamespace string
	// LockResource is the group and resource name to use to coordinate for the controller lock.
	// If unset, defaults to "endpoints".
	LockResource GroupResource
}

// GroupResource points to a resource by its name and API group.
type GroupResource struct {
	// Group is the name of an API group
	Group string
	// Resource is the name of a resource.
	Resource string
}

// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
// pods fulfilling a service to serve with.
type ServiceServingCert struct {
	// Signer holds the signing information used to automatically sign serving certificates.
	// If this value is nil, then certs are not signed automatically.
	Signer *CertInfo
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DefaultAdmissionConfig can be used to enable or disable various admission plugins.
// When this type is present as the `configuration` object under `pluginConfig` and *if* the admission plugin supports it,
// this will cause an "off by default" admission plugin to be enabled
type DefaultAdmissionConfig struct {
	metav1.TypeMeta

	// Disable turns off an admission plugin that is enabled by default.
	Disable bool
}
