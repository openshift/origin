package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=4

// ConsolePlugin is an extension for customizing OpenShift web console by
// dynamically loading code from another service running on the cluster.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
type ConsolePlugin struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Required
	// +required
	Spec ConsolePluginSpec `json:"spec"`
}

// ConsolePluginSpec is the desired plugin configuration.
type ConsolePluginSpec struct {
	// displayName is the display name of the plugin.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// service is a Kubernetes Service that exposes the plugin using a
	// deployment with an HTTP server. The Service must use HTTPS and
	// Service serving certificate. The console backend will proxy the
	// plugins assets from the Service using the service CA bundle.
	// +kubebuilder:validation:Required
	// +required
	Service ConsolePluginService `json:"service"`
	// proxy is a list of proxies that describe various service type
	// to which the plugin needs to connect to.
	// +kubebuilder:validation:Optional
	// +optional
	Proxy []ConsolePluginProxy `json:"proxy,omitempty"`
}

// ConsolePluginProxy holds information on various service types
// to which console's backend will proxy the plugin's requests.
type ConsolePluginProxy struct {
	// type is the type of the console plugin's proxy. Currently only "Service" is supported.
	// +kubebuilder:validation:Required
	// +required
	Type ConsolePluginProxyType `json:"type"`
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
	// +required
	Alias string `json:"alias"`
	// service is an in-cluster Service that the plugin will connect to.
	// The Service must use HTTPS. The console backend exposes an endpoint
	// in order to proxy communication between the plugin and the Service.
	// Note: service field is required for now, since currently only "Service"
	// type is supported.
	// +kubebuilder:validation:Required
	// +required
	Service ConsolePluginProxyServiceConfig `json:"service,omitempty"`
	// caCertificate provides the cert authority certificate contents,
	// in case the proxied Service is using custom service CA.
	// By default, the service CA bundle provided by the service-ca operator is used.
	// +kubebuilder:validation:Pattern=`^-----BEGIN CERTIFICATE-----([\s\S]*)-----END CERTIFICATE-----\s?$`
	// +kubebuilder:validation:Optional
	// +optional
	CACertificate string `json:"caCertificate,omitempty"`
	// authorize indicates if the proxied request should contain the logged-in user's
	// OpenShift access token in the "Authorization" request header. For example:
	//
	// Authorization: Bearer sha256~kV46hPnEYhCWFnB85r5NrprAxggzgb6GOeLbgcKNsH0
	//
	// By default the access token is not part of the proxied request.
	// +kubebuilder:default:=false
	// +kubebuilder:validation:Optional
	// +optional
	Authorize bool `json:"authorize,omitempty"`
}

// ProxyType is an enumeration of available proxy types
// +kubebuilder:validation:Pattern=`^(Service)$`
type ConsolePluginProxyType string

const (
	// ProxyTypeService is used when proxying communication to a Service
	ProxyTypeService ConsolePluginProxyType = "Service"
)

// ProxyTypeServiceConfig holds information on Service to which
// console's backend will proxy the plugin's requests.
type ConsolePluginProxyServiceConfig struct {
	// name of Service that the plugin needs to connect to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Name string `json:"name"`
	// namespace of Service that the plugin needs to connect to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Namespace string `json:"namespace"`
	// port on which the Service that the plugin needs to connect to
	// is listening on.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	// +required
	Port int32 `json:"port"`
}

// ConsolePluginService holds information on Service that is serving
// console dynamic plugin assets.
type ConsolePluginService struct {
	// name of Service that is serving the plugin assets.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Name string `json:"name"`
	// namespace of Service that is serving the plugin assets.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Namespace string `json:"namespace"`
	// port on which the Service that is serving the plugin is listening to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum:=65535
	// +kubebuilder:validation:Minimum:=1
	// +required
	Port int32 `json:"port"`
	// basePath is the path to the plugin's assets. The primary asset it the
	// manifest file called `plugin-manifest.json`, which is a JSON document
	// that contains metadata about the plugin and the extensions.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^/`
	// +kubebuilder:default:="/"
	// +required
	BasePath string `json:"basePath"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=4

// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
type ConsolePluginList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsolePlugin `json:"items"`
}
