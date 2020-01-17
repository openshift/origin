package v1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePrunerList is a slice of ImagePruner objects.
type ImagePrunerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []ImagePruner `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePruner is the configuration object for an image registry pruner
// managed by the registry operator.
type ImagePruner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	Spec ImagePrunerSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// +optional
	Status ImagePrunerStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// ImagePrunerSpec defines the specs for the running image pruner.
type ImagePrunerSpec struct {
	// schedule specifies when to execute the job using standard cronjob syntax: https://wikipedia.org/wiki/Cron.
	// Defaults to `0 0 * * *`.
	// +optional
	Schedule string `json:"schedule" protobuf:"bytes,1,opt,name=schedule"`
	// suspend specifies whether or not to suspend subsequent executions of this cronjob.
	// Defaults to false.
	// +optional
	Suspend *bool `json:"suspend,omitempty" protobuf:"bytes,2,opt,name=suspend"`
	// keepTagRevisions specifies the number of image revisions for a tag in an image stream that will be preserved.
	// Defaults to 5.
	// +optional
	KeepTagRevisions *int `json:"keepTagRevisions,omitempty" protobuf:"bytes,3,opt,name=keepTagRevisions"`
	// keepYoungerThan specifies the minimum age of an image and its referrers for it to be considered a candidate for pruning.
	// Defaults to 96h (96 hours).
	// +optional
	KeepYoungerThan *time.Duration `json:"keepYoungerThan,omitempty" protobuf:"varint,4,opt,name=keepYoungerThan,casttype=time.Duration"`
	// resources defines the resource requests and limits for the image pruner pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,5,opt,name=resources"`
	// affinity is a group of node affinity scheduling rules for the image pruner pod.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty" protobuf:"bytes,6,opt,name=affinity"`
	// nodeSelector defines the node selection constraints for the image pruner pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	// tolerations defines the node tolerations for the image pruner pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,8,rep,name=tolerations"`
	// successfulJobsHistoryLimit specifies how many successful image pruner jobs to retain.
	// Defaults to 3 if not set.
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty" protobuf:"bytes,1,opt,name=successfulJobsHistoryLimit"`
	// failedJobsHistoryLimit specifies how many failed image pruner jobs to retain.
	// Defaults to 3 if not set.
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty" protobuf:"bytes,2,opt,name=failedJobsHistoryLimit"`
}

// ImagePrunerStatus reports image pruner operational status.
type ImagePrunerStatus struct {
	// observedGeneration is the last generation change that has been applied.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"bytes,1,opt,name=observedGeneration"`
	// conditions is a list of conditions and their status.
	// +optional
	Conditions []operatorv1.OperatorCondition `json:"conditions,omitempty" protobuf:"bytes,2,rep,name=conditions"`
}
