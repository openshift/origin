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
	// applicationMenu holds information about section and icon used for the link in the
	// application menu, and it is applicable only when location is set to ApplicationMenu.
	//
	// +optional
	ApplicationMenu *ApplicationMenuSpec `json:"applicationMenu,omitempty"`
}

// ApplicationMenuSpec is the specification of the desired section and icon used for the link in the application menu.
type ApplicationMenuSpec struct {
	// section is the section of the application menu in which the link should appear.
	Section string `json:"section"`
	// imageUrl is the URL for the icon used in front of the link in the application menu.
	// The URL must be an HTTPS URL or a Data URI. The image should be square and will be shown at 24x24 pixels.
	// +optional
	ImageURL string `json:"imageURL,omitempty"`
}

// ConsoleLinkLocationSelector is a set of possible menu targets to which a link may be appended.
type ConsoleLinkLocation string

const (
	// HelpMenu indicates that the link should appear in the help menu in the console.
	HelpMenu ConsoleLinkLocation = "HelpMenu"
	// UserMenu indicates that the link should appear in the user menu in the console.
	UserMenu ConsoleLinkLocation = "UserMenu"
	// ApplicationMenu indicates that the link should appear inside the application menu of the console.
	ApplicationMenu ConsoleLinkLocation = "ApplicationMenu"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleLinkList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []ConsoleLink `json:"items"`
}
