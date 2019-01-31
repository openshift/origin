package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Console holds cluster-wide information about Console.  The canonical name is `cluster`
type Console struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec ConsoleSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status ConsoleStatus `json:"status"`
}

type ConsoleSpec struct {
	// +optional
	Authentication ConsoleAuthentication `json:"authentication,omitempty"`
}

type ConsoleStatus struct {
	// The hostname for the console. This will match the host for the route that
	// is created for the console.
	PublicHostname string `json:"publicHostname"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Console `json:"items"`
}

type ConsoleAuthentication struct {
	// An optional, absolute URL to redirect web browsers to after logging out of
	// the console. If not specified, it will redirect to the default login page.
	// This is required when using an identity provider that supports single
	// sign-on (SSO) such as:
	// - OpenID (Keycloak, Azure)
	// - RequestHeader (GSSAPI, SSPI, SAML)
	// - OAuth (GitHub, GitLab, Google)
	// Logging out of the console will destroy the user's token. The logoutRedirect
	// provides the user the option to perform single logout (SLO) through the identity
	// provider to destroy their single sign-on session.
	// +optional
	LogoutRedirect string `json:"logoutRedirect,omitempty"`
}
