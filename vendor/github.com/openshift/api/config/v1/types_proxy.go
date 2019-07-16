package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Proxy holds cluster-wide information on how to configure default proxies for the cluster. The canonical name is `cluster`
type Proxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec holds user-settable values for the proxy configuration
	// +kubebuilder:validation:Required
	// +required
	Spec ProxySpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status ProxyStatus `json:"status"`
}

// ProxySpec contains cluster proxy creation configuration.
type ProxySpec struct {
	// httpProxy is the URL of the proxy for HTTP requests.  Empty means unset and will not result in an env var.
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// httpsProxy is the URL of the proxy for HTTPS requests.  Empty means unset and will not result in an env var.
	// +optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// noProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used.
	// Empty means unset and will not result in an env var.
	// +optional
	NoProxy string `json:"noProxy,omitempty"`

	// readinessEndpoints is a list of endpoints used to verify readiness of the proxy.
	// +optional
	ReadinessEndpoints []string `json:"readinessEndpoints,omitempty"`
}

// ProxyStatus shows current known state of the cluster proxy.
type ProxyStatus struct {
	// httpProxy is the URL of the proxy for HTTP requests.
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// httpsProxy is the URL of the proxy for HTTPS requests.
	// +optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// noProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used.
	// +optional
	NoProxy string `json:"noProxy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProxyList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Proxy `json:"items"`
}
