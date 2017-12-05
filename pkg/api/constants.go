package api

// annotation keys
const (
	// OpenShiftDisplayName is a common, optional annotation that stores the name displayed by a UI when referencing a resource.
	OpenShiftDisplayName = "openshift.io/display-name"

	// OpenShiftProviderDisplayNameAnnotation is the name of a provider of a resource, e.g.
	// "Red Hat, Inc."
	OpenShiftProviderDisplayNameAnnotation = "openshift.io/provider-display-name"

	// OpenShiftDocumentationURLAnnotation is the url where documentation associated with
	// a resource can be found.
	OpenShiftDocumentationURLAnnotation = "openshift.io/documentation-url"

	// OpenShiftSupportURLAnnotation is the url where support for a template can be found.
	OpenShiftSupportURLAnnotation = "openshift.io/support-url"

	// OpenShiftDescription is a common, optional annotation that stores the description for a resource.
	OpenShiftDescription = "openshift.io/description"

	// OpenShiftLongDescriptionAnnotation is a resource's long description
	OpenShiftLongDescriptionAnnotation = "openshift.io/long-description"

	// DeprecatedKubeCreatedByAnnotation was removed by https://github.com/kubernetes/kubernetes/pull/54445 (liggitt approved).
	// TODO need to figure out who and how this affects.
	DeprecatedKubeCreatedByAnnotation = "kubernetes.io/created-by"
)
