package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleCLIDownload is an extension for configuring openshift web console command line interface (CLI) downloads.
type ConsoleCLIDownload struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConsoleCLIDownloadSpec `json:"spec"`
}

// ConsoleCLIDownloadSpec is the desired cli download configuration.
type ConsoleCLIDownloadSpec struct {
	// displayName is the display name of the CLI download.
	DisplayName string `json:"displayName"`
	// description is the description of the CLI download (can include markdown).
	Description string `json:"description"`
	// links is a list of objects that provide CLI download link details.
	Links []Link `json:"links"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleCLIDownloadList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []ConsoleCLIDownload `json:"items"`
}
