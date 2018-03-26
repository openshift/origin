package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftOrchestrationConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OpenShiftOrchestrationConfig `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftOrchestrationConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   OpenShiftOrchestrationConfigSpec   `json:"spec"`
	Status OpenShiftOrchestrationConfigStatus `json:"status"`
}

type OpenShiftOrchestrationConfigSpec struct {
	OpenShiftControlPlane ControlPlaneComponent `json:"openShiftControlPlane"`
	WebConsole            WebConsoleComponent   `json:"webConsole"`
}

type OpenShiftOrchestrationConfigStatus struct {
	InProgressVersion     string `json:"inProgressVersion"`
	LastSuccessfulVersion string `json:"lastSuccessfulVersion"`

	// LastUnsuccessfulRunErrors tracks errors from the last run.  If we put the system back into a working state
	// these will be from the last failure.
	LastUnsuccessfulRunErrors []string `json:"lastUnsuccessfulRunErrors"`
}

type Component struct {
	Enabled               bool   `json:"enabled"`
	OperatorImagePullSpec string `json:"operatorImagePullSpec"`
	OperatorLogLevel      int64  `json:"operatorLogLevel"`
	ImagePullSpec         string `json:"imagePullSpec"`
	Version               string `json:"version"`
	LogLevel              int64  `json:"logLevel"`
}

type ControlPlaneComponent struct {
	Component `json:",inline"`

	// TODO I think this goes away once I have a default and a configmap
	ControllerConfigHostPath string `json:"controllerConfigHostPath"`
	APIServerConfigHostPath  string `json:"apiServerConfigHostPath"`
}

// TODO I think this goes away once I have a default and a configmap
type WebConsoleComponent struct {
	Component `json:",inline"`
}
