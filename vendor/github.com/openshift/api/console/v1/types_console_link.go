package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleLink is an extension for customizing OpenShift web console links.
type ConsoleLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConsoleLinkSpec `json:"spec"`
}

// ConsoleLinkSpec is the desired console link configuration.
type ConsoleLinkSpec struct {
	Link `json:",inline"`
	// location determines which location in the console the link will be appended to (ApplicationMenu, HelpMenu, UserMenu, NamespaceDashboard).
	Location ConsoleLinkLocation `json:"location"`
	// applicationMenu holds information about section and icon used for the link in the
	// application menu, and it is applicable only when location is set to ApplicationMenu.
	//
	// +optional
	ApplicationMenu *ApplicationMenuSpec `json:"applicationMenu,omitempty"`
	// namespaceDashboard holds information about namespaces in which the dashboard link should
	// appear, and it is applicable only when location is set to NamespaceDashboard.
	// If not specified, the link will appear in all namespaces.
	//
	// +optional
	NamespaceDashboard *NamespaceDashboardSpec `json:"namespaceDashboard,omitempty"`
}

// ApplicationMenuSpec is the specification of the desired section and icon used for the link in the application menu.
type ApplicationMenuSpec struct {
	// section is the section of the application menu in which the link should appear.
	// This can be any text that will appear as a subheading in the application menu dropdown.
	// A new section will be created if the text does not match text of an existing section.
	Section string `json:"section"`
	// imageUrl is the URL for the icon used in front of the link in the application menu.
	// The URL must be an HTTPS URL or a Data URI. The image should be square and will be shown at 24x24 pixels.
	// +optional
	ImageURL string `json:"imageURL,omitempty"`
}

// NamespaceDashboardSpec is a specification of namespaces in which the dashboard link should appear.
// If both namespaces and namespaceSelector are specified, the link will appear in namespaces that match either
type NamespaceDashboardSpec struct {
	// namespaces is an array of namespace names in which the dashboard link should appear.
	//
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
	// namespaceSelector is used to select the Namespaces that should contain dashboard link by label.
	// If the namespace labels match, dashboard link will be shown for the namespaces.
	//
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// ConsoleLinkLocationSelector is a set of possible menu targets to which a link may be appended.
// +kubebuilder:validation:Pattern=`^(ApplicationMenu|HelpMenu|UserMenu|NamespaceDashboard)$`
type ConsoleLinkLocation string

const (
	// HelpMenu indicates that the link should appear in the help menu in the console.
	HelpMenu ConsoleLinkLocation = "HelpMenu"
	// UserMenu indicates that the link should appear in the user menu in the console.
	UserMenu ConsoleLinkLocation = "UserMenu"
	// ApplicationMenu indicates that the link should appear inside the application menu of the console.
	ApplicationMenu ConsoleLinkLocation = "ApplicationMenu"
	// NamespaceDashboard indicates that the link should appear in the namespaced dashboard of the console.
	NamespaceDashboard ConsoleLinkLocation = "NamespaceDashboard"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleLinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ConsoleLink `json:"items"`
}
