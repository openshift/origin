package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1alpha1api "github.com/openshift/api/operator/v1alpha1"
	webconsolev1 "github.com/openshift/api/webconsole/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftWebConsoleConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OpenShiftWebConsoleConfig `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftWebConsoleConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   OpenShiftWebConsoleConfigSpec   `json:"spec"`
	Status OpenShiftWebConsoleConfigStatus `json:"status"`
}

type OpenShiftWebConsoleConfigSpec struct {
	operatorsv1alpha1api.OperatorSpec `json:",inline"`

	// config holds a sparse config that the user wants for this component.  It only needs to be the overrides from the defaults
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. this config
	WebConsoleConfig webconsolev1.WebConsoleConfiguration `json:"config"`

	// TODO I suspect this should be automatic
	NodeSelector map[string]string `json:"nodeSelector"`

	// TODO I suspect that this should be automatic
	Replicas int32 `json:"replicas"`
}

type OpenShiftWebConsoleConfigStatus struct {
	operatorsv1alpha1api.OperatorStatus `json:",inline"`
}
