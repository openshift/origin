package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	buildv1 "github.com/openshift/api/build/v1"
)

type ExtendedArguments map[string][]string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeConfig is the fully specified config starting an OpenShift node
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type NodeConfig struct {
	metav1.TypeMeta `json:",inline"`

	// nodeName is the value used to identify this particular node in the cluster.  If possible, this should be your fully qualified hostname.
	// If you're describing a set of static nodes to the master, this value must match one of the values in the list
	NodeName string `json:"nodeName"`

	// Node may have multiple IPs, specify the IP to use for pod traffic routing
	// If not specified, network parse/lookup on the nodeName is performed and the first non-loopback address is used
	NodeIP string `json:"nodeIP"`

	// servingInfo describes how to start serving
	ServingInfo ServingInfo `json:"servingInfo"`

	// masterKubeConfig is a filename for the .kubeconfig file that describes how to connect this node to the master
	MasterKubeConfig string `json:"masterKubeConfig"`

	// masterClientConnectionOverrides provides overrides to the client connection used to connect to the master.
	MasterClientConnectionOverrides *ClientConnectionOverrides `json:"masterClientConnectionOverrides"`

	// dnsDomain holds the domain suffix that will be used for the DNS search path inside each container. Defaults to
	// 'cluster.local'.
	DNSDomain string `json:"dnsDomain"`

	// dnsIP is the IP address that pods will use to access cluster DNS. Defaults to the service IP of the Kubernetes
	// master. This IP must be listening on port 53 for compatibility with libc resolvers (which cannot be configured
	// to resolve names from any other port). When running more complex local DNS configurations, this is often set
	// to the local address of a DNS proxy like dnsmasq, which then will consult either the local DNS (see
	// dnsBindAddress) or the master DNS.
	DNSIP string `json:"dnsIP"`

	// dnsBindAddress is the ip:port to serve DNS on. If this is not set, the DNS server will not be started.
	// Because most DNS resolvers will only listen on port 53, if you select an alternative port you will need
	// a DNS proxy like dnsmasq to answer queries for containers. A common configuration is dnsmasq configured
	// on a node IP listening on 53 and delegating queries for dnsDomain to this process, while sending other
	// queries to the host environments nameservers.
	DNSBindAddress string `json:"dnsBindAddress"`

	// dnsNameservers is a list of ip:port values of recursive nameservers to forward queries to when running
	// a local DNS server if dnsBindAddress is set. If this value is empty, the DNS server will default to
	// the nameservers listed in /etc/resolv.conf. If you have configured dnsmasq or another DNS proxy on the
	// system, this value should be set to the upstream nameservers dnsmasq resolves with.
	DNSNameservers []string `json:"dnsNameservers"`

	// dnsRecursiveResolvConf is a path to a resolv.conf file that contains settings for an upstream server.
	// Only the nameservers and port fields are used. The file must exist and parse correctly. It adds extra
	// nameservers to DNSNameservers if set.
	DNSRecursiveResolvConf string `json:"dnsRecursiveResolvConf"`

	// Deprecated and maintained for backward compatibility, use NetworkConfig.NetworkPluginName instead
	DeprecatedNetworkPluginName string `json:"networkPluginName,omitempty"`

	// networkConfig provides network options for the node
	NetworkConfig NodeNetworkConfig `json:"networkConfig"`

	// volumeDirectory is the directory that volumes will be stored under
	VolumeDirectory string `json:"volumeDirectory"`

	// imageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig `json:"imageConfig"`

	// allowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool `json:"allowDisabledDocker"`

	// podManifestConfig holds the configuration for enabling the Kubelet to
	// create pods based from a manifest file(s) placed locally on the node
	PodManifestConfig *PodManifestConfig `json:"podManifestConfig"`

	// authConfig holds authn/authz configuration options
	AuthConfig NodeAuthConfig `json:"authConfig"`

	// dockerConfig holds Docker related configuration options.
	DockerConfig DockerConfig `json:"dockerConfig"`

	// kubeletArguments are key value pairs that will be passed directly to the Kubelet that match the Kubelet's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	KubeletArguments ExtendedArguments `json:"kubeletArguments,omitempty"`

	// proxyArguments are key value pairs that will be passed directly to the Proxy that match the Proxy's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	ProxyArguments ExtendedArguments `json:"proxyArguments,omitempty"`

	// iptablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string `json:"iptablesSyncPeriod"`

	// enableUnidling controls whether or not the hybrid unidling proxy will be set up
	EnableUnidling *bool `json:"enableUnidling"`

	// volumeConfig contains options for configuring volumes on the node.
	VolumeConfig NodeVolumeConfig `json:"volumeConfig"`
}

// NodeVolumeConfig contains options for configuring volumes on the node.
type NodeVolumeConfig struct {
	// localQuota contains options for controlling local volume quota on the node.
	LocalQuota LocalQuota `json:"localQuota"`
}

// MasterVolumeConfig contains options for configuring volume plugins in the master node.
type MasterVolumeConfig struct {
	// dynamicProvisioningEnabled is a boolean that toggles dynamic provisioning off when false, defaults to true
	DynamicProvisioningEnabled *bool `json:"dynamicProvisioningEnabled"`
}

// LocalQuota contains options for controlling local volume quota on the node.
type LocalQuota struct {
	// FSGroup can be specified to enable a quota on local storage use per unique FSGroup ID.
	// At present this is only implemented for emptyDir volumes, and if the underlying
	// volumeDirectory is on an XFS filesystem.
	PerFSGroup *resource.Quantity `json:"perFSGroup"`
}

// NodeAuthConfig holds authn/authz configuration options
type NodeAuthConfig struct {
	// authenticationCacheTTL indicates how long an authentication result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthenticationCacheTTL string `json:"authenticationCacheTTL"`

	// authenticationCacheSize indicates how many authentication results should be cached. If 0, the default cache size is used.
	AuthenticationCacheSize int `json:"authenticationCacheSize"`

	// authorizationCacheTTL indicates how long an authorization result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthorizationCacheTTL string `json:"authorizationCacheTTL"`

	// authorizationCacheSize indicates how many authorization results should be cached. If 0, the default cache size is used.
	AuthorizationCacheSize int `json:"authorizationCacheSize"`
}

// NodeNetworkConfig provides network options for the node
type NodeNetworkConfig struct {
	// networkPluginName is a string specifying the networking plugin
	NetworkPluginName string `json:"networkPluginName"`
	// Maximum transmission unit for the network packets
	MTU uint32 `json:"mtu"`
}

// DockerConfig holds Docker related configuration options.
type DockerConfig struct {
	// execHandlerName is the name of the handler to use for executing
	// commands in containers.
	ExecHandlerName DockerExecHandlerType `json:"execHandlerName"`
	// dockerShimSocket is the location of the dockershim socket the kubelet uses.
	// Currently unix socket is supported on Linux, and tcp is supported on windows.
	// Examples:'unix:///var/run/dockershim.sock', 'tcp://localhost:3735'
	DockerShimSocket string `json:"dockerShimSocket"`
	// dockerShimRootDirectory is the dockershim root directory.
	DockershimRootDirectory string `json:"dockerShimRootDirectory"`
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

// FeatureList contains a set of features
type FeatureList []string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MasterConfig holds the necessary configuration options for the OpenShift master
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type MasterConfig struct {
	metav1.TypeMeta `json:",inline"`

	// servingInfo describes how to start serving
	ServingInfo HTTPServingInfo `json:"servingInfo"`

	// authConfig configures authentication options in addition to the standard
	// oauth token and client certificate authenticators
	AuthConfig MasterAuthConfig `json:"authConfig"`

	// aggregatorConfig has options for configuring the aggregator component of the API server.
	AggregatorConfig AggregatorConfig `json:"aggregatorConfig"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins"`

	// apiLevels is a list of API levels that should be enabled on startup: v1 as examples
	APILevels []string `json:"apiLevels"`

	// masterPublicURL is how clients can access the OpenShift API server
	MasterPublicURL string `json:"masterPublicURL"`

	// controllers is a list of the controllers that should be started. If set to "none", no controllers
	// will start automatically. The default value is "*" which will start all controllers. When
	// using "*", you may exclude controllers by prepending a "-" in front of their name. No other
	// values are recognized at this time.
	Controllers string `json:"controllers"`

	// admissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig `json:"admissionConfig"`

	// controllerConfig holds configuration values for controllers
	ControllerConfig ControllerConfig `json:"controllerConfig"`

	// etcdStorageConfig contains information about how API resources are
	// stored in Etcd. These values are only relevant when etcd is the
	// backing store for the cluster.
	EtcdStorageConfig EtcdStorageConfig `json:"etcdStorageConfig"`

	// etcdClientInfo contains information about how to connect to etcd
	EtcdClientInfo EtcdConnectionInfo `json:"etcdClientInfo"`
	// kubeletClientInfo contains information about how to connect to kubelets
	KubeletClientInfo KubeletConnectionInfo `json:"kubeletClientInfo"`

	// KubernetesMasterConfig, if present start the kubernetes master in this process
	KubernetesMasterConfig KubernetesMasterConfig `json:"kubernetesMasterConfig"`
	// EtcdConfig, if present start etcd in this process
	EtcdConfig *EtcdConfig `json:"etcdConfig"`
	// OAuthConfig, if present start the /oauth endpoint in this process
	OAuthConfig *OAuthConfig `json:"oauthConfig"`

	// DNSConfig, if present start the DNS server in this process
	DNSConfig *DNSConfig `json:"dnsConfig"`

	// serviceAccountConfig holds options related to service accounts
	ServiceAccountConfig ServiceAccountConfig `json:"serviceAccountConfig"`

	// masterClients holds all the client connection information for controllers and other system components
	MasterClients MasterClients `json:"masterClients"`

	// imageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig `json:"imageConfig"`

	// imagePolicyConfig controls limits and behavior for importing images
	ImagePolicyConfig ImagePolicyConfig `json:"imagePolicyConfig"`

	// policyConfig holds information about where to locate critical pieces of bootstrapping policy
	PolicyConfig PolicyConfig `json:"policyConfig"`

	// projectConfig holds information about project creation and defaults
	ProjectConfig ProjectConfig `json:"projectConfig"`

	// routingConfig holds information about routing and route generation
	RoutingConfig RoutingConfig `json:"routingConfig"`

	// networkConfig to be passed to the compiled in network plugin
	NetworkConfig MasterNetworkConfig `json:"networkConfig"`

	// MasterVolumeConfig contains options for configuring volume plugins in the master node.
	VolumeConfig MasterVolumeConfig `json:"volumeConfig"`

	// jenkinsPipelineConfig holds information about the default Jenkins template
	// used for JenkinsPipeline build strategy.
	JenkinsPipelineConfig JenkinsPipelineConfig `json:"jenkinsPipelineConfig"`

	// auditConfig holds information related to auditing capabilities.
	AuditConfig AuditConfig `json:"auditConfig"`

	// DisableOpenAPI avoids starting the openapi endpoint because it is very expensive.
	// This option will be removed at a later time.  It is never serialized.
	DisableOpenAPI bool `json:"-"`
}

// MasterAuthConfig configures authentication options in addition to the standard
// oauth token and client certificate authenticators
type MasterAuthConfig struct {
	// requestHeader holds options for setting up a front proxy against the API.  It is optional.
	RequestHeader *RequestHeaderAuthenticationOptions `json:"requestHeader"`
	// WebhookTokenAuthnConfig, if present configures remote token reviewers
	WebhookTokenAuthenticators []WebhookTokenAuthenticator `json:"webhookTokenAuthenticators"`
	// oauthMetadataFile is a path to a file containing the discovery endpoint for OAuth 2.0 Authorization
	// Server Metadata for an external OAuth server.
	// See IETF Draft: // https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	// This option is mutually exclusive with OAuthConfig
	OAuthMetadataFile string `json:"oauthMetadataFile"`
}

// RequestHeaderAuthenticationOptions provides options for setting up a front proxy against the entire
// API instead of against the /oauth endpoint.
type RequestHeaderAuthenticationOptions struct {
	// clientCA is a file with the trusted signer certs.  It is required.
	ClientCA string `json:"clientCA"`
	// clientCommonNames is a required list of common names to require a match from.
	ClientCommonNames []string `json:"clientCommonNames"`

	// usernameHeaders is the list of headers to check for user information.  First hit wins.
	UsernameHeaders []string `json:"usernameHeaders"`
	// GroupNameHeader is the set of headers to check for group information.  All are unioned.
	GroupHeaders []string `json:"groupHeaders"`
	// extraHeaderPrefixes is the set of request header prefixes to inspect for user extra. X-Remote-Extra- is suggested.
	ExtraHeaderPrefixes []string `json:"extraHeaderPrefixes"`
}

// AggregatorConfig holds information required to make the aggregator function.
type AggregatorConfig struct {
	// proxyClientInfo specifies the client cert/key to use when proxying to aggregated API servers
	ProxyClientInfo CertInfo `json:"proxyClientInfo"`
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
	// The logs contains, method, user and a requested URL.
	Enabled bool `json:"enabled"`
	// All requests coming to the apiserver will be logged to this file.
	AuditFilePath string `json:"auditFilePath"`
	// Maximum number of days to retain old log files based on the timestamp encoded in their filename.
	MaximumFileRetentionDays int `json:"maximumFileRetentionDays"`
	// Maximum number of old log files to retain.
	MaximumRetainedFiles int `json:"maximumRetainedFiles"`
	// Maximum size in megabytes of the log file before it gets rotated. Defaults to 100MB.
	MaximumFileSizeMegabytes int `json:"maximumFileSizeMegabytes"`

	// policyFile is a path to the file that defines the audit policy configuration.
	PolicyFile string `json:"policyFile"`
	// policyConfiguration is an embedded policy configuration object to be used
	// as the audit policy configuration. If present, it will be used instead of
	// the path to the policy file.
	PolicyConfiguration runtime.RawExtension `json:"policyConfiguration"`

	// Format of saved audits (legacy or json).
	LogFormat LogFormatType `json:"logFormat"`

	// Path to a .kubeconfig formatted file that defines the audit webhook configuration.
	WebHookKubeConfig string `json:"webHookKubeConfig"`
	// Strategy for sending audit events (block or batch).
	WebHookMode WebHookModeType `json:"webHookMode"`
}

// JenkinsPipelineConfig holds configuration for the Jenkins pipeline strategy
type JenkinsPipelineConfig struct {
	// autoProvisionEnabled determines whether a Jenkins server will be spawned from the provided
	// template when the first build config in the project with type JenkinsPipeline
	// is created. When not specified this option defaults to true.
	AutoProvisionEnabled *bool `json:"autoProvisionEnabled"`
	// templateNamespace contains the namespace name where the Jenkins template is stored
	TemplateNamespace string `json:"templateNamespace"`
	// templateName is the name of the default Jenkins template
	TemplateName string `json:"templateName"`
	// serviceName is the name of the Jenkins service OpenShift uses to detect
	// whether a Jenkins pipeline handler has already been installed in a project.
	// This value *must* match a service name in the provided template.
	ServiceName string `json:"serviceName"`
	// parameters specifies a set of optional parameters to the Jenkins template.
	Parameters map[string]string `json:"parameters"`
}

// ImagePolicyConfig holds the necessary configuration options for limits and behavior for importing images
type ImagePolicyConfig struct {
	// maxImagesBulkImportedPerRepository controls the number of images that are imported when a user
	// does a bulk import of a container repository. This number defaults to 50 to prevent users from
	// importing large numbers of images accidentally. Set -1 for no limit.
	MaxImagesBulkImportedPerRepository int `json:"maxImagesBulkImportedPerRepository"`
	// disableScheduledImport allows scheduled background import of images to be disabled.
	DisableScheduledImport bool `json:"disableScheduledImport"`
	// scheduledImageImportMinimumIntervalSeconds is the minimum number of seconds that can elapse between when image streams
	// scheduled for background import are checked against the upstream repository. The default value is 15 minutes.
	ScheduledImageImportMinimumIntervalSeconds int `json:"scheduledImageImportMinimumIntervalSeconds"`
	// maxScheduledImageImportsPerMinute is the maximum number of scheduled image streams that will be imported in the
	// background per minute. The default value is 60. Set to -1 for unlimited.
	MaxScheduledImageImportsPerMinute int `json:"maxScheduledImageImportsPerMinute"`
	// allowedRegistriesForImport limits the container image registries that normal users may import
	// images from. Set this list to the registries that you trust to contain valid Docker
	// images and that you want applications to be able to import from. Users with
	// permission to create Images or ImageStreamMappings via the API are not affected by
	// this policy - typically only administrators or system integrations will have those
	// permissions.
	AllowedRegistriesForImport *AllowedRegistries `json:"allowedRegistriesForImport,omitempty"`
	// internalRegistryHostname sets the hostname for the default internal image
	// registry. The value must be in "hostname[:port]" format.
	InternalRegistryHostname string `json:"internalRegistryHostname,omitempty"`
	// externalRegistryHostname sets the hostname for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	ExternalRegistryHostname string `json:"externalRegistryHostname,omitempty"`
	// additionalTrustedCA is a path to a pem bundle file containing additional CAs that
	// should be trusted during imagestream import.
	AdditionalTrustedCA string `json:"additionalTrustedCA,omitempty"`
}

// AllowedRegistries represents a list of registries allowed for the image import.
type AllowedRegistries []RegistryLocation

// RegistryLocation contains a location of the registry specified by the registry domain
// name. The domain name might include wildcards, like '*' or '??'.
type RegistryLocation struct {
	// domainName specifies a domain name for the registry
	// In case the registry use non-standard (80 or 443) port, the port should be included
	// in the domain name as well.
	DomainName string `json:"domainName"`
	// insecure indicates whether the registry is secure (https) or insecure (http)
	// By default (if not specified) the registry is assumed as secure.
	Insecure bool `json:"insecure,omitempty"`
}

// holds the necessary configuration options for
type ProjectConfig struct {
	// defaultNodeSelector holds default project node label selector
	DefaultNodeSelector string `json:"defaultNodeSelector"`

	// projectRequestMessage is the string presented to a user if they are unable to request a project via the projectrequest api endpoint
	ProjectRequestMessage string `json:"projectRequestMessage"`

	// projectRequestTemplate is the template to use for creating projects in response to projectrequest.
	// It is in the format namespace/template and it is optional.
	// If it is not specified, a default template is used.
	ProjectRequestTemplate string `json:"projectRequestTemplate"`

	// securityAllocator controls the automatic allocation of UIDs and MCS labels to a project. If nil, allocation is disabled.
	SecurityAllocator *SecurityAllocator `json:"securityAllocator"`
}

// SecurityAllocator controls the automatic allocation of UIDs and MCS labels to a project. If nil, allocation is disabled.
type SecurityAllocator struct {
	// uidAllocatorRange defines the total set of Unix user IDs (UIDs) that will be allocated to projects automatically, and the size of the
	// block each namespace gets. For example, 1000-1999/10 will allocate ten UIDs per namespace, and will be able to allocate up to 100 blocks
	// before running out of space. The default is to allocate from 1 billion to 2 billion in 10k blocks (which is the expected size of the
	// ranges container images will use once user namespaces are started).
	UIDAllocatorRange string `json:"uidAllocatorRange"`
	// mcsAllocatorRange defines the range of MCS categories that will be assigned to namespaces. The format is
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
	// mcsLabelsPerProject defines the number of labels that should be reserved per project. The default is 5 to match the default UID and MCS
	// ranges (100k namespaces, 535k/5 labels).
	MCSLabelsPerProject int `json:"mcsLabelsPerProject"`
}

// holds the necessary configuration options for
type PolicyConfig struct {
	// userAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
	UserAgentMatchingConfig UserAgentMatchingConfig `json:"userAgentMatchingConfig"`
}

// UserAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
type UserAgentMatchingConfig struct {
	// If this list is non-empty, then a User-Agent must match one of the UserAgentRegexes to be allowed
	RequiredClients []UserAgentMatchRule `json:"requiredClients"`

	// If this list is non-empty, then a User-Agent must not match any of the UserAgentRegexes
	DeniedClients []UserAgentDenyRule `json:"deniedClients"`

	// defaultRejectionMessage is the message shown when rejecting a client.  If it is not a set, a generic message is given.
	DefaultRejectionMessage string `json:"defaultRejectionMessage"`
}

// UserAgentMatchRule describes how to match a given request based on User-Agent and HTTPVerb
type UserAgentMatchRule struct {
	// UserAgentRegex is a regex that is checked against the User-Agent.
	// Known variants of oc clients
	// 1. oc accessing kube resources: oc/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 2. oc accessing openshift resources: oc/v1.1.3 (linux/amd64) openshift/b348c2f
	// 3. openshift kubectl accessing kube resources:  openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 4. openshift kubectl accessing openshift resources: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
	// 5. oadm accessing kube resources: oadm/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 6. oadm accessing openshift resources: oadm/v1.1.3 (linux/amd64) openshift/b348c2f
	// 7. openshift cli accessing kube resources: openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 8. openshift cli accessing openshift resources: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
	Regex string `json:"regex"`

	// httpVerbs specifies which HTTP verbs should be matched.  An empty list means "match all verbs".
	HTTPVerbs []string `json:"httpVerbs"`
}

// UserAgentDenyRule adds a rejection message that can be used to help a user figure out how to get an approved client
type UserAgentDenyRule struct {
	UserAgentMatchRule `json:",inline"`

	// rejectionMessage is the message shown when rejecting a client.  If it is not a set, the default message is used.
	RejectionMessage string `json:"rejectionMessage"`
}

// RoutingConfig holds the necessary configuration options for routing to subdomains
type RoutingConfig struct {
	// subdomain is the suffix appended to $service.$namespace. to form the default route hostname
	// DEPRECATED: This field is being replaced by routers setting their own defaults. This is the
	// "default" route.
	Subdomain string `json:"subdomain"`
}

// MasterNetworkConfig to be passed to the compiled in network plugin
type MasterNetworkConfig struct {
	// networkPluginName is the name of the network plugin to use
	NetworkPluginName string `json:"networkPluginName"`
	// clusterNetworkCIDR is the CIDR string to specify the global overlay network's L3 space.  Deprecated, but maintained for backwards compatibility, use ClusterNetworks instead.
	DeprecatedClusterNetworkCIDR string `json:"clusterNetworkCIDR,omitempty"`
	// clusterNetworks is a list of ClusterNetwork objects that defines the global overlay network's L3 space by specifying a set of CIDR and netmasks that the SDN can allocate addressed from.  If this is specified, then ClusterNetworkCIDR and HostSubnetLength may not be set.
	ClusterNetworks []ClusterNetworkEntry `json:"clusterNetworks"`
	// hostSubnetLength is the number of bits to allocate to each host's subnet e.g. 8 would mean a /24 network on the host.  Deprecated, but maintained for backwards compatibility, use ClusterNetworks instead.
	DeprecatedHostSubnetLength uint32 `json:"hostSubnetLength,omitempty"`
	// ServiceNetwork is the CIDR string to specify the service networks
	ServiceNetworkCIDR string `json:"serviceNetworkCIDR"`
	// externalIPNetworkCIDRs controls what values are acceptable for the service external IP field. If empty, no externalIP
	// may be set. It may contain a list of CIDRs which are checked for access. If a CIDR is prefixed with !, IPs in that
	// CIDR will be rejected. Rejections will be applied first, then the IP checked against one of the allowed CIDRs. You
	// should ensure this range does not overlap with your nodes, pods, or service CIDRs for security reasons.
	ExternalIPNetworkCIDRs []string `json:"externalIPNetworkCIDRs"`
	// ingressIPNetworkCIDR controls the range to assign ingress ips from for services of type LoadBalancer on bare
	// metal. If empty, ingress ips will not be assigned. It may contain a single CIDR that will be allocated from.
	// For security reasons, you should ensure that this range does not overlap with the CIDRs reserved for external ips,
	// nodes, pods, or services.
	IngressIPNetworkCIDR string `json:"ingressIPNetworkCIDR"`
	// vxlanPort is the VXLAN port used by the cluster defaults. If it is not set, 4789 is the default value
	VXLANPort uint32 `json:"vxlanPort,omitempty"`
}

// ClusterNetworkEntry defines an individual cluster network. The CIDRs cannot overlap with other cluster network CIDRs, CIDRs reserved for external ips, CIDRs reserved for service networks, and CIDRs reserved for ingress ips.
type ClusterNetworkEntry struct {
	// cidr defines the total range of a cluster networks address space.
	CIDR string `json:"cidr"`
	// hostSubnetLength is the number of bits of the accompanying CIDR address to allocate to each node. eg, 8 would mean that each node would have a /24 slice of the overlay network for its pod.
	HostSubnetLength uint32 `json:"hostSubnetLength"`
}

// ImageConfig holds the necessary configuration options for building image names for system components
type ImageConfig struct {
	// format is the format of the name to be built for the system component
	Format string `json:"format"`
	// latest determines if the latest tag will be pulled from the registry
	Latest bool `json:"latest"`
}

// RemoteConnectionInfo holds information necessary for establishing a remote connection
type RemoteConnectionInfo struct {
	// url is the remote URL to connect to
	URL string `json:"url"`
	// ca is the CA for verifying TLS connections
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information to present
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// KubeletConnectionInfo holds information necessary for connecting to a kubelet
type KubeletConnectionInfo struct {
	// port is the port to connect to kubelets on
	Port uint `json:"port"`
	// ca is the CA for verifying TLS connections to kubelets
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to kubelets
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// EtcdConnectionInfo holds information necessary for connecting to an etcd server
type EtcdConnectionInfo struct {
	// urls are the URLs for etcd
	URLs []string `json:"urls"`
	// ca is a file containing trusted roots for the etcd server certificates
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to etcd
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// EtcdStorageConfig holds the necessary configuration options for the etcd storage underlying OpenShift and Kubernetes
type EtcdStorageConfig struct {
	// kubernetesStorageVersion is the API version that Kube resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	KubernetesStorageVersion string `json:"kubernetesStorageVersion"`
	// kubernetesStoragePrefix is the path within etcd that the Kubernetes resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located. The default value is 'kubernetes.io'.
	KubernetesStoragePrefix string `json:"kubernetesStoragePrefix"`
	// openShiftStorageVersion is the API version that OS resources in etcd should be
	// serialized to. This value should *not* be advanced until all clients in the
	// cluster that read from etcd have code that allows them to read the new version.
	OpenShiftStorageVersion string `json:"openShiftStorageVersion"`
	// openShiftStoragePrefix is the path within etcd that the OpenShift resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located. The default value is 'openshift.io'.
	OpenShiftStoragePrefix string `json:"openShiftStoragePrefix"`
}

// ServingInfo holds information about serving web pages
type ServingInfo struct {
	// bindAddress is the ip:port to serve on
	BindAddress string `json:"bindAddress"`
	// bindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string `json:"bindNetwork"`
	// CertInfo is the TLS cert info for serving secure traffic.
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
	// clientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string `json:"clientCA"`
	// namedCertificates is a list of certificates to use to secure requests to specific hostnames
	NamedCertificates []NamedCertificate `json:"namedCertificates"`
	// minTLSVersion is the minimum TLS version supported.
	// Values must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants
	MinTLSVersion string `json:"minTLSVersion,omitempty"`
	// cipherSuites contains an overridden list of ciphers for the server to support.
	// Values must match cipher suite IDs from https://golang.org/pkg/crypto/tls/#pkg-constants
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

// NamedCertificate specifies a certificate/key, and the names it should be served for
type NamedCertificate struct {
	// names is a list of DNS names this certificate should be used to secure
	// A name can be a normal DNS name, or can contain leading wildcard segments.
	Names []string `json:"names"`
	// CertInfo is the TLS cert info for serving secure traffic
	CertInfo `json:",inline"`
}

// HTTPServingInfo holds configuration for serving HTTP
type HTTPServingInfo struct {
	// ServingInfo is the HTTP serving information
	ServingInfo `json:",inline"`
	// maxRequestsInFlight is the number of concurrent requests allowed to the server. If zero, no limit.
	MaxRequestsInFlight int `json:"maxRequestsInFlight"`
	// requestTimeoutSeconds is the number of seconds before requests are timed out. The default is 60 minutes, if
	// -1 there is no limit on requests.
	RequestTimeoutSeconds int `json:"requestTimeoutSeconds"`
}

// MasterClients holds references to `.kubeconfig` files that qualify master clients for OpenShift and Kubernetes
type MasterClients struct {
	// openshiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string `json:"openshiftLoopbackKubeConfig"`

	// openshiftLoopbackClientConnectionOverrides specifies client overrides for system components to loop back to this master.
	OpenShiftLoopbackClientConnectionOverrides *ClientConnectionOverrides `json:"openshiftLoopbackClientConnectionOverrides"`
}

// ClientConnectionOverrides are a set of overrides to the default client connection settings.
type ClientConnectionOverrides struct {
	// acceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string `json:"acceptContentTypes"`
	// contentType is the content type used when sending data to the server from this client.
	ContentType string `json:"contentType"`

	// qps controls the number of queries per second allowed for this connection.
	QPS float32 `json:"qps"`
	// burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32 `json:"burst"`
}

// DNSConfig holds the necessary configuration options for DNS
type DNSConfig struct {
	// bindAddress is the ip:port to serve DNS on
	BindAddress string `json:"bindAddress"`
	// bindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string `json:"bindNetwork"`
	// allowRecursiveQueries allows the DNS server on the master to answer queries recursively. Note that open
	// resolvers can be used for DNS amplification attacks and the master DNS should not be made accessible
	// to public networks.
	AllowRecursiveQueries bool `json:"allowRecursiveQueries"`
}

// WebhookTokenAuthenticators holds the necessary configuation options for
// external token authenticators
type WebhookTokenAuthenticator struct {
	// configFile is a path to a Kubeconfig file with the webhook configuration
	ConfigFile string `json:"configFile"`
	// cacheTTL indicates how long an authentication result should be cached.
	// It takes a valid time duration string (e.g. "5m").
	// If empty, you get a default timeout of 2 minutes.
	// If zero (e.g. "0m"), caching is disabled
	CacheTTL string `json:"cacheTTL"`
}

// OAuthConfig holds the necessary configuration options for OAuth authentication
type OAuthConfig struct {
	// masterCA is the CA for verifying the TLS connection back to the MasterURL.
	MasterCA *string `json:"masterCA"`

	// masterURL is used for making server-to-server calls to exchange authorization codes for access tokens
	MasterURL string `json:"masterURL"`

	// masterPublicURL is used for building valid client redirect URLs for internal and external access
	MasterPublicURL string `json:"masterPublicURL"`

	// assetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string `json:"assetPublicURL"`

	// alwaysShowProviderSelection will force the provider selection page to render even when there is only a single provider.
	AlwaysShowProviderSelection bool `json:"alwaysShowProviderSelection"`

	// identityProviders is an ordered list of ways for a user to identify themselves
	IdentityProviders []IdentityProvider `json:"identityProviders"`

	// grantConfig describes how to handle grants
	GrantConfig GrantConfig `json:"grantConfig"`

	// sessionConfig hold information about configuring sessions.
	SessionConfig *SessionConfig `json:"sessionConfig"`

	// tokenConfig contains options for authorization and access tokens
	TokenConfig TokenConfig `json:"tokenConfig"`

	// templates allow you to customize pages like the login page.
	Templates *OAuthTemplates `json:"templates"`
}

// OAuthTemplates allow for customization of pages like the login page
type OAuthTemplates struct {
	// login is a path to a file containing a go template used to render the login page.
	// If unspecified, the default login page is used.
	Login string `json:"login"`

	// providerSelection is a path to a file containing a go template used to render the provider selection page.
	// If unspecified, the default provider selection page is used.
	ProviderSelection string `json:"providerSelection"`

	// error is a path to a file containing a go template used to render error pages during the authentication or grant flow
	// If unspecified, the default error page is used.
	Error string `json:"error"`
}

// ServiceAccountConfig holds the necessary configuration options for a service account
type ServiceAccountConfig struct {
	// managedNames is a list of service account names that will be auto-created in every namespace.
	// If no names are specified, the ServiceAccountsController will not be started.
	ManagedNames []string `json:"managedNames"`

	// limitSecretReferences controls whether or not to allow a service account to reference any secret in a namespace
	// without explicitly referencing them
	LimitSecretReferences bool `json:"limitSecretReferences"`

	// privateKeyFile is a file containing a PEM-encoded private RSA key, used to sign service account tokens.
	// If no private key is specified, the service account TokensController will not be started.
	PrivateKeyFile string `json:"privateKeyFile"`

	// publicKeyFiles is a list of files, each containing a PEM-encoded public RSA key.
	// (If any file contains a private key, the public portion of the key is used)
	// The list of public keys is used to verify presented service account tokens.
	// Each key is tried in order until the list is exhausted or verification succeeds.
	// If no keys are specified, no service account authentication will be available.
	PublicKeyFiles []string `json:"publicKeyFiles"`

	// masterCA is the CA for verifying the TLS connection back to the master.  The service account controller will automatically
	// inject the contents of this file into pods so they can verify connections to the master.
	MasterCA string `json:"masterCA"`
}

// TokenConfig holds the necessary configuration options for authorization and access tokens
type TokenConfig struct {
	// authorizeTokenMaxAgeSeconds defines the maximum age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32 `json:"authorizeTokenMaxAgeSeconds"`
	// accessTokenMaxAgeSeconds defines the maximum age of access tokens
	AccessTokenMaxAgeSeconds int32 `json:"accessTokenMaxAgeSeconds"`
	// accessTokenInactivityTimeoutSeconds defined the default token
	// inactivity timeout for tokens granted by any client.
	// Setting it to nil means the feature is completely disabled (default)
	// The default setting can be overridden on OAuthClient basis.
	// The value represents the maximum amount of time that can occur between
	// consecutive uses of the token. Tokens become invalid if they are not
	// used within this temporal window. The user will need to acquire a new
	// token to regain access once a token times out.
	// Valid values are:
	// - 0: Tokens never time out
	// - X: Tokens time out if there is no activity for X seconds
	// The current minimum allowed value for X is 300 (5 minutes)
	AccessTokenInactivityTimeoutSeconds *int32 `json:"accessTokenInactivityTimeoutSeconds,omitempty"`
}

// SessionConfig specifies options for cookie-based sessions. Used by AuthRequestHandlerSession
type SessionConfig struct {
	// sessionSecretsFile is a reference to a file containing a serialized SessionSecrets object
	// If no file is specified, a random signing and encryption key are generated at each server start
	SessionSecretsFile string `json:"sessionSecretsFile"`
	// sessionMaxAgeSeconds specifies how long created sessions last. Used by AuthRequestHandlerSession
	SessionMaxAgeSeconds int32 `json:"sessionMaxAgeSeconds"`
	// sessionName is the cookie name used to store the session
	SessionName string `json:"sessionName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SessionSecrets list the secrets to use to sign/encrypt and authenticate/decrypt created sessions.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type SessionSecrets struct {
	metav1.TypeMeta `json:",inline"`

	// secrets is a list of secrets
	// New sessions are signed and encrypted using the first secret.
	// Existing sessions are decrypted/authenticated by each secret until one succeeds. This allows rotating secrets.
	Secrets []SessionSecret `json:"secrets"`
}

// SessionSecret is a secret used to authenticate/decrypt cookie-based sessions
type SessionSecret struct {
	// authentication is used to authenticate sessions using HMAC. Recommended to use a secret with 32 or 64 bytes.
	Authentication string `json:"authentication"`
	// encryption is used to encrypt sessions. Must be 16, 24, or 32 characters long, to select AES-128, AES-
	Encryption string `json:"encryption"`
}

// IdentityProvider provides identities for users authenticating using credentials
type IdentityProvider struct {
	// name is used to qualify the identities returned by this provider
	Name string `json:"name"`
	// UseAsChallenger indicates whether to issue WWW-Authenticate challenges for this provider
	UseAsChallenger bool `json:"challenge"`
	// UseAsLogin indicates whether to use this identity provider for unauthenticated browsers to login against
	UseAsLogin bool `json:"login"`
	// mappingMethod determines how identities from this provider are mapped to users
	MappingMethod string `json:"mappingMethod"`
	// provider contains the information about how to set up a specific identity provider
	Provider runtime.RawExtension `json:"provider"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BasicAuthPasswordIdentityProvider provides identities for users authenticating using HTTP basic auth credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type BasicAuthPasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// RemoteConnectionInfo contains information about how to connect to the external basic auth server
	RemoteConnectionInfo `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AllowAllPasswordIdentityProvider provides identities for users authenticating using non-empty passwords
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type AllowAllPasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DenyAllPasswordIdentityProvider provides no identities for users
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type DenyAllPasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTPasswdPasswordIdentityProvider provides identities for users authenticating using htpasswd credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type HTPasswdPasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// file is a reference to your htpasswd file
	File string `json:"file"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LDAPPasswordIdentityProvider provides identities for users authenticating using LDAP credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type LDAPPasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`
	// url is an RFC 2255 URL which specifies the LDAP search parameters to use. The syntax of the URL is
	//    ldap://host:port/basedn?attribute?scope?filter
	URL string `json:"url"`
	// bindDN is an optional DN to bind with during the search phase.
	BindDN string `json:"bindDN"`
	// bindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource `json:"bindPassword"`

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool `json:"insecure"`
	// ca is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`
	// attributes maps LDAP attributes to identities
	Attributes LDAPAttributeMapping `json:"attributes"`
}

// LDAPAttributeMapping maps LDAP attributes to OpenShift identity fields
type LDAPAttributeMapping struct {
	// id is the list of attributes whose values should be used as the user ID. Required.
	// LDAP standard identity attribute is "dn"
	ID []string `json:"id"`
	// preferredUsername is the list of attributes whose values should be used as the preferred username.
	// LDAP standard login attribute is "uid"
	PreferredUsername []string `json:"preferredUsername"`
	// name is the list of attributes whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	// LDAP standard display name attribute is "cn"
	Name []string `json:"name"`
	// email is the list of attributes whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string `json:"email"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KeystonePasswordIdentityProvider provides identities for users authenticating using keystone password credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type KeystonePasswordIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`
	// RemoteConnectionInfo contains information about how to connect to the keystone server
	RemoteConnectionInfo `json:",inline"`
	// Domain Name is required for keystone v3
	DomainName string `json:"domainName"`
	// useKeystoneIdentity flag indicates that user should be authenticated by keystone ID, not by username
	UseKeystoneIdentity bool `json:"useKeystoneIdentity"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RequestHeaderIdentityProvider provides identities for users authenticating using request header credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type RequestHeaderIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// loginURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect interactive logins will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	LoginURL string `json:"loginURL"`

	// challengeURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect WWW-Authenticate challenges will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	ChallengeURL string `json:"challengeURL"`

	// clientCA is a file with the trusted signer certs.  If empty, no request verification is done, and any direct request to the OAuth server can impersonate any identity from this provider, merely by setting a request header.
	ClientCA string `json:"clientCA"`
	// clientCommonNames is an optional list of common names to require a match from. If empty, any client certificate validated against the clientCA bundle is considered authoritative.
	ClientCommonNames []string `json:"clientCommonNames"`

	// headers is the set of headers to check for identity information
	Headers []string `json:"headers"`
	// preferredUsernameHeaders is the set of headers to check for the preferred username
	PreferredUsernameHeaders []string `json:"preferredUsernameHeaders"`
	// nameHeaders is the set of headers to check for the display name
	NameHeaders []string `json:"nameHeaders"`
	// emailHeaders is the set of headers to check for the email address
	EmailHeaders []string `json:"emailHeaders"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubIdentityProvider provides identities for users authenticating using GitHub credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type GitHubIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// clientID is the oauth client ID
	ClientID string `json:"clientID"`
	// clientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`
	// organizations optionally restricts which organizations are allowed to log in
	Organizations []string `json:"organizations"`
	// teams optionally restricts which teams are allowed to log in. Format is <org>/<team>.
	Teams []string `json:"teams"`
	// hostname is the optional domain (e.g. "mycompany.com") for use with a hosted instance of GitHub Enterprise.
	// It must match the GitHub Enterprise settings value that is configured at /setup/settings#hostname.
	Hostname string `json:"hostname"`
	// ca is the optional trusted certificate authority bundle to use when making requests to the server.
	// If empty, the default system roots are used.  This can only be configured when hostname is set to a non-empty value.
	CA string `json:"ca"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabIdentityProvider provides identities for users authenticating using GitLab credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type GitLabIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// ca is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`
	// url is the oauth server base URL
	URL string `json:"url"`
	// clientID is the oauth client ID
	ClientID string `json:"clientID"`
	// clientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`
	// legacy determines if OAuth2 or OIDC should be used
	// If true, OAuth2 is used
	// If false, OIDC is used
	// If nil and the URL's host is gitlab.com, OIDC is used
	// Otherwise, OAuth2 is used
	// In a future release, nil will default to using OIDC
	// Eventually this flag will be removed and only OIDC will be used
	Legacy *bool `json:"legacy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GoogleIdentityProvider provides identities for users authenticating using Google credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type GoogleIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// clientID is the oauth client ID
	ClientID string `json:"clientID"`
	// clientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`

	// hostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string `json:"hostedDomain"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenIDIdentityProvider provides identities for users authenticating using OpenID credentials
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type OpenIDIdentityProvider struct {
	metav1.TypeMeta `json:",inline"`

	// ca is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`

	// clientID is the oauth client ID
	ClientID string `json:"clientID"`
	// clientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`

	// extraScopes are any scopes to request in addition to the standard "openid" scope.
	ExtraScopes []string `json:"extraScopes"`

	// extraAuthorizeParameters are any custom parameters to add to the authorize request.
	ExtraAuthorizeParameters map[string]string `json:"extraAuthorizeParameters"`

	// urls to use to authenticate
	URLs OpenIDURLs `json:"urls"`

	// claims mappings
	Claims OpenIDClaims `json:"claims"`
}

// OpenIDURLs are URLs to use when authenticating with an OpenID identity provider
type OpenIDURLs struct {
	// authorize is the oauth authorization URL
	Authorize string `json:"authorize"`
	// token is the oauth token granting URL
	Token string `json:"token"`
	// userInfo is the optional userinfo URL.
	// If present, a granted access_token is used to request claims
	// If empty, a granted id_token is parsed for claims
	UserInfo string `json:"userInfo"`
}

// OpenIDClaims contains a list of OpenID claims to use when authenticating with an OpenID identity provider
type OpenIDClaims struct {
	// id is the list of claims whose values should be used as the user ID. Required.
	// OpenID standard identity claim is "sub"
	ID []string `json:"id"`
	// preferredUsername is the list of claims whose values should be used as the preferred username.
	// If unspecified, the preferred username is determined from the value of the id claim
	PreferredUsername []string `json:"preferredUsername"`
	// name is the list of claims whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	Name []string `json:"name"`
	// email is the list of claims whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string `json:"email"`
}

// GrantConfig holds the necessary configuration options for grant handlers
type GrantConfig struct {
	// method determines the default strategy to use when an OAuth client requests a grant.
	// This method will be used only if the specific OAuth client doesn't provide a strategy
	// of their own. Valid grant handling methods are:
	//  - auto:   always approves grant requests, useful for trusted clients
	//  - prompt: prompts the end user for approval of grant requests, useful for third-party clients
	//  - deny:   always denies grant requests, useful for black-listed clients
	Method GrantHandlerType `json:"method"`

	// serviceAccountMethod is used for determining client authorization for service account oauth client.
	// It must be either: deny, prompt
	ServiceAccountMethod GrantHandlerType `json:"serviceAccountMethod"`
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

// EtcdConfig holds the necessary configuration options for connecting with an etcd database
type EtcdConfig struct {
	// servingInfo describes how to start serving the etcd master
	ServingInfo ServingInfo `json:"servingInfo"`
	// address is the advertised host:port for client connections to etcd
	Address string `json:"address"`
	// peerServingInfo describes how to start serving the etcd peer
	PeerServingInfo ServingInfo `json:"peerServingInfo"`
	// peerAddress is the advertised host:port for peer connections to etcd
	PeerAddress string `json:"peerAddress"`

	// StorageDir is the path to the etcd storage directory
	StorageDir string `json:"storageDirectory"`
}

// KubernetesMasterConfig holds the necessary configuration options for the Kubernetes master
type KubernetesMasterConfig struct {
	// apiLevels is a list of API levels that should be enabled on startup: v1 as examples
	APILevels []string `json:"apiLevels"`
	// disabledAPIGroupVersions is a map of groups to the versions (or *) that should be disabled.
	DisabledAPIGroupVersions map[string][]string `json:"disabledAPIGroupVersions"`

	// masterIP is the public IP address of kubernetes stuff.  If empty, the first result from net.InterfaceAddrs will be used.
	MasterIP string `json:"masterIP"`
	// masterEndpointReconcileTTL sets the time to live in seconds of an endpoint record recorded by each master. The endpoints are checked
	// at an interval that is 2/3 of this value and this value defaults to 15s if unset. In very large clusters, this value may be increased to
	// reduce the possibility that the master endpoint record expires (due to other load on the etcd server) and causes masters to drop in and
	// out of the kubernetes service record. It is not recommended to set this value below 15s.
	MasterEndpointReconcileTTL int `json:"masterEndpointReconcileTTL"`
	// servicesSubnet is the subnet to use for assigning service IPs
	ServicesSubnet string `json:"servicesSubnet"`
	// servicesNodePortRange is the range to use for assigning service public ports on a host.
	ServicesNodePortRange string `json:"servicesNodePortRange"`

	// schedulerConfigFile points to a file that describes how to set up the scheduler. If empty, you get the default scheduling rules.
	SchedulerConfigFile string `json:"schedulerConfigFile"`

	// podEvictionTimeout controls grace period for deleting pods on failed nodes.
	// It takes valid time duration string. If empty, you get the default pod eviction timeout.
	PodEvictionTimeout string `json:"podEvictionTimeout"`
	// proxyClientInfo specifies the client cert/key to use when proxying to pods
	ProxyClientInfo CertInfo `json:"proxyClientInfo"`

	// apiServerArguments are key value pairs that will be passed directly to the Kube apiserver that match the apiservers's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	APIServerArguments ExtendedArguments `json:"apiServerArguments"`
	// controllerArguments are key value pairs that will be passed directly to the Kube controller manager that match the
	// controller manager's command line arguments.  These are not migrated, but if you reference a value that does not exist
	// the server will not start. These values may override other settings in KubernetesMasterConfig which may cause invalid
	// configurations.
	ControllerArguments ExtendedArguments `json:"controllerArguments"`
	// schedulerArguments are key value pairs that will be passed directly to the Kube scheduler that match the scheduler's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	SchedulerArguments ExtendedArguments `json:"schedulerArguments"`
}

// CertInfo relates a certificate with a private key
type CertInfo struct {
	// certFile is a file containing a PEM-encoded certificate
	CertFile string `json:"certFile"`
	// keyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	KeyFile string `json:"keyFile"`
}

// PodManifestConfig holds the necessary configuration options for using pod manifests
type PodManifestConfig struct {
	// path specifies the path for the pod manifest file or directory
	// If its a directory, its expected to contain on or more manifest files
	// This is used by the Kubelet to create pods on the node
	Path string `json:"path"`
	// fileCheckIntervalSeconds is the interval in seconds for checking the manifest file(s) for new data
	// The interval needs to be a positive value
	FileCheckIntervalSeconds int64 `json:"fileCheckIntervalSeconds"`
}

// StringSource allows specifying a string inline, or externally via env var or file.
// When it contains only a string value, it marshals to a simple JSON string.
type StringSource struct {
	// StringSourceSpec specifies the string value, or external location
	StringSourceSpec `json:",inline"`
}

// StringSourceSpec specifies a string value, or external location
type StringSourceSpec struct {
	// value specifies the cleartext value, or an encrypted value if keyFile is specified.
	Value string `json:"value"`

	// env specifies an envvar containing the cleartext value, or an encrypted value if the keyFile is specified.
	Env string `json:"env"`

	// file references a file containing the cleartext value, or an encrypted value if a keyFile is specified.
	File string `json:"file"`

	// keyFile references a file containing the key to use to decrypt the value.
	KeyFile string `json:"keyFile"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LDAPSyncConfig holds the necessary configuration options to define an LDAP group sync
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type LDAPSyncConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Host is the scheme, host and port of the LDAP server to connect to:
	// scheme://host:port
	URL string `json:"url"`
	// bindDN is an optional DN to bind to the LDAP server with
	BindDN string `json:"bindDN"`
	// bindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource `json:"bindPassword"`

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool `json:"insecure"`
	// ca is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`

	// LDAPGroupUIDToOpenShiftGroupNameMapping is an optional direct mapping of LDAP group UIDs to
	// OpenShift Group names
	LDAPGroupUIDToOpenShiftGroupNameMapping map[string]string `json:"groupUIDNameMapping"`

	// RFC2307Config holds the configuration for extracting data from an LDAP server set up in a fashion
	// similar to RFC2307: first-class group and user entries, with group membership determined by a
	// multi-valued attribute on the group entry listing its members
	RFC2307Config *RFC2307Config `json:"rfc2307,omitempty"`

	// ActiveDirectoryConfig holds the configuration for extracting data from an LDAP server set up in a
	// fashion similar to that used in Active Directory: first-class user entries, with group membership
	// determined by a multi-valued attribute on members listing groups they are a member of
	ActiveDirectoryConfig *ActiveDirectoryConfig `json:"activeDirectory,omitempty"`

	// AugmentedActiveDirectoryConfig holds the configuration for extracting data from an LDAP server
	// set up in a fashion similar to that used in Active Directory as described above, with one addition:
	// first-class group entries exist and are used to hold metadata but not group membership
	AugmentedActiveDirectoryConfig *AugmentedActiveDirectoryConfig `json:"augmentedActiveDirectory,omitempty"`
}

// RFC2307Config holds the necessary configuration options to define how an LDAP group sync interacts with an LDAP
// server using the RFC2307 schema
type RFC2307Config struct {
	// AllGroupsQuery holds the template for an LDAP query that returns group entries.
	AllGroupsQuery LDAPQuery `json:"groupsQuery"`

	// GroupUIDAttributes defines which attribute on an LDAP group entry will be interpreted as its unique identifier.
	// (ldapGroupUID)
	GroupUIDAttribute string `json:"groupUIDAttribute"`

	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
	// an OpenShift group
	GroupNameAttributes []string `json:"groupNameAttributes"`

	// groupMembershipAttributes defines which attributes on an LDAP group entry will be interpreted  as its members.
	// The values contained in those attributes must be queryable by your UserUIDAttribute
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`

	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery `json:"usersQuery"`

	// userUIDAttribute defines which attribute on an LDAP user entry will be interpreted as its unique identifier.
	// It must correspond to values that will be found from the GroupMembershipAttributes
	UserUIDAttribute string `json:"userUIDAttribute"`

	// userNameAttributes defines which attributes on an LDAP user entry will be used, in order, as its OpenShift user name.
	// The first attribute with a non-empty value is used. This should match your PreferredUsername setting for your LDAPPasswordIdentityProvider
	UserNameAttributes []string `json:"userNameAttributes"`

	// tolerateMemberNotFoundErrors determines the behavior of the LDAP sync job when missing user entries are
	// encountered. If 'true', an LDAP query for users that doesn't find any will be tolerated and an only
	// and error will be logged. If 'false', the LDAP sync job will fail if a query for users doesn't find
	// any. The default value is 'false'. Misconfigured LDAP sync jobs with this flag set to 'true' can cause
	// group membership to be removed, so it is recommended to use this flag with caution.
	TolerateMemberNotFoundErrors bool `json:"tolerateMemberNotFoundErrors"`

	// tolerateMemberOutOfScopeErrors determines the behavior of the LDAP sync job when out-of-scope user entries
	// are encountered. If 'true', an LDAP query for a user that falls outside of the base DN given for the all
	// user query will be tolerated and only an error will be logged. If 'false', the LDAP sync job will fail
	// if a user query would search outside of the base DN specified by the all user query. Misconfigured LDAP
	// sync jobs with this flag set to 'true' can result in groups missing users, so it is recommended to use
	// this flag with caution.
	TolerateMemberOutOfScopeErrors bool `json:"tolerateMemberOutOfScopeErrors"`
}

// ActiveDirectoryConfig holds the necessary configuration options to define how an LDAP group sync interacts with an LDAP
// server using the Active Directory schema
type ActiveDirectoryConfig struct {
	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery `json:"usersQuery"`

	// userNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string `json:"userNameAttributes"`

	// groupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`
}

// AugmentedActiveDirectoryConfig holds the necessary configuration options to define how an LDAP group sync interacts with an LDAP
// server using the augmented Active Directory schema
type AugmentedActiveDirectoryConfig struct {
	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery `json:"usersQuery"`

	// userNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string `json:"userNameAttributes"`

	// groupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`

	// AllGroupsQuery holds the template for an LDAP query that returns group entries.
	AllGroupsQuery LDAPQuery `json:"groupsQuery"`

	// GroupUIDAttributes defines which attribute on an LDAP group entry will be interpreted as its unique identifier.
	// (ldapGroupUID)
	GroupUIDAttribute string `json:"groupUIDAttribute"`

	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
	// an OpenShift group
	GroupNameAttributes []string `json:"groupNameAttributes"`
}

// LDAPQuery holds the options necessary to build an LDAP query
type LDAPQuery struct {
	// The DN of the branch of the directory where all searches should start from
	BaseDN string `json:"baseDN"`

	// The (optional) scope of the search. Can be:
	// base: only the base object,
	// one:  all object on the base level,
	// sub:  the entire subtree
	// Defaults to the entire subtree if not set
	Scope string `json:"scope"`

	// The (optional) behavior of the search with regards to alisases. Can be:
	// never:  never dereference aliases,
	// search: only dereference in searching,
	// base:   only dereference in finding the base object,
	// always: always dereference
	// Defaults to always dereferencing if not set
	DerefAliases string `json:"derefAliases"`

	// TimeLimit holds the limit of time in seconds that any request to the server can remain outstanding
	// before the wait for a response is given up. If this is 0, no client-side limit is imposed
	TimeLimit int `json:"timeout"`

	// filter is a valid LDAP search filter that retrieves all relevant entries from the LDAP server with the base DN
	Filter string `json:"filter"`

	// pageSize is the maximum preferred page size, measured in LDAP entries. A page size of 0 means no paging will be done.
	PageSize int `json:"pageSize"`
}

// AdmissionPluginConfig holds the necessary configuration options for admission plugins
type AdmissionPluginConfig struct {
	// location is the path to a configuration file that contains the plugin's
	// configuration
	Location string `json:"location"`

	// configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.RawExtension `json:"configuration"`
}

// AdmissionConfig holds the necessary configuration options for admission
type AdmissionConfig struct {
	// pluginConfig allows specifying a configuration file per admission control plugin
	PluginConfig map[string]*AdmissionPluginConfig `json:"pluginConfig"`

	// pluginOrderOverride is a list of admission control plugin names that will be installed
	// on the master. Order is significant. If empty, a default list of plugins is used.
	PluginOrderOverride []string `json:"pluginOrderOverride,omitempty"`
}

// ControllerConfig holds configuration values for controllers
type ControllerConfig struct {
	// controllers is a list of controllers to enable.  '*' enables all on-by-default controllers, 'foo' enables the controller "+
	// named 'foo', '-foo' disables the controller named 'foo'.
	// Defaults to "*".
	Controllers []string `json:"controllers"`
	// election defines the configuration for electing a controller instance to make changes to
	// the cluster. If unspecified, the ControllerTTL value is checked to determine whether the
	// legacy direct etcd election code will be used.
	Election *ControllerElectionConfig `json:"election"`
	// serviceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
	// pods fulfilling a service to serve with.
	ServiceServingCert ServiceServingCert `json:"serviceServingCert"`
}

// ControllerElectionConfig contains configuration values for deciding how a controller
// will be elected to act as leader.
type ControllerElectionConfig struct {
	// lockName is the resource name used to act as the lock for determining which controller
	// instance should lead.
	LockName string `json:"lockName"`
	// lockNamespace is the resource namespace used to act as the lock for determining which
	// controller instance should lead. It defaults to "kube-system"
	LockNamespace string `json:"lockNamespace"`
	// lockResource is the group and resource name to use to coordinate for the controller lock.
	// If unset, defaults to "configmaps".
	LockResource GroupResource `json:"lockResource"`
}

// GroupResource points to a resource by its name and API group.
type GroupResource struct {
	// group is the name of an API group
	Group string `json:"group"`
	// resource is the name of a resource.
	Resource string `json:"resource"`
}

// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
// pods fulfilling a service to serve with.
type ServiceServingCert struct {
	// signer holds the signing information used to automatically sign serving certificates.
	// If this value is nil, then certs are not signed automatically.
	Signer *CertInfo `json:"signer"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DefaultAdmissionConfig can be used to enable or disable various admission plugins.
// When this type is present as the `configuration` object under `pluginConfig` and *if* the admission plugin supports it,
// this will cause an "off by default" admission plugin to be enabled
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type DefaultAdmissionConfig struct {
	metav1.TypeMeta `json:",inline"`

	// disable turns off an admission plugin that is enabled by default.
	Disable bool `json:"disable"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildDefaultsConfig controls the default information for Builds
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type BuildDefaultsConfig struct {
	metav1.TypeMeta `json:",inline"`

	// gitHTTPProxy is the location of the HTTPProxy for Git source
	GitHTTPProxy string `json:"gitHTTPProxy,omitempty"`

	// gitHTTPSProxy is the location of the HTTPSProxy for Git source
	GitHTTPSProxy string `json:"gitHTTPSProxy,omitempty"`

	// gitNoProxy is the list of domains for which the proxy should not be used
	GitNoProxy string `json:"gitNoProxy,omitempty"`

	// env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	Env []corev1.EnvVar `json:"env,omitempty"`

	// sourceStrategyDefaults are default values that apply to builds using the
	// source strategy.
	SourceStrategyDefaults *SourceStrategyDefaultsConfig `json:"sourceStrategyDefaults,omitempty"`

	// imageLabels is a list of labels that are applied to the resulting image.
	// User can override a default label by providing a label with the same name in their
	// Build/BuildConfig.
	ImageLabels []buildv1.ImageLabel `json:"imageLabels,omitempty"`

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string `json:"annotations,omitempty"`

	// resources defines resource requirements to execute the build.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// SourceStrategyDefaultsConfig contains values that apply to builds using the
// source strategy.
type SourceStrategyDefaultsConfig struct {
	// incremental indicates if s2i build strategies should perform an incremental
	// build or not
	Incremental *bool `json:"incremental,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildOverridesConfig controls override settings for builds
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +openshift:compatibility-gen:internal
type BuildOverridesConfig struct {
	metav1.TypeMeta `json:",inline"`

	// forcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool `json:"forcePull"`

	// imageLabels is a list of labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	ImageLabels []buildv1.ImageLabel `json:"imageLabels,omitempty"`

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string `json:"annotations,omitempty"`

	// tolerations is a list of Tolerations that will override any existing
	// tolerations set on a build pod.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}
