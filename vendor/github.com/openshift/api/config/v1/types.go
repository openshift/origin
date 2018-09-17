package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HTTPServingInfo holds configuration for serving HTTP
type HTTPServingInfo struct {
	// ServingInfo is the HTTP serving information
	ServingInfo `json:",inline" protobuf:"bytes,1,opt,name=servingInfo"`
	// MaxRequestsInFlight is the number of concurrent requests allowed to the server. If zero, no limit.
	MaxRequestsInFlight int64 `json:"maxRequestsInFlight" protobuf:"varint,2,opt,name=maxRequestsInFlight"`
	// RequestTimeoutSeconds is the number of seconds before requests are timed out. The default is 60 minutes, if
	// -1 there is no limit on requests.
	RequestTimeoutSeconds int64 `json:"requestTimeoutSeconds" protobuf:"varint,3,opt,name=requestTimeoutSeconds"`
}

// ServingInfo holds information about serving web pages
type ServingInfo struct {
	// BindAddress is the ip:port to serve on
	BindAddress string `json:"bindAddress" protobuf:"bytes,1,opt,name=bindAddress"`
	// BindNetwork is the type of network to bind to - defaults to "tcp4", accepts "tcp",
	// "tcp4", and "tcp6"
	BindNetwork string `json:"bindNetwork" protobuf:"bytes,2,opt,name=bindNetwork"`
	// CertInfo is the TLS cert info for serving secure traffic.
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline" protobuf:"bytes,3,opt,name=certInfo"`
	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA string `json:"clientCA" protobuf:"bytes,4,opt,name=clientCA"`
	// NamedCertificates is a list of certificates to use to secure requests to specific hostnames
	NamedCertificates []NamedCertificate `json:"namedCertificates" protobuf:"bytes,5,rep,name=namedCertificates"`
	// MinTLSVersion is the minimum TLS version supported.
	// Values must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants
	MinTLSVersion string `json:"minTLSVersion,omitempty" protobuf:"bytes,6,opt,name=minTLSVersion"`
	// CipherSuites contains an overridden list of ciphers for the server to support.
	// Values must match cipher suite IDs from https://golang.org/pkg/crypto/tls/#pkg-constants
	CipherSuites []string `json:"cipherSuites,omitempty" protobuf:"bytes,7,rep,name=cipherSuites"`
}

// CertInfo relates a certificate with a private key
type CertInfo struct {
	// CertFile is a file containing a PEM-encoded certificate
	CertFile string `json:"certFile" protobuf:"bytes,1,opt,name=certFile"`
	// KeyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	KeyFile string `json:"keyFile" protobuf:"bytes,2,opt,name=keyFile"`
}

// NamedCertificate specifies a certificate/key, and the names it should be served for
type NamedCertificate struct {
	// Names is a list of DNS names this certificate should be used to secure
	// A name can be a normal DNS name, or can contain leading wildcard segments.
	Names []string `json:"names" protobuf:"bytes,1,rep,name=names"`
	// CertInfo is the TLS cert info for serving secure traffic
	CertInfo `json:",inline" protobuf:"bytes,2,opt,name=certInfo"`
}

// LeaderElection provides information to elect a leader
type LeaderElection struct {
	// disable allows leader election to be suspended while allowing a fully defaulted "normal" startup case.
	Disable bool `json:"disable,omitempty" protobuf:"varint,1,opt,name=disable"`
	// namespace indicates which namespace the resource is in
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	// name indicates what name to use for the resource
	Name string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`

	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	LeaseDuration metav1.Duration `json:"leaseDuration,omitempty" protobuf:"bytes,4,opt,name=leaseDuration"`
	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading. This must be less
	// than or equal to the lease duration. This is only applicable if leader
	// election is enabled.
	RenewDeadline metav1.Duration `json:"renewDeadline,omitempty" protobuf:"bytes,5,opt,name=renewDeadline"`
	// retryPeriod is the duration the clients should wait between attempting
	// acquisition and renewal of a leadership. This is only applicable if
	// leader election is enabled.
	RetryPeriod metav1.Duration `json:"retryPeriod,omitempty" protobuf:"bytes,6,opt,name=retryPeriod"`
}

// StringSource allows specifying a string inline, or externally via env var or file.
// When it contains only a string value, it marshals to a simple JSON string.
type StringSource struct {
	// StringSourceSpec specifies the string value, or external location
	StringSourceSpec `json:",inline" protobuf:"bytes,1,opt,name=stringSourceSpec"`
}

// StringSourceSpec specifies a string value, or external location
type StringSourceSpec struct {
	// Value specifies the cleartext value, or an encrypted value if keyFile is specified.
	Value string `json:"value" protobuf:"bytes,1,opt,name=value"`

	// Env specifies an envvar containing the cleartext value, or an encrypted value if the keyFile is specified.
	Env string `json:"env" protobuf:"bytes,2,opt,name=env"`

	// File references a file containing the cleartext value, or an encrypted value if a keyFile is specified.
	File string `json:"file" protobuf:"bytes,3,opt,name=file"`

	// KeyFile references a file containing the key to use to decrypt the value.
	KeyFile string `json:"keyFile" protobuf:"bytes,4,opt,name=keyFile"`
}

// RemoteConnectionInfo holds information necessary for establishing a remote connection
type RemoteConnectionInfo struct {
	// URL is the remote URL to connect to
	URL string `json:"url" protobuf:"bytes,1,opt,name=url"`
	// CA is the CA for verifying TLS connections
	CA string `json:"ca" protobuf:"bytes,2,opt,name=ca"`
	// CertInfo is the TLS client cert information to present
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline" protobuf:"bytes,3,opt,name=certInfo"`
}

// AdmissionPluginConfig holds the necessary configuration options for admission plugins
type AdmissionPluginConfig struct {
	// Location is the path to a configuration file that contains the plugin's
	// configuration
	Location string `json:"location" protobuf:"bytes,1,opt,name=location"`

	// Configuration is an embedded configuration object to be used as the plugin's
	// configuration. If present, it will be used instead of the path to the configuration file.
	Configuration runtime.RawExtension `json:"configuration" protobuf:"bytes,2,opt,name=configuration"`
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
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
	// All requests coming to the apiserver will be logged to this file.
	AuditFilePath string `json:"auditFilePath" protobuf:"bytes,2,opt,name=auditFilePath"`
	// Maximum number of days to retain old log files based on the timestamp encoded in their filename.
	MaximumFileRetentionDays int32 `json:"maximumFileRetentionDays" protobuf:"varint,3,opt,name=maximumFileRetentionDays"`
	// Maximum number of old log files to retain.
	MaximumRetainedFiles int32 `json:"maximumRetainedFiles" protobuf:"varint,4,opt,name=maximumRetainedFiles"`
	// Maximum size in megabytes of the log file before it gets rotated. Defaults to 100MB.
	MaximumFileSizeMegabytes int32 `json:"maximumFileSizeMegabytes" protobuf:"varint,5,opt,name=maximumFileSizeMegabytes"`

	// PolicyFile is a path to the file that defines the audit policy configuration.
	PolicyFile string `json:"policyFile" protobuf:"bytes,6,opt,name=policyFile"`
	// PolicyConfiguration is an embedded policy configuration object to be used
	// as the audit policy configuration. If present, it will be used instead of
	// the path to the policy file.
	PolicyConfiguration runtime.RawExtension `json:"policyConfiguration" protobuf:"bytes,7,opt,name=policyConfiguration"`

	// Format of saved audits (legacy or json).
	LogFormat LogFormatType `json:"logFormat" protobuf:"bytes,8,opt,name=logFormat,casttype=LogFormatType"`

	// Path to a .kubeconfig formatted file that defines the audit webhook configuration.
	WebHookKubeConfig string `json:"webHookKubeConfig" protobuf:"bytes,9,opt,name=webHookKubeConfig"`
	// Strategy for sending audit events (block or batch).
	WebHookMode WebHookModeType `json:"webHookMode" protobuf:"bytes,10,opt,name=webHookMode,casttype=WebHookModeType"`
}

// EtcdConnectionInfo holds information necessary for connecting to an etcd server
type EtcdConnectionInfo struct {
	// URLs are the URLs for etcd
	URLs []string `json:"urls" protobuf:"bytes,1,rep,name=urls"`
	// CA is a file containing trusted roots for the etcd server certificates
	CA string `json:"ca" protobuf:"bytes,2,opt,name=ca"`
	// CertInfo is the TLS client cert information for securing communication to etcd
	// this is anonymous so that we can inline it for serialization
	CertInfo `json:",inline" protobuf:"bytes,3,opt,name=certInfo"`
}

type EtcdStorageConfig struct {
	EtcdConnectionInfo `json:",inline" protobuf:"bytes,1,opt,name=etcdConnectionInfo"`

	// StoragePrefix is the path within etcd that the OpenShift resources will
	// be rooted under. This value, if changed, will mean existing objects in etcd will
	// no longer be located.
	StoragePrefix string `json:"storagePrefix" protobuf:"bytes,2,opt,name=storagePrefix"`
}

// GenericAPIServerConfig is an inline-able struct for aggregated apiservers that need to store data in etcd
type GenericAPIServerConfig struct {
	// ServingInfo describes how to start serving
	ServingInfo HTTPServingInfo `json:"servingInfo" protobuf:"bytes,1,opt,name=servingInfo"`

	// CORSAllowedOrigins
	CORSAllowedOrigins []string `json:"corsAllowedOrigins" protobuf:"bytes,2,rep,name=corsAllowedOrigins"`

	// AuditConfig describes how to configure audit information
	AuditConfig AuditConfig `json:"auditConfig" protobuf:"bytes,3,opt,name=auditConfig"`

	// StorageConfig contains information about how to use
	StorageConfig EtcdStorageConfig `json:"storageConfig" protobuf:"bytes,4,opt,name=storageConfig"`

	AdmissionPluginConfig map[string]AdmissionPluginConfig `json:"admissionPluginConfig" protobuf:"bytes,5,rep,name=admissionPluginConfig"`

	KubeClientConfig KubeClientConfig `json:"kubeClientConfig" protobuf:"bytes,6,opt,name=kubeClientConfig,json=kubeClientConfig"`
}

type KubeClientConfig struct {
	// kubeConfig is a .kubeconfig filename for going to the owning kube-apiserver.  Empty uses an in-cluster-config
	KubeConfig string `json:"kubeConfig" protobuf:"bytes,1,opt,name=kubeConfig"`

	// connectionOverrides specifies client overrides for system components to loop back to this master.
	ConnectionOverrides ClientConnectionOverrides `json:"connectionOverrides" protobuf:"bytes,2,opt,name=connectionOverrides"`
}

type ClientConnectionOverrides struct {
	// acceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string `json:"acceptContentTypes" protobuf:"bytes,1,opt,name=acceptContentTypes"`
	// contentType is the content type used when sending data to the server from this client.
	ContentType string `json:"contentType" protobuf:"bytes,2,opt,name=contentType"`

	// qps controls the number of queries per second allowed for this connection.
	QPS float32 `json:"qps" protobuf:"fixed32,3,opt,name=qps"`
	// burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32 `json:"burst" protobuf:"varint,4,opt,name=burst"`
}
