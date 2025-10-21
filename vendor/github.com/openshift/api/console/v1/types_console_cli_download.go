package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleCLIDownload is an extension for configuring openshift web console command line interface (CLI) downloads.
//
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=consoleclidownloads,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/481
// +openshift:file-pattern=operatorOrdering=00
// +openshift:capability=Console
// +kubebuilder:metadata:annotations="description=Extension for configuring openshift web console command line interface (CLI) downloads."
// +kubebuilder:metadata:annotations="displayName=ConsoleCLIDownload"
// +kubebuilder:printcolumn:name=Display name,JSONPath=.spec.displayName,type=string
// +kubebuilder:printcolumn:name=Age,JSONPath=.metadata.creationTimestamp,type=date
// +openshift:compatibility-gen:level=2
type ConsoleCLIDownload struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConsoleCLIDownloadSpec `json:"spec"`
}

// ConsoleCLIDownloadSpec is the desired cli download configuration.
type ConsoleCLIDownloadSpec struct {
	// displayName is the display name of the CLI download.
	DisplayName string `json:"displayName"`
	// description is the description of the CLI download (can include markdown).
	Description string `json:"description"`
	// links is a list of objects that provide CLI download link details.
	Links []CLIDownloadLink `json:"links"`
}

type CLIDownloadLink struct {
	// text is the display text for the link
	// +optional
	Text string `json:"text"`
	// href is the absolute secure URL for the link (must use https)
	// +kubebuilder:validation:Pattern=`^https://`
	Href string `json:"href"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type ConsoleCLIDownloadList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsoleCLIDownload `json:"items"`
}
