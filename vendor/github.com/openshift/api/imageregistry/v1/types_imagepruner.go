package v1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePrunerList is a slice of ImagePruner objects.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ImagePrunerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ImagePruner `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePruner is the configuration object for an image registry pruner
// managed by the registry operator.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=imagepruners,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/555
// +openshift:file-pattern=operatorOrdering=01
type ImagePruner struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	Spec ImagePrunerSpec `json:"spec"`
	// +optional
	Status ImagePrunerStatus `json:"status"`
}

// ImagePrunerSpec defines the specs for the running image pruner.
type ImagePrunerSpec struct {
	// schedule specifies when to execute the job using standard cronjob syntax: https://wikipedia.org/wiki/Cron.
	// Defaults to `0 0 * * *`.
	// +optional
	Schedule string `json:"schedule"`
	// suspend specifies whether or not to suspend subsequent executions of this cronjob.
	// Defaults to false.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`
	// keepTagRevisions specifies the number of image revisions for a tag in an image stream that will be preserved.
	// Defaults to 3.
	// +optional
	KeepTagRevisions *int `json:"keepTagRevisions,omitempty"`
	// keepYoungerThan specifies the minimum age in nanoseconds of an image and its referrers for it to be considered a candidate for pruning.
	// DEPRECATED: This field is deprecated in favor of keepYoungerThanDuration. If both are set, this field is ignored and keepYoungerThanDuration takes precedence.
	// +optional
	KeepYoungerThan *time.Duration `json:"keepYoungerThan,omitempty"`
	// keepYoungerThanDuration specifies the minimum age of an image and its referrers for it to be considered a candidate for pruning.
	// Defaults to 60m (60 minutes).
	// +optional
	// +kubebuilder:validation:Format=duration
	KeepYoungerThanDuration *metav1.Duration `json:"keepYoungerThanDuration,omitempty"`
	// resources defines the resource requests and limits for the image pruner pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// affinity is a group of node affinity scheduling rules for the image pruner pod.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// nodeSelector defines the node selection constraints for the image pruner pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// tolerations defines the node tolerations for the image pruner pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// successfulJobsHistoryLimit specifies how many successful image pruner jobs to retain.
	// Defaults to 3 if not set.
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`
	// failedJobsHistoryLimit specifies how many failed image pruner jobs to retain.
	// Defaults to 3 if not set.
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`
	// ignoreInvalidImageReferences indicates whether the pruner can ignore
	// errors while parsing image references.
	// +optional
	IgnoreInvalidImageReferences bool `json:"ignoreInvalidImageReferences,omitempty"`
	// logLevel sets the level of log output for the pruner job.
	//
	// Valid values are: "Normal", "Debug", "Trace", "TraceAll".
	// Defaults to "Normal".
	// +optional
	// +kubebuilder:default=Normal
	LogLevel operatorv1.LogLevel `json:"logLevel,omitempty"`
}

// ImagePrunerStatus reports image pruner operational status.
type ImagePrunerStatus struct {
	// observedGeneration is the last generation change that has been applied.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// conditions is a list of conditions and their status.
	// +optional
	Conditions []operatorv1.OperatorCondition `json:"conditions,omitempty"`
}
