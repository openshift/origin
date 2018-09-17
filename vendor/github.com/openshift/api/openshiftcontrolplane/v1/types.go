package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftAPIServerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// provides the standard apiserver configuration
	configv1.GenericAPIServerConfig `json:",inline" protobuf:"bytes,1,opt,name=genericAPIServerConfig"`

	// imagePolicyConfig feeds the image policy admission plugin
	ImagePolicyConfig ImagePolicyConfig `json:"imagePolicyConfig" protobuf:"bytes,9,opt,name=imagePolicyConfig"`

	// projectConfig feeds an admission plugin
	ProjectConfig ProjectConfig `json:"projectConfig" protobuf:"bytes,10,opt,name=projectConfig"`

	// routingConfig holds information about routing and route generation
	RoutingConfig RoutingConfig `json:"routingConfig" protobuf:"bytes,11,opt,name=routingConfig"`

	// serviceAccountOAuthGrantMethod is used for determining client authorization for service account oauth client.
	// It must be either: deny, prompt, or ""
	ServiceAccountOAuthGrantMethod GrantHandlerType `json:"serviceAccountOAuthGrantMethod" protobuf:"bytes,12,opt,name=serviceAccountOAuthGrantMethod,casttype=GrantHandlerType"`

	// jenkinsPipelineConfig holds information about the default Jenkins template
	// used for JenkinsPipeline build strategy.
	// TODO this needs to become a normal plugin config
	JenkinsPipelineConfig JenkinsPipelineConfig `json:"jenkinsPipelineConfig" protobuf:"bytes,13,opt,name=jenkinsPipelineConfig"`

	// cloudProviderFile points to the cloud config file
	// TODO this needs to become a normal plugin config
	CloudProviderFile string `json:"cloudProviderFile" protobuf:"bytes,14,opt,name=cloudProviderFile"`

	// TODO this needs to be removed.
	APIServerArguments map[string][]string `json:"apiServerArguments" protobuf:"bytes,14,rep,name=apiServerArguments"`
}

type GrantHandlerType string

const (
	// GrantHandlerAuto auto-approves client authorization grant requests
	GrantHandlerAuto GrantHandlerType = "auto"
	// GrantHandlerPrompt prompts the user to approve new client authorization grant requests
	GrantHandlerPrompt GrantHandlerType = "prompt"
	// GrantHandlerDeny auto-denies client authorization grant requests
	GrantHandlerDeny GrantHandlerType = "deny"
)

// RoutingConfig holds the necessary configuration options for routing to subdomains
type RoutingConfig struct {
	// subdomain is the suffix appended to $service.$namespace. to form the default route hostname
	// DEPRECATED: This field is being replaced by routers setting their own defaults. This is the
	// "default" route.
	Subdomain string `json:"subdomain" protobuf:"bytes,1,opt,name=subdomain"`
}

type ImagePolicyConfig struct {
	// maxImagesBulkImportedPerRepository controls the number of images that are imported when a user
	// does a bulk import of a Docker repository. This number is set low to prevent users from
	// importing large numbers of images accidentally. Set -1 for no limit.
	MaxImagesBulkImportedPerRepository int `json:"maxImagesBulkImportedPerRepository" protobuf:"varint,1,opt,name=maxImagesBulkImportedPerRepository"`
	// allowedRegistriesForImport limits the docker registries that normal users may import
	// images from. Set this list to the registries that you trust to contain valid Docker
	// images and that you want applications to be able to import from. Users with
	// permission to create Images or ImageStreamMappings via the API are not affected by
	// this policy - typically only administrators or system integrations will have those
	// permissions.
	AllowedRegistriesForImport AllowedRegistries `json:"allowedRegistriesForImport" protobuf:"bytes,2,rep,name=allowedRegistriesForImport"`

	// internalRegistryHostname sets the hostname for the default internal image
	// registry. The value must be in "hostname[:port]" format.
	// For backward compatibility, users can still use OPENSHIFT_DEFAULT_REGISTRY
	// environment variable but this setting overrides the environment variable.
	InternalRegistryHostname string `json:"internalRegistryHostname" protobuf:"bytes,3,opt,name=internalRegistryHostname"`
	// externalRegistryHostname sets the hostname for the default external image
	// registry. The external hostname should be set only when the image registry
	// is exposed externally. The value is used in 'publicDockerImageRepository'
	// field in ImageStreams. The value must be in "hostname[:port]" format.
	ExternalRegistryHostname string `json:"externalRegistryHostname" protobuf:"bytes,4,opt,name=externalRegistryHostname"`

	// additionalTrustedCA is a path to a pem bundle file containing additional CAs that
	// should be trusted during imagestream import.
	AdditionalTrustedCA string `json:"additionalTrustedCA" protobuf:"bytes,5,opt,name=additionalTrustedCA"`
}

// AllowedRegistries represents a list of registries allowed for the image import.
type AllowedRegistries []RegistryLocation

// RegistryLocation contains a location of the registry specified by the registry domain
// name. The domain name might include wildcards, like '*' or '??'.
type RegistryLocation struct {
	// DomainName specifies a domain name for the registry
	// In case the registry use non-standard (80 or 443) port, the port should be included
	// in the domain name as well.
	DomainName string `json:"domainName" protobuf:"bytes,1,opt,name=domainName"`
	// Insecure indicates whether the registry is secure (https) or insecure (http)
	// By default (if not specified) the registry is assumed as secure.
	Insecure bool `json:"insecure,omitempty" protobuf:"varint,2,opt,name=insecure"`
}

type ProjectConfig struct {
	// defaultNodeSelector holds default project node label selector
	DefaultNodeSelector string `json:"defaultNodeSelector" protobuf:"bytes,1,opt,name=defaultNodeSelector"`

	// projectRequestMessage is the string presented to a user if they are unable to request a project via the projectrequest api endpoint
	ProjectRequestMessage string `json:"projectRequestMessage" protobuf:"bytes,2,opt,name=projectRequestMessage"`

	// projectRequestTemplate is the template to use for creating projects in response to projectrequest.
	// It is in the format namespace/template and it is optional.
	// If it is not specified, a default template is used.
	ProjectRequestTemplate string `json:"projectRequestTemplate" protobuf:"bytes,3,opt,name=projectRequestTemplate"`
}

// JenkinsPipelineConfig holds configuration for the Jenkins pipeline strategy
type JenkinsPipelineConfig struct {
	// autoProvisionEnabled determines whether a Jenkins server will be spawned from the provided
	// template when the first build config in the project with type JenkinsPipeline
	// is created. When not specified this option defaults to true.
	AutoProvisionEnabled *bool `json:"autoProvisionEnabled" protobuf:"varint,1,opt,name=autoProvisionEnabled"`
	// templateNamespace contains the namespace name where the Jenkins template is stored
	TemplateNamespace string `json:"templateNamespace" protobuf:"bytes,2,opt,name=templateNamespace"`
	// templateName is the name of the default Jenkins template
	TemplateName string `json:"templateName" protobuf:"bytes,3,opt,name=templateName"`
	// serviceName is the name of the Jenkins service OpenShift uses to detect
	// whether a Jenkins pipeline handler has already been installed in a project.
	// This value *must* match a service name in the provided template.
	ServiceName string `json:"serviceName" protobuf:"bytes,4,opt,name=serviceName"`
	// parameters specifies a set of optional parameters to the Jenkins template.
	Parameters map[string]string `json:"parameters" protobuf:"bytes,5,rep,name=parameters"`
}
