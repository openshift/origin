package v1

import (
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

type ExtendedArguments map[string][]string

// NodeConfig is the fully specified config starting an OpenShift node
type NodeConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// NodeName is the value used to identify this particular node in the cluster.  If possible, this should be your fully qualified hostname.
	// If you're describing a set of static nodes to the master, this value must match one of the values in the list
	NodeName string `json:"nodeName"`

	// Node may have multiple IPs, specify the IP to use for pod traffic routing
	// If not specified, network parse/lookup on the nodeName is performed and the first non-loopback address is used
	NodeIP string `json:"nodeIP"`

	// ServingInfo describes how to start serving
	ServingInfo ServingInfo `json:"servingInfo"`

	// MasterKubeConfig is a filename for the .kubeconfig file that describes how to connect this node to the master
	MasterKubeConfig string `json:"masterKubeConfig"`

	// MasterClientConnectionOverrides provides overrides to the client connection used to connect to the master.
	MasterClientConnectionOverrides *ClientConnectionOverrides `json:"masterClientConnectionOverrides"`

	// DNSDomain holds the domain suffix
	DNSDomain string `json:"dnsDomain"`

	// DNSIP holds the IP
	DNSIP string `json:"dnsIP"`

	// Deprecated and maintained for backward compatibility, use NetworkConfig.NetworkPluginName instead
	DeprecatedNetworkPluginName string `json:"networkPluginName,omitempty"`

	// NetworkConfig provides network options for the node
	NetworkConfig NodeNetworkConfig `json:"networkConfig"`

	// VolumeDirectory is the directory that volumes will be stored under
	VolumeDirectory string `json:"volumeDirectory"`

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig `json:"imageConfig"`

	// AllowDisabledDocker if true, the Kubelet will ignore errors from Docker.  This means that a node can start on a machine that doesn't have docker started.
	AllowDisabledDocker bool `json:"allowDisabledDocker"`

	// PodManifestConfig holds the configuration for enabling the Kubelet to
	// create pods based from a manifest file(s) placed locally on the node
	PodManifestConfig *PodManifestConfig `json:"podManifestConfig"`

	// AuthConfig holds authn/authz configuration options
	AuthConfig NodeAuthConfig `json:"authConfig"`

	// DockerConfig holds Docker related configuration options.
	DockerConfig DockerConfig `json:"dockerConfig"`

	// KubeletArguments are key value pairs that will be passed directly to the Kubelet that match the Kubelet's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	KubeletArguments ExtendedArguments `json:"kubeletArguments,omitempty"`

	// ProxyArguments are key value pairs that will be passed directly to the Proxy that match the Proxy's
	// command line arguments.  These are not migrated or validated, so if you use them they may become invalid.
	// These values override other settings in NodeConfig which may cause invalid configurations.
	ProxyArguments ExtendedArguments `json:"proxyArguments,omitempty"`

	// IPTablesSyncPeriod is how often iptable rules are refreshed
	IPTablesSyncPeriod string `json:"iptablesSyncPeriod"`

	// EnableUnidling controls whether or not the hybrid unidling proxy will be set up
	EnableUnidling *bool `json:"enableUnidling"`

	// VolumeConfig contains options for configuring volumes on the node.
	VolumeConfig NodeVolumeConfig `json:"volumeConfig"`
}

// NodeVolumeConfig contains options for configuring volumes on the node.
type NodeVolumeConfig struct {
	// LocalQuota contains options for controlling local volume quota on the node.
	LocalQuota LocalQuota `json:"localQuota"`
}

// MasterVolumeConfig contains options for configuring volume plugins in the master node.
type MasterVolumeConfig struct {
	// DynamicProvisioningEnabled is a boolean that toggles dynamic provisioning off when false, defaults to true
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
	// AuthenticationCacheTTL indicates how long an authentication result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthenticationCacheTTL string `json:"authenticationCacheTTL"`

	// AuthenticationCacheSize indicates how many authentication results should be cached. If 0, the default cache size is used.
	AuthenticationCacheSize int `json:"authenticationCacheSize"`

	// AuthorizationCacheTTL indicates how long an authorization result should be cached.
	// It takes a valid time duration string (e.g. "5m"). If empty, you get the default timeout. If zero (e.g. "0m"), caching is disabled
	AuthorizationCacheTTL string `json:"authorizationCacheTTL"`

	// AuthorizationCacheSize indicates how many authorization results should be cached. If 0, the default cache size is used.
	AuthorizationCacheSize int `json:"authorizationCacheSize"`
}

// NodeNetworkConfig provides network options for the node
type NodeNetworkConfig struct {
	// NetworkPluginName is a string specifying the networking plugin
	NetworkPluginName string `json:"networkPluginName"`
	// Maximum transmission unit for the network packets
	MTU uint32 `json:"mtu"`
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
	// DockerExecHandlerNsenter uses nsenter for executing commands in containers.
	DockerExecHandlerNsenter DockerExecHandlerType = "nsenter"

	// ControllersDisabled indicates no controllers should be enabled.
	ControllersDisabled = "none"
	// ControllersAll indicates all controllers should be started.
	ControllersAll = "*"
)

// FeatureList contains a set of features
type FeatureList []string

// MasterConfig holds the necessary configuration options for the OpenShift master
type MasterConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ServingInfo describes how to start serving
	ServingInfo HTTPServingInfo `json:"servingInfo"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins"`

	// APILevels is a list of API levels that should be enabled on startup: v1 as examples
	APILevels []string `json:"apiLevels"`

	// MasterPublicURL is how clients can access the OpenShift API server
	MasterPublicURL string `json:"masterPublicURL"`

	// Controllers is a list of the controllers that should be started. If set to "none", no controllers
	// will start automatically. The default value is "*" which will start all controllers. When
	// using "*", you may exclude controllers by prepending a "-" in front of their name. No other
	// values are recognized at this time.
	Controllers string `json:"controllers"`
	// PauseControllers instructs the master to not automatically start controllers, but instead
	// to wait until a notification to the server is received before launching them.
	PauseControllers bool `json:"pauseControllers"`
	// ControllerLeaseTTL enables controller election, instructing the master to attempt to acquire
	// a lease before controllers start and renewing it within a number of seconds defined by this value.
	// Setting this value non-negative forces pauseControllers=true. This value defaults off (0, or
	// omitted) and controller election can be disabled with -1.
	ControllerLeaseTTL int `json:"controllerLeaseTTL"`

	// AdmissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig `json:"admissionConfig"`

	// ControllerConfig holds configuration values for controllers
	ControllerConfig ControllerConfig `json:"controllerConfig"`

	// DisabledFeatures is a list of features that should not be started.  We
	// omitempty here because its very unlikely that anyone will want to
	// manually disable features and we don't want to encourage it.
	DisabledFeatures FeatureList `json:"disabledFeatures"`

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

	// ImageConfig holds options that describe how to build image names for system components
	ImageConfig ImageConfig `json:"imageConfig"`

	// ImagePolicyConfig controls limits and behavior for importing images
	ImagePolicyConfig ImagePolicyConfig `json:"imagePolicyConfig"`

	// PolicyConfig holds information about where to locate critical pieces of bootstrapping policy
	PolicyConfig PolicyConfig `json:"policyConfig"`

	// ProjectConfig holds information about project creation and defaults
	ProjectConfig ProjectConfig `json:"projectConfig"`

	// RoutingConfig holds information about routing and route generation
	RoutingConfig RoutingConfig `json:"routingConfig"`

	// NetworkConfig to be passed to the compiled in network plugin
	NetworkConfig MasterNetworkConfig `json:"networkConfig"`

	// MasterVolumeConfig contains options for configuring volume plugins in the master node.
	VolumeConfig MasterVolumeConfig `json:"volumeConfig"`

	// JenkinsPipelineConfig holds information about the default Jenkins template
	// used for JenkinsPipeline build strategy.
	JenkinsPipelineConfig JenkinsPipelineConfig `json:"jenkinsPipelineConfig"`

	// AuditConfig holds information related to auditing capabilities.
	AuditConfig AuditConfig `json:"auditConfig"`
}

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
}

// JenkinsPipelineConfig holds configuration for the Jenkins pipeline strategy
type JenkinsPipelineConfig struct {
	// AutoProvisionEnabled determines whether a Jenkins server will be spawned from the provided
	// template when the first build config in the project with type JenkinsPipeline
	// is created. When not specified this option defaults to true.
	AutoProvisionEnabled *bool `json:"autoProvisionEnabled"`
	// TemplateNamespace contains the namespace name where the Jenkins template is stored
	TemplateNamespace string `json:"templateNamespace"`
	// TemplateName is the name of the default Jenkins template
	TemplateName string `json:"templateName"`
	// ServiceName is the name of the Jenkins service OpenShift uses to detect
	// whether a Jenkins pipeline handler has already been installed in a project.
	// This value *must* match a service name in the provided template.
	ServiceName string `json:"serviceName"`
	// Parameters specifies a set of optional parameters to the Jenkins template.
	Parameters map[string]string `json:"parameters"`
}

// ImagePolicyConfig holds the necessary configuration options for limits and behavior for importing images
type ImagePolicyConfig struct {
	// MaxImagesBulkImportedPerRepository controls the number of images that are imported when a user
	// does a bulk import of a Docker repository. This number defaults to 5 to prevent users from
	// importing large numbers of images accidentally. Set -1 for no limit.
	MaxImagesBulkImportedPerRepository int `json:"maxImagesBulkImportedPerRepository"`
	// DisableScheduledImport allows scheduled background import of images to be disabled.
	DisableScheduledImport bool `json:"disableScheduledImport"`
	// ScheduledImageImportMinimumIntervalSeconds is the minimum number of seconds that can elapse between when image streams
	// scheduled for background import are checked against the upstream repository. The default value is 15 minutes.
	ScheduledImageImportMinimumIntervalSeconds int `json:"scheduledImageImportMinimumIntervalSeconds"`
	// MaxScheduledImageImportsPerMinute is the maximum number of scheduled image streams that will be imported in the
	// background per minute. The default value is 60. Set to -1 for unlimited.
	MaxScheduledImageImportsPerMinute int `json:"maxScheduledImageImportsPerMinute"`
}

//  holds the necessary configuration options for
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

// SecurityAllocator controls the automatic allocation of UIDs and MCS labels to a project. If nil, allocation is disabled.
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

//  holds the necessary configuration options for
type PolicyConfig struct {
	// BootstrapPolicyFile points to a template that contains roles and rolebindings that will be created if no policy object exists in the master namespace
	BootstrapPolicyFile string `json:"bootstrapPolicyFile"`

	// OpenShiftSharedResourcesNamespace is the namespace where shared OpenShift resources live (like shared templates)
	OpenShiftSharedResourcesNamespace string `json:"openshiftSharedResourcesNamespace"`

	// OpenShiftInfrastructureNamespace is the namespace where OpenShift infrastructure resources live (like controller service accounts)
	OpenShiftInfrastructureNamespace string `json:"openshiftInfrastructureNamespace"`

	// UserAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
	UserAgentMatchingConfig UserAgentMatchingConfig `json:"userAgentMatchingConfig"`
}

// UserAgentMatchingConfig controls how API calls from *voluntarily* identifying clients will be handled.  THIS DOES NOT DEFEND AGAINST MALICIOUS CLIENTS!
type UserAgentMatchingConfig struct {
	// If this list is non-empty, then a User-Agent must match one of the UserAgentRegexes to be allowed
	RequiredClients []UserAgentMatchRule `json:"requiredClients"`

	// If this list is non-empty, then a User-Agent must not match any of the UserAgentRegexes
	DeniedClients []UserAgentDenyRule `json:"deniedClients"`

	// DefaultRejectionMessage is the message shown when rejecting a client.  If it is not a set, a generic message is given.
	DefaultRejectionMessage string `json:"defaultRejectionMessage"`
}

// UserAgentMatchRule describes how to match a given request based on User-Agent and HTTPVerb
type UserAgentMatchRule struct {
	// UserAgentRegex is a regex that is checked against the User-Agent.
	// Known variants of oc clients
	// 1. oc accessing kube resources: oc/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 2. oc accessing openshift resources: oc/v1.1.3 (linux/amd64) openshift/b348c2f
	// 3. openshift kubectl accessing kube resources:  openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 4. openshit kubectl accessing openshift resources: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
	// 5. oadm accessing kube resources: oadm/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 6. oadm accessing openshift resources: oadm/v1.1.3 (linux/amd64) openshift/b348c2f
	// 7. openshift cli accessing kube resources: openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
	// 8. openshift cli accessing openshift resources: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
	Regex string `json:"regex"`

	// HTTPVerbs specifies which HTTP verbs should be matched.  An empty list means "match all verbs".
	HTTPVerbs []string `json:"httpVerbs"`
}

// UserAgentDenyRule adds a rejection message that can be used to help a user figure out how to get an approved client
type UserAgentDenyRule struct {
	UserAgentMatchRule `json:", inline"`

	// RejectionMessage is the message shown when rejecting a client.  If it is not a set, the default message is used.
	RejectionMessage string `json:"rejectionMessage"`
}

// RoutingConfig holds the necessary configuration options for routing to subdomains
type RoutingConfig struct {
	// Subdomain is the suffix appended to $service.$namespace. to form the default route hostname
	// DEPRECATED: This field is being replaced by routers setting their own defaults. This is the
	// "default" route.
	Subdomain string `json:"subdomain"`
}

// MasterNetworkConfig to be passed to the compiled in network plugin
type MasterNetworkConfig struct {
	// NetworkPluginName is the name of the network plugin to use
	NetworkPluginName string `json:"networkPluginName"`
	// ClusterNetworkCIDR is the CIDR string to specify the global overlay network's L3 space
	ClusterNetworkCIDR string `json:"clusterNetworkCIDR"`
	// HostSubnetLength is the number of bits to allocate to each host's subnet e.g. 8 would mean a /24 network on the host
	HostSubnetLength uint32 `json:"hostSubnetLength"`
	// ServiceNetwork is the CIDR string to specify the service networks
	ServiceNetworkCIDR string `json:"serviceNetworkCIDR"`
	// ExternalIPNetworkCIDRs controls what values are acceptable for the service external IP field. If empty, no externalIP
	// may be set. It may contain a list of CIDRs which are checked for access. If a CIDR is prefixed with !, IPs in that
	// CIDR will be rejected. Rejections will be applied first, then the IP checked against one of the allowed CIDRs. You
	// should ensure this range does not overlap with your nodes, pods, or service CIDRs for security reasons.
	ExternalIPNetworkCIDRs []string `json:"externalIPNetworkCIDRs"`
	// IngressIPNetworkCIDR controls the range to assign ingress ips from for services of type LoadBalancer on bare
	// metal. If empty, ingress ips will not be assigned. It may contain a single CIDR that will be allocated from.
	// For security reasons, you should ensure that this range does not overlap with the CIDRs reserved for external ips,
	// nodes, pods, or services.
	IngressIPNetworkCIDR string `json:"ingressIPNetworkCIDR"`
}

// ImageConfig holds the necessary configuration options for building image names for system components
type ImageConfig struct {
	// Format is the format of the name to be built for the system component
	Format string `json:"format"`
	// Latest determines if the latest tag will be pulled from the registry
	Latest bool `json:"latest"`
}

// RemoteConnectionInfo holds information necessary for establishing a remote connection
type RemoteConnectionInfo struct {
	// URL is the remote URL to connect to
	URL string `json:"url"`
	// CA is the CA for verifying TLS connections
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information to present
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// KubeletConnectionInfo holds information necessary for connecting to a kubelet
type KubeletConnectionInfo struct {
	// Port is the port to connect to kubelets on
	Port uint `json:"port"`
	// CA is the CA for verifying TLS connections to kubelets
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to kubelets
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// EtcdConnectionInfo holds information necessary for connecting to an etcd server
type EtcdConnectionInfo struct {
	// URLs are the URLs for etcd
	URLs []string `json:"urls"`
	// CA is a file containing trusted roots for the etcd server certificates
	CA string `json:"ca"`
	// CertInfo is the TLS client cert information for securing communication to etcd
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
}

// EtcdStorageConfig holds the necessary configuration options for the etcd storage underlying OpenShift and Kubernetes
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

// ServingInfo holds information about serving web pages
type ServingInfo struct {
	// BindAddress is the ip:port to serve on
	BindAddress string `json:"bindAddress"`
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string `json:"bindNetwork"`
	// CertInfo is the TLS cert info for serving secure traffic.
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline"`
	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string `json:"clientCA"`
	// NamedCertificates is a list of certificates to use to secure requests to specific hostnames
	NamedCertificates []NamedCertificate `json:"namedCertificates"`
}

// NamedCertificate specifies a certificate/key, and the names it should be served for
type NamedCertificate struct {
	// Names is a list of DNS names this certificate should be used to secure
	// A name can be a normal DNS name, or can contain leading wildcard segments.
	Names []string `json:"names"`
	// CertInfo is the TLS cert info for serving secure traffic
	CertInfo `json:",inline"`
}

// HTTPServingInfo holds configuration for serving HTTP
type HTTPServingInfo struct {
	// ServingInfo is the HTTP serving information
	ServingInfo `json:",inline"`
	// MaxRequestsInFlight is the number of concurrent requests allowed to the server. If zero, no limit.
	MaxRequestsInFlight int `json:"maxRequestsInFlight"`
	// RequestTimeoutSeconds is the number of seconds before requests are timed out. The default is 60 minutes, if
	// -1 there is no limit on requests.
	RequestTimeoutSeconds int `json:"requestTimeoutSeconds"`
}

// MasterClients holds references to `.kubeconfig` files that qualify master clients for OpenShift and Kubernetes
type MasterClients struct {
	// OpenShiftLoopbackKubeConfig is a .kubeconfig filename for system components to loopback to this master
	OpenShiftLoopbackKubeConfig string `json:"openshiftLoopbackKubeConfig"`
	// ExternalKubernetesKubeConfig is a .kubeconfig filename for proxying to Kubernetes
	ExternalKubernetesKubeConfig string `json:"externalKubernetesKubeConfig"`

	// OpenShiftLoopbackClientConnectionOverrides specifies client overrides for system components to loop back to this master.
	OpenShiftLoopbackClientConnectionOverrides *ClientConnectionOverrides `json:"openshiftLoopbackClientConnectionOverrides"`
	// ExternalKubernetesClientConnectionOverrides specifies client overrides for proxying to Kubernetes.
	ExternalKubernetesClientConnectionOverrides *ClientConnectionOverrides `json:"externalKubernetesClientConnectionOverrides"`
}

// ClientConnectionOverrides are a set of overrides to the default client connection settings.
type ClientConnectionOverrides struct {
	// AcceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string `json:"acceptContentTypes"`
	// ContentType is the content type used when sending data to the server from this client.
	ContentType string `json:"contentType"`

	// QPS controls the number of queries per second allowed for this connection.
	QPS float32 `json:"qps"`
	// Burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32 `json:"burst"`
}

// DNSConfig holds the necessary configuration options for DNS
type DNSConfig struct {
	// BindAddress is the ip:port to serve DNS on
	BindAddress string `json:"bindAddress"`
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string `json:"bindNetwork"`
	// AllowRecursiveQueries allows the DNS server on the master to answer queries recursively. Note that open
	// resolvers can be used for DNS amplification attacks and the master DNS should not be made accessible
	// to public networks.
	AllowRecursiveQueries bool `json:"allowRecursiveQueries"`
}

// AssetConfig holds the necessary configuration options for serving assets
type AssetConfig struct {
	// ServingInfo is the HTTP serving information for these assets
	ServingInfo HTTPServingInfo `json:"servingInfo"`

	// PublicURL is where you can find the asset server (TODO do we really need this?)
	PublicURL string `json:"publicURL"`

	// LogoutURL is an optional, absolute URL to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURL string `json:"logoutURL"`

	// MasterPublicURL is how the web console can access the OpenShift v1 server
	MasterPublicURL string `json:"masterPublicURL"`

	// LoggingPublicURL is the public endpoint for logging (optional)
	LoggingPublicURL string `json:"loggingPublicURL"`

	// MetricsPublicURL is the public endpoint for metrics (optional)
	MetricsPublicURL string `json:"metricsPublicURL"`

	// ExtensionScripts are file paths on the asset server files to load as scripts when the Web
	// Console loads
	ExtensionScripts []string `json:"extensionScripts"`

	// ExtensionProperties are key(string) and value(string) pairs that will be injected into the console under
	// the global variable OPENSHIFT_EXTENSION_PROPERTIES
	ExtensionProperties map[string]string `json:"extensionProperties"`

	// ExtensionStylesheets are file paths on the asset server files to load as stylesheets when
	// the Web Console loads
	ExtensionStylesheets []string `json:"extensionStylesheets"`

	// Extensions are files to serve from the asset server filesystem under a subcontext
	Extensions []AssetExtensionsConfig `json:"extensions"`

	// ExtensionDevelopment when true tells the asset server to reload extension scripts and
	// stylesheets for every request rather than only at startup. It lets you develop extensions
	// without having to restart the server for every change.
	ExtensionDevelopment bool `json:"extensionDevelopment"`
}

// OAuthConfig holds the necessary configuration options for OAuth authentication
type OAuthConfig struct {
	// MasterCA is the CA for verifying the TLS connection back to the MasterURL.
	MasterCA *string `json:"masterCA"`

	// MasterURL is used for making server-to-server calls to exchange authorization codes for access tokens
	MasterURL string `json:"masterURL"`

	// MasterPublicURL is used for building valid client redirect URLs for external access
	MasterPublicURL string `json:"masterPublicURL"`

	// AssetPublicURL is used for building valid client redirect URLs for external access
	AssetPublicURL string `json:"assetPublicURL"`

	// AlwaysShowProviderSelection will force the provider selection page to render even when there is only a single provider.
	AlwaysShowProviderSelection bool `json:"alwaysShowProviderSelection"`

	//IdentityProviders is an ordered list of ways for a user to identify themselves
	IdentityProviders []IdentityProvider `json:"identityProviders"`

	// GrantConfig describes how to handle grants
	GrantConfig GrantConfig `json:"grantConfig"`

	// SessionConfig hold information about configuring sessions.
	SessionConfig *SessionConfig `json:"sessionConfig"`

	// TokenConfig contains options for authorization and access tokens
	TokenConfig TokenConfig `json:"tokenConfig"`

	// Templates allow you to customize pages like the login page.
	Templates *OAuthTemplates `json:"templates"`
}

// OAuthTemplates allow for customization of pages like the login page
type OAuthTemplates struct {
	// Login is a path to a file containing a go template used to render the login page.
	// If unspecified, the default login page is used.
	Login string `json:"login"`

	// ProviderSelection is a path to a file containing a go template used to render the provider selection page.
	// If unspecified, the default provider selection page is used.
	ProviderSelection string `json:"providerSelection"`

	// Error is a path to a file containing a go template used to render error pages during the authentication or grant flow
	// If unspecified, the default error page is used.
	Error string `json:"error"`
}

// ServiceAccountConfig holds the necessary configuration options for a service account
type ServiceAccountConfig struct {
	// ManagedNames is a list of service account names that will be auto-created in every namespace.
	// If no names are specified, the ServiceAccountsController will not be started.
	ManagedNames []string `json:"managedNames"`

	// LimitSecretReferences controls whether or not to allow a service account to reference any secret in a namespace
	// without explicitly referencing them
	LimitSecretReferences bool `json:"limitSecretReferences"`

	// PrivateKeyFile is a file containing a PEM-encoded private RSA key, used to sign service account tokens.
	// If no private key is specified, the service account TokensController will not be started.
	PrivateKeyFile string `json:"privateKeyFile"`

	// PublicKeyFiles is a list of files, each containing a PEM-encoded public RSA key.
	// (If any file contains a private key, the public portion of the key is used)
	// The list of public keys is used to verify presented service account tokens.
	// Each key is tried in order until the list is exhausted or verification succeeds.
	// If no keys are specified, no service account authentication will be available.
	PublicKeyFiles []string `json:"publicKeyFiles"`

	// MasterCA is the CA for verifying the TLS connection back to the master.  The service account controller will automatically
	// inject the contents of this file into pods so they can verify connections to the master.
	MasterCA string `json:"masterCA"`
}

// TokenConfig holds the necessary configuration options for authorization and access tokens
type TokenConfig struct {
	// AuthorizeTokenMaxAgeSeconds defines the maximum age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32 `json:"authorizeTokenMaxAgeSeconds"`
	// AccessTokenMaxAgeSeconds defines the maximum age of access tokens
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
	unversioned.TypeMeta `json:",inline"`

	// Secrets is a list of secrets
	// New sessions are signed and encrypted using the first secret.
	// Existing sessions are decrypted/authenticated by each secret until one succeeds. This allows rotating secrets.
	Secrets []SessionSecret `json:"secrets"`
}

// SessionSecret is a secret used to authenticate/decrypt cookie-based sessions
type SessionSecret struct {
	// Authentication is used to authenticate sessions using HMAC. Recommended to use a secret with 32 or 64 bytes.
	Authentication string `json:"authentication"`
	// Encryption is used to encrypt sessions. Must be 16, 24, or 32 characters long, to select AES-128, AES-
	Encryption string `json:"encryption"`
}

// IdentityProvider provides identities for users authenticating using credentials
type IdentityProvider struct {
	// Name is used to qualify the identities returned by this provider
	Name string `json:"name"`
	// UseAsChallenger indicates whether to issue WWW-Authenticate challenges for this provider
	UseAsChallenger bool `json:"challenge"`
	// UseAsLogin indicates whether to use this identity provider for unauthenticated browsers to login against
	UseAsLogin bool `json:"login"`
	// MappingMethod determines how identities from this provider are mapped to users
	MappingMethod string `json:"mappingMethod"`
	// Provider contains the information about how to set up a specific identity provider
	Provider runtime.RawExtension `json:"provider"`
}

// BasicAuthPasswordIdentityProvider provides identities for users authenticating using HTTP basic auth credentials
type BasicAuthPasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// RemoteConnectionInfo contains information about how to connect to the external basic auth server
	RemoteConnectionInfo `json:",inline"`
}

// AllowAllPasswordIdentityProvider provides identities for users authenticating using non-empty passwords
type AllowAllPasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`
}

// DenyAllPasswordIdentityProvider provides no identities for users
type DenyAllPasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`
}

// HTPasswdPasswordIdentityProvider provides identities for users authenticating using htpasswd credentials
type HTPasswdPasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// File is a reference to your htpasswd file
	File string `json:"file"`
}

// LDAPPasswordIdentityProvider provides identities for users authenticating using LDAP credentials
type LDAPPasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`
	// URL is an RFC 2255 URL which specifies the LDAP search parameters to use. The syntax of the URL is
	//    ldap://host:port/basedn?attribute?scope?filter
	URL string `json:"url"`
	// BindDN is an optional DN to bind with during the search phase.
	BindDN string `json:"bindDN"`
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource `json:"bindPassword"`

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool `json:"insecure"`
	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`
	// Attributes maps LDAP attributes to identities
	Attributes LDAPAttributeMapping `json:"attributes"`
}

// LDAPAttributeMapping maps LDAP attributes to OpenShift identity fields
type LDAPAttributeMapping struct {
	// ID is the list of attributes whose values should be used as the user ID. Required.
	// LDAP standard identity attribute is "dn"
	ID []string `json:"id"`
	// PreferredUsername is the list of attributes whose values should be used as the preferred username.
	// LDAP standard login attribute is "uid"
	PreferredUsername []string `json:"preferredUsername"`
	// Name is the list of attributes whose values should be used as the display name. Optional.
	// If unspecified, no display name is set for the identity
	// LDAP standard display name attribute is "cn"
	Name []string `json:"name"`
	// Email is the list of attributes whose values should be used as the email address. Optional.
	// If unspecified, no email is set for the identity
	Email []string `json:"email"`
}

// KeystonePasswordIdentityProvider provides identities for users authenticating using keystone password credentials
type KeystonePasswordIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`
	// RemoteConnectionInfo contains information about how to connect to the keystone server
	RemoteConnectionInfo `json:",inline"`
	// Domain Name is required for keystone v3
	DomainName string `json:"domainName"`
}

// RequestHeaderIdentityProvider provides identities for users authenticating using request header credentials
type RequestHeaderIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// LoginURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect interactive logins will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	LoginURL string `json:"loginURL"`

	// ChallengeURL is a URL to redirect unauthenticated /authorize requests to
	// Unauthenticated requests from OAuth clients which expect WWW-Authenticate challenges will be redirected here
	// ${url} is replaced with the current URL, escaped to be safe in a query parameter
	//   https://www.example.com/sso-login?then=${url}
	// ${query} is replaced with the current query string
	//   https://www.example.com/auth-proxy/oauth/authorize?${query}
	ChallengeURL string `json:"challengeURL"`

	// ClientCA is a file with the trusted signer certs.  If empty, no request verification is done, and any direct request to the OAuth server can impersonate any identity from this provider, merely by setting a request header.
	ClientCA string `json:"clientCA"`
	// ClientCommonNames is an optional list of common names to require a match from. If empty, any client certificate validated against the clientCA bundle is considered authoritative.
	ClientCommonNames []string `json:"clientCommonNames"`

	// Headers is the set of headers to check for identity information
	Headers []string `json:"headers"`
	// PreferredUsernameHeaders is the set of headers to check for the preferred username
	PreferredUsernameHeaders []string `json:"preferredUsernameHeaders"`
	// NameHeaders is the set of headers to check for the display name
	NameHeaders []string `json:"nameHeaders"`
	// EmailHeaders is the set of headers to check for the email address
	EmailHeaders []string `json:"emailHeaders"`
}

// GitHubIdentityProvider provides identities for users authenticating using GitHub credentials
type GitHubIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`
	// Organizations optionally restricts which organizations are allowed to log in
	Organizations []string `json:"organizations"`
}

// GitLabIdentityProvider provides identities for users authenticating using GitLab credentials
type GitLabIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`
	// URL is the oauth server base URL
	URL string `json:"url"`
	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`
}

// GoogleIdentityProvider provides identities for users authenticating using Google credentials
type GoogleIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`

	// HostedDomain is the optional Google App domain (e.g. "mycompany.com") to restrict logins to
	HostedDomain string `json:"hostedDomain"`
}

// OpenIDIdentityProvider provides identities for users authenticating using OpenID credentials
type OpenIDIdentityProvider struct {
	unversioned.TypeMeta `json:",inline"`

	// CA is the optional trusted certificate authority bundle to use when making requests to the server
	// If empty, the default system roots are used
	CA string `json:"ca"`

	// ClientID is the oauth client ID
	ClientID string `json:"clientID"`
	// ClientSecret is the oauth client secret
	ClientSecret StringSource `json:"clientSecret"`

	// ExtraScopes are any scopes to request in addition to the standard "openid" scope.
	ExtraScopes []string `json:"extraScopes"`

	// ExtraAuthorizeParameters are any custom parameters to add to the authorize request.
	ExtraAuthorizeParameters map[string]string `json:"extraAuthorizeParameters"`

	// URLs to use to authenticate
	URLs OpenIDURLs `json:"urls"`

	// Claims mappings
	Claims OpenIDClaims `json:"claims"`
}

// OpenIDURLs are URLs to use when authenticating with an OpenID identity provider
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

// OpenIDClaims contains a list of OpenID claims to use when authenticating with an OpenID identity provider
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

// GrantConfig holds the necessary configuration options for grant handlers
type GrantConfig struct {
	// Method determines the default strategy to use when an OAuth client requests a grant.
	// This method will be used only if the specific OAuth client doesn't provide a strategy
	// of their own. Valid grant handling methods are:
	//  - auto:   always approves grant requests, useful for trusted clients
	//  - prompt: prompts the end user for approval of grant requests, useful for third-party clients
	//  - deny:   always denies grant requests, useful for black-listed clients
	Method GrantHandlerType `json:"method"`

	// ServiceAccountMethod is used for determining client authorization for service account oauth client.
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
	// ServingInfo describes how to start serving the etcd master
	ServingInfo ServingInfo `json:"servingInfo"`
	// Address is the advertised host:port for client connections to etcd
	Address string `json:"address"`
	// PeerServingInfo describes how to start serving the etcd peer
	PeerServingInfo ServingInfo `json:"peerServingInfo"`
	// PeerAddress is the advertised host:port for peer connections to etcd
	PeerAddress string `json:"peerAddress"`

	// StorageDir is the path to the etcd storage directory
	StorageDir string `json:"storageDirectory"`
}

// KubernetesMasterConfig holds the necessary configuration options for the Kubernetes master
type KubernetesMasterConfig struct {
	// APILevels is a list of API levels that should be enabled on startup: v1 as examples
	APILevels []string `json:"apiLevels"`
	// DisabledAPIGroupVersions is a map of groups to the versions (or *) that should be disabled.
	DisabledAPIGroupVersions map[string][]string `json:"disabledAPIGroupVersions"`

	// MasterIP is the public IP address of kubernetes stuff.  If empty, the first result from net.InterfaceAddrs will be used.
	MasterIP string `json:"masterIP"`
	// MasterCount is the number of expected masters that should be running. This value defaults to 1 and may be set to a positive integer,
	// or if set to -1, indicates this is part of a cluster.
	MasterCount int `json:"masterCount"`
	// ServicesSubnet is the subnet to use for assigning service IPs
	ServicesSubnet string `json:"servicesSubnet"`
	// ServicesNodePortRange is the range to use for assigning service public ports on a host.
	ServicesNodePortRange string `json:"servicesNodePortRange"`
	// StaticNodeNames is the list of nodes that are statically known
	StaticNodeNames []string `json:"staticNodeNames"`

	// SchedulerConfigFile points to a file that describes how to set up the scheduler. If empty, you get the default scheduling rules.
	SchedulerConfigFile string `json:"schedulerConfigFile"`

	// PodEvictionTimeout controls grace period for deleting pods on failed nodes.
	// It takes valid time duration string. If empty, you get the default pod eviction timeout.
	PodEvictionTimeout string `json:"podEvictionTimeout"`
	// ProxyClientInfo specifies the client cert/key to use when proxying to pods
	ProxyClientInfo CertInfo `json:"proxyClientInfo"`

	// AdmissionConfig contains admission control plugin configuration.
	AdmissionConfig AdmissionConfig `json:"admissionConfig"`

	// APIServerArguments are key value pairs that will be passed directly to the Kube apiserver that match the apiservers's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	APIServerArguments ExtendedArguments `json:"apiServerArguments"`
	// ControllerArguments are key value pairs that will be passed directly to the Kube controller manager that match the
	// controller manager's command line arguments.  These are not migrated, but if you reference a value that does not exist
	// the server will not start. These values may override other settings in KubernetesMasterConfig which may cause invalid
	// configurations.
	ControllerArguments ExtendedArguments `json:"controllerArguments"`
	// SchedulerArguments are key value pairs that will be passed directly to the Kube scheduler that match the scheduler's
	// command line arguments.  These are not migrated, but if you reference a value that does not exist the server will not
	// start. These values may override other settings in KubernetesMasterConfig which may cause invalid configurations.
	SchedulerArguments ExtendedArguments `json:"schedulerArguments"`
}

// CertInfo relates a certificate with a private key
type CertInfo struct {
	// CertFile is a file containing a PEM-encoded certificate
	CertFile string `json:"certFile"`
	// KeyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	KeyFile string `json:"keyFile"`
}

// PodManifestConfig holds the necessary configuration options for using pod manifests
type PodManifestConfig struct {
	// Path specifies the path for the pod manifest file or directory
	// If its a directory, its expected to contain on or more manifest files
	// This is used by the Kubelet to create pods on the node
	Path string `json:"path"`
	// FileCheckIntervalSeconds is the interval in seconds for checking the manifest file(s) for new data
	// The interval needs to be a positive value
	FileCheckIntervalSeconds int64 `json:"fileCheckIntervalSeconds"`
}

// AssetExtensionsConfig holds the necessary configuration options for asset extensions
type AssetExtensionsConfig struct {
	// SubContext is the path under /<context>/extensions/ to serve files from SourceDirectory
	Name string `json:"name"`
	// SourceDirectory is a directory on the asset server to serve files under Name in the Web
	// Console. It may have nested folders.
	SourceDirectory string `json:"sourceDirectory"`
	// HTML5Mode determines whether to redirect to the root index.html when a file is not found.
	// This is needed for apps that use the HTML5 history API like AngularJS apps with HTML5
	// mode enabled. If HTML5Mode is true, also rewrite the base element in index.html with the
	// Web Console's context root. Defaults to false.
	HTML5Mode bool `json:"html5Mode"`
}

// StringSource allows specifying a string inline, or externally via env var or file.
// When it contains only a string value, it marshals to a simple JSON string.
type StringSource struct {
	// StringSourceSpec specifies the string value, or external location
	StringSourceSpec `json:",inline"`
}

// StringSourceSpec specifies a string value, or external location
type StringSourceSpec struct {
	// Value specifies the cleartext value, or an encrypted value if keyFile is specified.
	Value string `json:"value"`

	// Env specifies an envvar containing the cleartext value, or an encrypted value if the keyFile is specified.
	Env string `json:"env"`

	// File references a file containing the cleartext value, or an encrypted value if a keyFile is specified.
	File string `json:"file"`

	// KeyFile references a file containing the key to use to decrypt the value.
	KeyFile string `json:"keyFile"`
}

// LDAPSyncConfig holds the necessary configuration options to define an LDAP group sync
type LDAPSyncConfig struct {
	unversioned.TypeMeta `json:",inline"`
	// Host is the scheme, host and port of the LDAP server to connect to:
	// scheme://host:port
	URL string `json:"url"`
	// BindDN is an optional DN to bind to the LDAP server with
	BindDN string `json:"bindDN"`
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword StringSource `json:"bindPassword"`

	// Insecure, if true, indicates the connection should not use TLS.
	// Cannot be set to true with a URL scheme of "ldaps://"
	// If false, "ldaps://" URLs connect using TLS, and "ldap://" URLs are upgraded to a TLS connection using StartTLS as specified in https://tools.ietf.org/html/rfc2830
	Insecure bool `json:"insecure"`
	// CA is the optional trusted certificate authority bundle to use when making requests to the server
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

	// GroupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
	// an OpenShift group
	GroupNameAttributes []string `json:"groupNameAttributes"`

	// GroupMembershipAttributes defines which attributes on an LDAP group entry will be interpreted  as its members.
	// The values contained in those attributes must be queryable by your UserUIDAttribute
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`

	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery `json:"usersQuery"`

	// UserUIDAttribute defines which attribute on an LDAP user entry will be interpreted as its unique identifier.
	// It must correspond to values that will be found from the GroupMembershipAttributes
	UserUIDAttribute string `json:"userUIDAttribute"`

	// UserNameAttributes defines which attributes on an LDAP user entry will be used, in order, as its OpenShift user name.
	// The first attribute with a non-empty value is used. This should match your PreferredUsername setting for your LDAPPasswordIdentityProvider
	UserNameAttributes []string `json:"userNameAttributes"`

	// TolerateMemberNotFoundErrors determines the behavior of the LDAP sync job when missing user entries are
	// encountered. If 'true', an LDAP query for users that doesn't find any will be tolerated and an only
	// and error will be logged. If 'false', the LDAP sync job will fail if a query for users doesn't find
	// any. The default value is 'false'. Misconfigured LDAP sync jobs with this flag set to 'true' can cause
	// group membership to be removed, so it is recommended to use this flag with caution.
	TolerateMemberNotFoundErrors bool `json:"tolerateMemberNotFoundErrors"`

	// TolerateMemberOutOfScopeErrors determines the behavior of the LDAP sync job when out-of-scope user entries
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

	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string `json:"userNameAttributes"`

	// GroupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`
}

// AugmentedActiveDirectoryConfig holds the necessary configuration options to define how an LDAP group sync interacts with an LDAP
// server using the augmented Active Directory schema
type AugmentedActiveDirectoryConfig struct {
	// AllUsersQuery holds the template for an LDAP query that returns user entries.
	AllUsersQuery LDAPQuery `json:"usersQuery"`

	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its OpenShift user name.
	UserNameAttributes []string `json:"userNameAttributes"`

	// GroupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	GroupMembershipAttributes []string `json:"groupMembershipAttributes"`

	// AllGroupsQuery holds the template for an LDAP query that returns group entries.
	AllGroupsQuery LDAPQuery `json:"groupsQuery"`

	// GroupUIDAttributes defines which attribute on an LDAP group entry will be interpreted as its unique identifier.
	// (ldapGroupUID)
	GroupUIDAttribute string `json:"groupUIDAttribute"`

	// GroupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for
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

	// Filter is a valid LDAP search filter that retrieves all relevant entries from the LDAP server with the base DN
	Filter string `json:"filter"`

	// PageSize is the maximum preferred page size, measured in LDAP entries. A page size of 0 means no paging will be done.
	PageSize int `json:"pageSize"`
}

// AdmissionPluginConfig holds the necessary configuration options for admission plugins
type AdmissionPluginConfig struct {
	// Location is the path to a configuration file that contains the plugin's
	// configuration
	Location string `json:"location"`

	// Configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.RawExtension `json:"configuration"`
}

// AdmissionConfig holds the necessary configuration options for admission
type AdmissionConfig struct {
	// PluginConfig allows specifying a configuration file per admission control plugin
	PluginConfig map[string]AdmissionPluginConfig `json:"pluginConfig"`

	// PluginOrderOverride is a list of admission control plugin names that will be installed
	// on the master. Order is significant. If empty, a default list of plugins is used.
	PluginOrderOverride []string `json:"pluginOrderOverride,omitempty"`
}

// ControllerConfig holds configuration values for controllers
type ControllerConfig struct {
	// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
	// pods fulfilling a service to serve with.
	ServiceServingCert ServiceServingCert `json:"serviceServingCert"`
}

// ServiceServingCert holds configuration for service serving cert signer which creates cert/key pairs for
// pods fulfilling a service to serve with.
type ServiceServingCert struct {
	// Signer holds the signing information used to automatically sign serving certificates.
	// If this value is nil, then certs are not signed automatically.
	Signer *CertInfo `json:"signer"`
}

// DefaultAdmissionConfig can be used to enable or disable various admission plugins.
// When this type is present as the `configuration` object under `pluginConfig` and *if* the admission plugin supports it,
// this will cause an "off by default" admission plugin to be enabled
type DefaultAdmissionConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// Disable turns off an admission plugin that is enabled by default.
	Disable bool `json:"disable"`
}
