package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleLink is an extension for customizing OpenShift web console links.
type ConsoleLink struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConsoleLinkSpec `json:"spec"`
}

// ConsoleLinkSpec is the desired console link configuration.
type ConsoleLinkSpec struct {
	Link `json:",inline"`
	// location determines which location in the console the link will be appended to.
	Location ConsoleLinkLocation `json:"location"`
}

// ConsoleLinkLocationSelector is a set of possible menu targets to which a link may be appended.
type ConsoleLinkLocation string

const (
	// HelpMenu indicates that the link should appear in the help menu in the console.
	HelpMenu ConsoleLinkLocation = "HelpMenu"
	// UserMenu indicates that the link should appear in the user menu in the console.
	UserMenu ConsoleLinkLocation = "UserMenu"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleLinkList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []ConsoleLink `json:"items"`
}
