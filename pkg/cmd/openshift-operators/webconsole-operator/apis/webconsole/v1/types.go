package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

type ManagementState string

const (
	Enabled   ManagementState = "Enabled"
	Unmanaged ManagementState = "Unmanaged"
	Disabled  ManagementState = "Disabled"
)

type OpenShiftWebConsoleConfigSpec struct {
	ManagementState ManagementState `json:"managementState"`

	// TODO I think this should eventually embed the entire master config
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. existing config
	// 3. this config
	WebConsoleConfig webconsolev1.WebConsoleConfiguration `json:"config"`

	// ImagePullSpec is the image to use
	ImagePullSpec string `json:"imagePullSpec"`

	// Version is major.minor.micro-patch?.  Usually patch is ignored.
	Version string `json:"version"`

	LogLevel int64 `json:"logLevel"`

	// TODO I suspect this should be automatic
	NodeSelector map[string]string `json:"nodeSelector"`

	// TODO I suspect that this should be automatic
	Replicas int32 `json:"replicas"`
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type OpenShiftOperatorCondition struct {
	Type               String          `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
}

type OpenShiftWebConsoleConfigStatus struct {
	Conditions []OpenShiftOperatorCondition `json:"conditions,omitempty"`

	InProgressVersion     string `json:"inProgressVersion"`
	LastSuccessfulVersion string `json:"lastSuccessfulVersion"`

	// LastUnsuccessfulRunErrors tracks errors from the last run.  If we put the system back into a working state
	// these will be from the last failure.
	LastUnsuccessfulRunErrors []string `json:"lastUnsuccessfulRunErrors"`
}
