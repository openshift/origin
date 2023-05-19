package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=1

// ConsolePlugin is an extension for customizing OpenShift web console by
// dynamically loading code from another service running on the cluster.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
type ConsolePlugin struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Required
	Spec ConsolePluginSpec `json:"spec"`
}

// ConsolePluginSpec is the desired plugin configuration.
type ConsolePluginSpec struct {
	// displayName is the display name of the plugin.
	// The dispalyName should be between 1 and 128 characters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	DisplayName string `json:"displayName"`
	// backend holds the configuration of backend which is serving console's plugin .
	// +kubebuilder:validation:Required
	Backend ConsolePluginBackend `json:"backend"`
	// proxy is a list of proxies that describe various service type
	// to which the plugin needs to connect to.
	// +optional
	Proxy []ConsolePluginProxy `json:"proxy,omitempty"`
	// i18n is the configuration of plugin's localization resources.
	// +optional
	I18n ConsolePluginI18n `json:"i18n"`
}

// LoadType is an enumeration of i18n loading types
// +kubebuilder:validation:Enum:=Preload;Lazy;""
type LoadType string

const (
	// Preload will load all plugin's localization resources during
	// loading of the plugin.
	Preload LoadType = "Preload"
	// Lazy wont preload any plugin's localization resources, instead
	// will leave thier loading to runtime's lazy-loading.
	Lazy LoadType = "Lazy"
	// Empty is the default value of the LoadType field and it's
	// purpose is to improve discoverability of the field. The
	// the behaviour is equivalent to Lazy type.
	Empty LoadType = ""
)

// ConsolePluginI18n holds information on localization resources that are served by
// the dynamic plugin.
type ConsolePluginI18n struct {
	// loadType indicates how the plugin's localization resource should be loaded.
	// Valid values are Preload, Lazy and the empty string.
	// When set to Preload, all localization resources are fetched when the plugin is loaded.
	// When set to Lazy, localization resources are lazily loaded as and when they are required by the console.
	// When omitted or set to the empty string, the behaviour is equivalent to Lazy type.
	// +kubebuilder:validation:Required
	LoadType LoadType `json:"loadType"`
}

// ConsolePluginProxy holds information on various service types
// to which console's backend will proxy the plugin's requests.
type ConsolePluginProxy struct {
	// endpoint provides information about endpoint to which the request is proxied to.
	// +kubebuilder:validation:Required
	Endpoint ConsolePluginProxyEndpoint `json:"endpoint"`
	// alias is a proxy name that identifies the plugin's proxy. An alias name
	// should be unique per plugin. The console backend exposes following
	// proxy endpoint:
	//
	// /api/proxy/plugin/<plugin-name>/<proxy-alias>/<request-path>?<optional-query-parameters>
	//
	// Request example path:
	//
	// /api/proxy/plugin/acm/search/pods?namespace=openshift-apiserver
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9-_]+$`
	Alias string `json:"alias"`
	// caCertificate provides the cert authority certificate contents,
	// in case the proxied Service is using custom service CA.
	// By default, the service CA bundle provided by the service-ca operator is used.
	// +kubebuilder:validation:Pattern=`^-----BEGIN CERTIFICATE-----([\s\S]*)-----END CERTIFICATE-----\s?$`
	// +optional
	CACertificate string `json:"caCertificate,omitempty"`
	// authorization provides information about authorization type,
	// which the proxied request should contain
	// +kubebuilder:default:="None"
	// +optional
	Authorization AuthorizationType `json:"authorization,omitempty"`
}

// ConsolePluginProxyEndpoint holds information about the endpoint to which
// request will be proxied to.
// +union
type ConsolePluginProxyEndpoint struct {
	// type is the type of the console plugin's proxy. Currently only "Service" is supported.
	//
	// ---
	// + When handling unknown values, consumers should report an error and stop processing the plugin.
	//
	// +kubebuilder:validation:Required
	// +unionDiscriminator
	Type ConsolePluginProxyType `json:"type"`
	// service is an in-cluster Service that the plugin will connect to.
	// The Service must use HTTPS. The console backend exposes an endpoint
	// in order to proxy communication between the plugin and the Service.
	// Note: service field is required for now, since currently only "Service"
	// type is supported.
	// +optional
	Service *ConsolePluginProxyServiceConfig `json:"service,omitempty"`
}

// ProxyType is an enumeration of available proxy types
// +kubebuilder:validation:Enum:=Service
type ConsolePluginProxyType string

const (
	// ProxyTypeService is used when proxying communication to a Service
	ProxyTypeService ConsolePluginProxyType = "Service"
)

// AuthorizationType is an enumerate of available authorization types
// +kubebuilder:validation:Enum:=UserToken;None
type AuthorizationType string

const (
	// UserToken indicates that the proxied request should contain the logged-in user's
	// OpenShift access token in the "Authorization" request header. For example:
	//
	// Authorization: Bearer sha256~kV46hPnEYhCWFnB85r5NrprAxggzgb6GOeLbgcKNsH0
	//
	UserToken AuthorizationType = "UserToken"
	// None indicates that proxied request wont contain authorization of any type.
	None AuthorizationType = "None"
)

// ProxyTypeServiceConfig holds information on Service to which
// console's backend will proxy the plugin's requests.
type ConsolePluginProxyServiceConfig struct {
	// name of Service that the plugin needs to connect to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`
	// namespace of Service that the plugin needs to connect to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Namespace string `json:"namespace"`
	// port on which the Service that the plugin needs to connect to
	// is listening on.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	Port int32 `json:"port"`
}

// ConsolePluginBackendType is an enumeration of available backend types
// +kubebuilder:validation:Enum:=Service
type ConsolePluginBackendType string

const (
	// Service is used when plugin's backend is served by a Kubernetes Service
	Service ConsolePluginBackendType = "Service"
)

// ConsolePluginBackend holds information about the endpoint which serves
// the console's plugin
// +union
type ConsolePluginBackend struct {
	// type is the backend type which servers the console's plugin. Currently only "Service" is supported.
	//
	// ---
	// + When handling unknown values, consumers should report an error and stop processing the plugin.
	//
	// +kubebuilder:validation:Required
	// +unionDiscriminator
	Type ConsolePluginBackendType `json:"type"`
	// service is a Kubernetes Service that exposes the plugin using a
	// deployment with an HTTP server. The Service must use HTTPS and
	// Service serving certificate. The console backend will proxy the
	// plugins assets from the Service using the service CA bundle.
	// +optional
	Service *ConsolePluginService `json:"service"`
}

// ConsolePluginService holds information on Service that is serving
// console dynamic plugin assets.
type ConsolePluginService struct {
	// name of Service that is serving the plugin assets.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`
	// namespace of Service that is serving the plugin assets.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Namespace string `json:"namespace"`
	// port on which the Service that is serving the plugin is listening to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	Port int32 `json:"port"`
	// basePath is the path to the plugin's assets. The primary asset it the
	// manifest file called `plugin-manifest.json`, which is a JSON document
	// that contains metadata about the plugin and the extensions.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.\-_~!$&'()*+,;=:@\/]*$`
	// +kubebuilder:default:="/"
	// +optional
	BasePath string `json:"basePath"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=1

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
type ConsolePluginList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsolePlugin `json:"items"`
}
