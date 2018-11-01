package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Image holds cluster-wide information about how to handle images.  The canonical name is `cluster`
type Image struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec ImageSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status ImageStatus `json:"status"`
}

type ImageSpec struct {
	// AllowedRegistriesForImport limits the container image registries that normal users may import
	// images from. Set this list to the registries that you trust to contain valid Docker
	// images and that you want applications to be able to import from. Users with
	// permission to create Images or ImageStreamMappings via the API are not affected by
	// this policy - typically only administrators or system integrations will have those
	// permissions.
	AllowedRegistriesForImport []RegistryLocation `json:"allowedRegistriesForImport,omitempty"`

	// ExternalRegistryHostname sets the hostname for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	ExternalRegistryHostname string `json:"externalRegistryHostname,omitempty"`

	// AdditionalTrustedCA is a reference to a ConfigMap containing additional CAs that
	// should be trusted during imagestream import.
	AdditionalTrustedCA ConfigMapReference `json:"additionalTrustedCA,omitempty"`
}

type ImageStatus struct {

	// this value is set by the image registry operator which controls the internal registry hostname
	// InternalRegistryHostname sets the hostname for the default internal image
	// registry. The value must be in "hostname[:port]" format.
	// For backward compatibility, users can still use OPENSHIFT_DEFAULT_REGISTRY
	// environment variable but this setting overrides the environment variable.
	InternalRegistryHostname string `json:"internalRegistryHostname,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Image `json:"items"`
}

// RegistryLocation contains a location of the registry specified by the registry domain
// name. The domain name might include wildcards, like '*' or '??'.
type RegistryLocation struct {
	// DomainName specifies a domain name for the registry
	// In case the registry use non-standard (80 or 443) port, the port should be included
	// in the domain name as well.
	DomainName string `json:"domainName"`
	// Insecure indicates whether the registry is secure (https) or insecure (http)
	// By default (if not specified) the registry is assumed as secure.
	Insecure bool `json:"insecure,omitempty"`
}

// ConfigMapReference references the location of a configmap.
type ConfigMapReference struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Key allows pointing to a specific key/value inside of the configmap.  This is useful for logical file references.
	Key string `json:"filename,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Build holds cluster-wide information on how to handle builds. The canonical name is `cluster`
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec holds user-settable values for the build controller configuration
	// +optional
	Spec BuildSpec `json:"spec,omitempty"`
}

type BuildSpec struct {
	// AdditionalTrustedCA is a reference to a ConfigMap containing additional CAs that
	// should be trusted for image pushes and pulls during builds.
	// +optional
	AdditionalTrustedCA ConfigMapReference `json:"additionalTrustedCA,omitempty"`
	// BuildDefaults controls the default information for Builds
	// +optional
	BuildDefaults BuildDefaults `json:"buildDefaults,omitempty"`
	// BuildOverrides controls override settings for builds
	// +optional
	BuildOverrides BuildOverrides `json:"buildOverrides,omitempty"`
}

type BuildDefaults struct {
	// GitHTTPProxy is the location of the HTTPProxy for Git source
	// +optional
	GitHTTPProxy string `json:"gitHTTPProxy,omitempty"`

	// GitHTTPSProxy is the location of the HTTPSProxy for Git source
	// +optional
	GitHTTPSProxy string `json:"gitHTTPSProxy,omitempty"`

	// GitNoProxy is the list of domains for which the proxy should not be used
	// +optional
	GitNoProxy string `json:"gitNoProxy,omitempty"`

	// Env is a set of default environment variables that will be applied to the
	// build if the specified variables do not exist on the build
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// User can override a default label by providing a label with the same name in their
	// Build/BuildConfig.
	// +optional
	ImageLabels []ImageLabel `json:"imageLabels,omitempty"`

	// Resources defines resource requirements to execute the build.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ImageLabel struct {
	// Name defines the name of the label. It must have non-zero length.
	Name string `json:"name"`

	// Value defines the literal value of the label.
	// +optional
	Value string `json:"value,omitempty"`
}

type BuildOverrides struct {
	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	// +optional
	ImageLabels []ImageLabel `json:"imageLabels,omitempty"`

	// NodeSelector is a selector which must be true for the build pod to fit on a node
	// +optional
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// Tolerations is a list of Tolerations that will override any existing
	// tolerations set on a build pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BuildList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []Build `json:"items"`
}

// HTTPServingInfo holds configuration for serving HTTP
type HTTPServingInfo struct {
	// ServingInfo is the HTTP serving information
	ServingInfo `json:",inline"`
	// MaxRequestsInFlight is the number of concurrent requests allowed to the server. If zero, no limit.
	MaxRequestsInFlight int64 `json:"maxRequestsInFlight"`
	// RequestTimeoutSeconds is the number of seconds before requests are timed out. The default is 60 minutes, if
	// -1 there is no limit on requests.
	RequestTimeoutSeconds int64 `json:"requestTimeoutSeconds"`
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
	// MinTLSVersion is the minimum TLS version supported.
	// Values must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants
	MinTLSVersion string `json:"minTLSVersion,omitempty"`
	// CipherSuites contains an overridden list of ciphers for the server to support.
	// Values must match cipher suite IDs from https://golang.org/pkg/crypto/tls/#pkg-constants
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

// CertInfo relates a certificate with a private key
type CertInfo struct {
	// CertFile is a file containing a PEM-encoded certificate
	CertFile string `json:"certFile"`
	// KeyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	KeyFile string `json:"keyFile"`
}

// NamedCertificate specifies a certificate/key, and the names it should be served for
type NamedCertificate struct {
	// Names is a list of DNS names this certificate should be used to secure
	// A name can be a normal DNS name, or can contain leading wildcard segments.
	Names []string `json:"names"`
	// CertInfo is the TLS cert info for serving secure traffic
	CertInfo `json:",inline"`
}

// LeaderElection provides information to elect a leader
type LeaderElection struct {
	// disable allows leader election to be suspended while allowing a fully defaulted "normal" startup case.
	Disable bool `json:"disable,omitempty"`
	// namespace indicates which namespace the resource is in
	Namespace string `json:"namespace,omitempty"`
	// name indicates what name to use for the resource
	Name string `json:"name,omitempty"`

	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	LeaseDuration metav1.Duration `json:"leaseDuration,omitempty"`
	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading. This must be less
	// than or equal to the lease duration. This is only applicable if leader
	// election is enabled.
	RenewDeadline metav1.Duration `json:"renewDeadline,omitempty"`
	// retryPeriod is the duration the clients should wait between attempting
	// acquisition and renewal of a leadership. This is only applicable if
	// leader election is enabled.
	RetryPeriod metav1.Duration `json:"retryPeriod,omitempty"`
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

// AdmissionPluginConfig holds the necessary configuration options for admission plugins
type AdmissionPluginConfig struct {
	// Location is the path to a configuration file that contains the plugin's
	// configuration
	Location string `json:"location"`

	// Configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.RawExtension `json:"configuration"`
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
	MaximumFileRetentionDays int32 `json:"maximumFileRetentionDays"`
	// Maximum number of old log files to retain.
	MaximumRetainedFiles int32 `json:"maximumRetainedFiles"`
	// Maximum size in megabytes of the log file before it gets rotated. Defaults to 100MB.
	MaximumFileSizeMegabytes int32 `json:"maximumFileSizeMegabytes"`

	// PolicyFile is a path to the file that defines the audit policy configuration.
	PolicyFile string `json:"policyFile"`
	// PolicyConfiguration is an embedded policy configuration object to be used
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

type EtcdStorageConfig struct {
	EtcdConnectionInfo `json:",inline"`

	// StoragePrefix is the path within etcd that the OpenShift resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located.
	StoragePrefix string `json:"storagePrefix"`
}

// GenericAPIServerConfig is an inline-able struct for aggregated apiservers that need to store data in etcd
type GenericAPIServerConfig struct {
	// ServingInfo describes how to start serving
	ServingInfo HTTPServingInfo `json:"servingInfo"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins"`

	// AuditConfig describes how to configure audit information
	AuditConfig AuditConfig `json:"auditConfig"`

	// StorageConfig contains information about how to use
	StorageConfig EtcdStorageConfig `json:"storageConfig"`

	AdmissionPluginConfig map[string]AdmissionPluginConfig `json:"admissionPluginConfig"`

	KubeClientConfig KubeClientConfig `json:"kubeClientConfig"`
}

type KubeClientConfig struct {
	// kubeConfig is a .kubeconfig filename for going to the owning kube-apiserver.  Empty uses an in-cluster-config
	KubeConfig string `json:"kubeConfig"`

	// connectionOverrides specifies client overrides for system components to loop back to this master.
	ConnectionOverrides ClientConnectionOverrides `json:"connectionOverrides"`
}

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
