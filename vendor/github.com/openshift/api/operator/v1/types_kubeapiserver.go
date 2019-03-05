package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeAPIServer provides information to configure an operator to manage kube-apiserver.
type KubeAPIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// +required
	Spec   KubeAPIServerSpec   `json:"spec"`
	// +optional
	Status KubeAPIServerStatus `json:"status"`
}

type KubeAPIServerSpec struct {
	StaticPodOperatorSpec `json:",inline"`

	// forceRedeploymentReason can be used to force the redeployment of the kube-apiserver by providing a unique string.
	// This provides a mechanism to kick a previously failed deployment and provide a reason why you think it will work
	// this time instead of failing again on the same config.
	ForceRedeploymentReason string `json:"forceRedeploymentReason"`
}

type KubeAPIServerStatus struct {
	StaticPodOperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeAPIServerList is a collection of items
type KubeAPIServerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	// Items contains the items
	Items []KubeAPIServer `json:"items"`
}
