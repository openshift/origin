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
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec ConsolePluginSpec `json:"spec"`
}

// ConsolePluginSpec is the desired plugin configuration.
type ConsolePluginSpec struct {
	// displayName is the display name of the plugin.
	DisplayName string `json:"displayName"`
	// service is a Kubernetes Service that exposes the plugin using a
	// deployment with an HTTP server. The Service must use HTTPS and
	// service serving certificate. The console backend will proxy the
	// plugins assets from the Service using the service CA bundle.
	Service ConsolePluginService `json:"service"`
}

// ConsolePluginService holds information on service that is serving
// console dynamic plugin assets.
type ConsolePluginService struct {
	// name of Service that is serving the plugin.
	Name string `json:"name"`
	// namespace of Service that is serving the plugin.
	Namespace string `json:"namespace"`
	// port on which the Service that is serving the plugin is listening to.
	Port int32 `json:"port"`
	// basePath is the path to the plugin's assets. The primary asset it the
	// manifest file called `plugin-manifest.json`, which is a JSON document
	// that contains metadata about the plugin and the extensions.
	BasePath string `json:"basePath"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=4

// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
type ConsolePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ConsolePlugin `json:"items"`
}
