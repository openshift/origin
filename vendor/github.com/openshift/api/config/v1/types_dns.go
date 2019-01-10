package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNS holds cluster-wide information about DNS.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type DNS struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec DNSSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status DNSStatus `json:"status"`
}

type DNSSpec struct {
	// baseDomain is the base domain of the cluster. All managed DNS records will
	// be sub-domains of this base.
	//
	// For example, given the base domain `openshift.example.com`, an API server
	// DNS record may be created for `cluster-api.openshift.example.com`.
	BaseDomain string `json:"baseDomain"`
}

type DNSStatus struct {
	// dnsSuffix (service-ca amongst others)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNS `json:"items"`
}
