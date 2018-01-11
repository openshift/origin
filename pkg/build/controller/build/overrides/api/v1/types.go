package v1

import (
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/api/build/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	metav1.TypeMeta `json:",inline"`

	// forcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool `json:"forcePull"`

	// imageLabels is a list of docker labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	ImageLabels []buildapi.ImageLabel `json:"imageLabels,omitempty"`

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string `json:"annotations,omitempty"`

	// tolerations is a list of Tolerations that will override any existing
	// tolerations set on a build pod.
	Tolerations []kapi.Toleration `json:"tolerations,omitempty"`
}
