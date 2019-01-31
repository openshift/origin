package annotations

// annotation keys
// NEVER ADD TO THIS LIST.  Annotations need to be owned in the API groups they are associated with, so these constants end
// up nested in an API group, not top level in the OpenShift namespace.  The items located here are examples of annotations
// claiming a global namespace key that have never achieved global reach.  In the future, names should be based on the
// consuming component.
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
)
