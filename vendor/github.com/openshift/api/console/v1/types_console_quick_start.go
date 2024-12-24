package v1

import (
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleQuickStart is an extension for guiding user through various
// workflows in the OpenShift web console.
//
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=consolequickstarts,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/750
// +openshift:file-pattern=operatorOrdering=00
// +openshift:capability=Console
// +kubebuilder:metadata:annotations="description=Extension for guiding user through various workflows in the OpenShift web console."
// +kubebuilder:metadata:annotations="displayName=ConsoleQuickStart"
// +openshift:compatibility-gen:level=2
type ConsoleQuickStart struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec ConsoleQuickStartSpec `json:"spec"`
}

// ConsoleQuickStartSpec is the desired quick start configuration.
type ConsoleQuickStartSpec struct {
	// displayName is the display name of the Quick Start.
	// +kubebuilder:validation:MinLength=1
	// +required
	DisplayName string `json:"displayName"`
	// icon is a base64 encoded image that will be displayed beside the Quick Start display name.
	// The icon should be an vector image for easy scaling. The size of the icon should be 40x40.
	// +optional
	Icon string `json:"icon,omitempty"`
	// tags is a list of strings that describe the Quick Start.
	// +optional
	Tags []string `json:"tags,omitempty"`
	// durationMinutes describes approximately how many minutes it will take to complete the Quick Start.
	// +kubebuilder:validation:Minimum=1
	// +required
	DurationMinutes int `json:"durationMinutes"`
	// description is the description of the Quick Start. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +required
	Description string `json:"description"`
	// prerequisites contains all prerequisites that need to be met before taking a Quick Start. (includes markdown)
	// +optional
	Prerequisites []string `json:"prerequisites,omitempty"`
	// introduction describes the purpose of the Quick Start. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +required
	Introduction string `json:"introduction"`
	// tasks is the list of steps the user has to perform to complete the Quick Start.
	// +kubebuilder:validation:MinItems=1
	// +required
	Tasks []ConsoleQuickStartTask `json:"tasks"`
	// conclusion sums up the Quick Start and suggests the possible next steps. (includes markdown)
	// +optional
	Conclusion string `json:"conclusion,omitempty"`
	// nextQuickStart is a list of the following Quick Starts, suggested for the user to try.
	// +optional
	NextQuickStart []string `json:"nextQuickStart,omitempty"`
	// accessReviewResources contains a list of resources that the user's access
	// will be reviewed against in order for the user to complete the Quick Start.
	// The Quick Start will be hidden if any of the access reviews fail.
	// +optional
	AccessReviewResources []authorizationv1.ResourceAttributes `json:"accessReviewResources,omitempty"`
}

// ConsoleQuickStartTask is a single step in a Quick Start.
type ConsoleQuickStartTask struct {
	// title describes the task and is displayed as a step heading.
	// +kubebuilder:validation:MinLength=1
	// +required
	Title string `json:"title"`
	// description describes the steps needed to complete the task. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +required
	Description string `json:"description"`
	// review contains instructions to validate the task is complete. The user will select 'Yes' or 'No'.
	// using a radio button, which indicates whether the step was completed successfully.
	// +optional
	Review *ConsoleQuickStartTaskReview `json:"review,omitempty"`
	// summary contains information about the passed step.
	// +optional
	Summary *ConsoleQuickStartTaskSummary `json:"summary,omitempty"`
}

// ConsoleQuickStartTaskReview contains instructions that validate a task was completed successfully.
type ConsoleQuickStartTaskReview struct {
	// instructions contains steps that user needs to take in order
	// to validate his work after going through a task. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +required
	Instructions string `json:"instructions"`
	// failedTaskHelp contains suggestions for a failed task review and is shown at the end of task. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +required
	FailedTaskHelp string `json:"failedTaskHelp"`
}

// ConsoleQuickStartTaskSummary contains information about a passed step.
type ConsoleQuickStartTaskSummary struct {
	// success describes the succesfully passed task.
	// +kubebuilder:validation:MinLength=1
	// +required
	Success string `json:"success"`
	// failed briefly describes the unsuccessfully passed task. (includes markdown)
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Failed string `json:"failed"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type ConsoleQuickStartList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsoleQuickStart `json:"items"`
}
