package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WebConsoleConfiguration holds the necessary configuration options for serving the web console
type WebConsoleConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// ServingInfo is the HTTP serving information for these assets
	ServingInfo HTTPServingInfo `json:"servingInfo" protobuf:"bytes,1,opt,name=servingInfo"`

	// ClusterInfo holds information the web console needs to talk to the cluster such as master public URL
	// and metrics public URL
	ClusterInfo ClusterInfo `json:"clusterInfo" protobuf:"bytes,2,rep,name=clusterInfo"`

	// Features define various feature gates for the web console
	Features FeaturesConfiguration `json:"features" protobuf:"bytes,3,opt,name=featureInfo"`

	// Extensions define custom scripts, stylesheets, and properties used for web console customization
	Extensions ExtensionsConfiguration `json:"extensions" protobuf:"bytes,4,rep,name=extensions"`
}

// ClusterInfo holds information the web console needs to talk to the cluster such as master public URL and
// metrics public URL
type ClusterInfo struct {
	// ConsolePublicURL is where you can find the web console server (TODO do we really need this?)
	ConsolePublicURL string `json:"consolePublicURL" protobuf:"bytes,1,opt,name=consolePublicURL"`

	// MasterPublicURL is how the web console can access the OpenShift v1 server
	MasterPublicURL string `json:"masterPublicURL" protobuf:"bytes,2,opt,name=masterPublicURL"`

	// LoggingPublicURL is the public endpoint for logging (optional)
	LoggingPublicURL string `json:"loggingPublicURL" protobuf:"bytes,3,opt,name=loggingPublicURL"`

	// MetricsPublicURL is the public endpoint for metrics (optional)
	MetricsPublicURL string `json:"metricsPublicURL" protobuf:"bytes,4,opt,name=metricsPublicURL"`

	// LogoutPublicURL is an optional, absolute URL to redirect web browsers to after logging out of the web
	// console. If not specified, the built-in logout page is shown.
	LogoutPublicURL string `json:"logoutPublicURL" protobuf:"bytes,5,opt,name=logoutPublicURL"`
}

// FeaturesConfiguration defines various feature gates for the web console
type FeaturesConfiguration struct {
	// InactivityTimeoutMinutes is the number of minutes of inactivity before you are automatically logged out of
	// the web console (optional). If set to 0, inactivity timeout is disabled.
	InactivityTimeoutMinutes int64 `json:"inactivityTimeoutMinutes" protobuf:"varint,1,opt,name=inactivityTimeoutMinutes"`

	// ClusterResourceOverridesEnabled indicates that the cluster is configured for overcommit. When set to
	// true, the web console will hide the CPU request, CPU limit, and memory request fields in its editors
	// and skip validation on those fields. The memory limit field will still be displayed.
	ClusterResourceOverridesEnabled bool `json:"clusterResourceOverridesEnabled" protobuf:"varint,2,opt,name=clusterResourceOverridesEnabled"`
}

// ExtensionsConfiguration holds custom script, stylesheets, and properties used for web console customization
type ExtensionsConfiguration struct {
	// ScriptURLs are URLs to load as scripts when the Web Console loads. The URLs must be accessible from
	// the browser.
	ScriptURLs []string `json:"scriptURLs" protobuf:"bytes,1,rep,name=scriptURLs"`
	// StylesheetURLs are URLs to load as stylesheets when the Web Console loads. The URLs must be accessible
	// from the browser.
	StylesheetURLs []string `json:"stylesheetURLs" protobuf:"bytes,2,rep,name=stylesheetURLs"`
	// Properties are key(string) and value(string) pairs that will be injected into the console under the
	// global variable OPENSHIFT_EXTENSION_PROPERTIES
	Properties map[string]string `json:"properties" protobuf:"bytes,3,rep,name=properties"`
}

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
