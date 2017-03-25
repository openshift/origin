package api

// annotation keys
const (
	// IconClassAnnotation is the rss class of an icon representing a template
	IconClassAnnotation = "iconClass"

	// LongDescriptionAnnotation is a template's long description
	LongDescriptionAnnotation = "template.openshift.io/long-description"

	// ProviderDisplayNameAnnotation is the name of a template provider, e.g.
	// "Red Hat, Inc."
	ProviderDisplayNameAnnotation = "template.openshift.io/provider-display-name"

	// DocumentationURLAnnotation is the url where documentation associated with
	// a template can be found
	DocumentationURLAnnotation = "template.openshift.io/documentation-url"

	// SupportURLAnnotation is the url where support for a template can be found
	SupportURLAnnotation = "template.openshift.io/support-url"

	// TemplateInstanceLabel is used to label every object created by the
	// TemplateInstance API.
	TemplateInstanceLabel = "template.openshift.io/template-instance"

	// NamespaceParameterKey is the name of the key in the Open Service Broker API
	// ProvisionRequest Parameters object where we receive the name of the
	// namespace into which a template should be provisioned.  The '/' and '.'
	// characters in the name happen to make this an invalid template parameter
	// name so there is no immediate overlap with passed template parameters in
	// the same object.
	NamespaceParameterKey = "template.openshift.io/namespace"

	// RequesterUsernameParameterKey is the name of the key in the Open Service
	// Broker API ProvisionRequest Parameters object where we receive the user
	// name which will be impersonated during template provisioning.  See above
	// note.
	RequesterUsernameParameterKey = "template.openshift.io/requester-username"

	// ServiceBrokerRoot is the API root of the template service broker.
	ServiceBrokerRoot = "/brokers/template.openshift.io"

	// ServiceMetadataIconClass is the key for the template iconClass as returned
	// in the services.metadata map from a service broker catalog response
	ServiceMetadataIconClass = "console.openshift.io/iconClass"
)
