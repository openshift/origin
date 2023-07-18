package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleSample is an extension to customizing OpenShift web console by adding samples.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ConsoleSample struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// spec contains configuration for a console sample.
	// +kubebuilder:validation:Required
	Spec ConsoleSampleSpec `json:"spec"`
}

// ConsoleSampleSpec is the desired sample for the web console.
// Samples will appear with their title, descriptions and a badge in a samples catalog.
type ConsoleSampleSpec struct {
	// title is the display name of the sample.
	//
	// It is required and must be no more than 50 characters in length.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	Title string `json:"title"`

	// abstract is a short introduction to the sample.
	//
	// It is required and must be no more than 100 characters in length.
	//
	// The abstract is shown on the sample card tile below the title and provider
	// and is limited to three lines of content.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=100
	Abstract string `json:"abstract"`

	// description is a long form explanation of the sample.
	//
	// It is required and can have a maximum length of **4096** characters.
	//
	// It is a README.md-like content for additional information, links, pre-conditions, and other instructions.
	// It will be rendered as Markdown so that it can contain line breaks, links, and other simple formatting.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=4096
	Description string `json:"description"`

	// icon is an optional base64 encoded image and shown beside the sample title.
	//
	// The format must follow the data: URL format and can have a maximum size of **10 KB**.
	//
	//   data:[<mediatype>][;base64],<base64 encoded image>
	//
	// For example:
	//
	//   data:image;base64,             plus the base64 encoded image.
	//
	// Vector images can also be used. SVG icons must start with:
	//
	//   data:image/svg+xml;base64,     plus the base64 encoded SVG image.
	//
	// All sample catalog icons will be shown on a white background (also when the dark theme is used).
	// The web console ensures that different aspect ratios work correctly.
	// Currently, the surface of the icon is at most 40x100px.
	//
	// For more information on the data URL format, please visit
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/Data_URLs.
	// +optional
	// +kubebuilder:validation:Pattern=`^data:([a-z/\.+0-9]*;(([-a-zA-Z0-9=])*;)?)?base64,`
	// +kubebuilder:validation:MaxLength=14000
	Icon string `json:"icon"`

	// type is an optional label to group multiple samples.
	//
	// It is optional and must be no more than 20 characters in length.
	//
	// Recommendation is a singular term like "Builder Image", "Devfile" or "Serverless Function".
	//
	// Currently, the type is shown a badge on the sample card tile in the top right corner.
	// +optional
	// +kubebuilder:validation:MaxLength=20
	Type string `json:"type"`

	// provider is an optional label to honor who provides the sample.
	//
	// It is optional and must be no more than 50 characters in length.
	//
	// A provider can be a company like "Red Hat" or an organization like "CNCF" or "Knative".
	//
	// Currently, the provider is only shown on the sample card tile below the title with the prefix "Provided by "
	// +optional
	// +kubebuilder:validation:MaxLength=50
	Provider string `json:"provider"`

	// tags are optional string values that can be used to find samples in the samples catalog.
	//
	// Examples of common tags may be "Java", "Quarkus", etc.
	//
	// They will be displayed on the samples details page.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems:=10
	Tags []string `json:"tags"`

	// source defines where to deploy the sample service from.
	// The sample may be sourced from an external git repository or container image.
	// +kubebuilder:validation:Required
	Source ConsoleSampleSource `json:"source"`
}

// ConsoleSampleSourceType is an enumeration of the supported sample types.
// Unsupported samples types will be ignored in the web console.
// +kubebuilder:validation:Enum:=GitImport;ContainerImport
type ConsoleSampleSourceType string

const (
	// A sample that let the user import code from a git repository.
	GitImport ConsoleSampleSourceType = "GitImport"
	// A sample that let the user import a container image.
	ContainerImport ConsoleSampleSourceType = "ContainerImport"
)

// ConsoleSampleSource is the actual sample definition and can hold different sample types.
// Unsupported sample types will be ignored in the web console.
// +union
// +kubebuilder:validation:XValidation:rule="self.type == 'GitImport' ? has(self.gitImport) : !has(self.gitImport)",message="source.gitImport is required when source.type is GitImport, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="self.type == 'ContainerImport' ? has(self.containerImport) : !has(self.containerImport)",message="source.containerImport is required when source.type is ContainerImport, and forbidden otherwise"
type ConsoleSampleSource struct {
	// type of the sample, currently supported: "GitImport";"ContainerImport"
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:="GitImport";"ContainerImport"
	Type ConsoleSampleSourceType `json:"type"`

	// gitImport allows the user to import code from a git repository.
	// +unionMember
	// +optional
	GitImport *ConsoleSampleGitImportSource `json:"gitImport,omitempty"`

	// containerImport allows the user import a container image.
	// +unionMember
	// +optional
	ContainerImport *ConsoleSampleContainerImportSource `json:"containerImport,omitempty"`
}

// ConsoleSampleGitImportSource let the user import code from a public Git repository.
type ConsoleSampleGitImportSource struct {
	// repository contains the reference to the actual Git repository.
	// +kubebuilder:validation:Required
	Repository ConsoleSampleGitImportSourceRepository `json:"repository"`
	// service contains configuration for the Service resource created for this sample.
	// +optional
	// +kubebuilder:default={"targetPort": 8080}
	// +default:={"targetPort": 8080}
	Service ConsoleSampleGitImportSourceService `json:"service"`
}

// ConsoleSampleGitImportSourceRepository let the user import code from a public git repository.
type ConsoleSampleGitImportSourceRepository struct {
	// url of the Git repository that contains a HTTP service.
	// The HTTP service must be exposed on the default port (8080) unless
	// otherwise configured with the port field.
	//
	// Only public repositories on GitHub, GitLab and Bitbucket are currently supported:
	//
	//   - https://github.com/<org>/<repository>
	//   - https://gitlab.com/<org>/<repository>
	//   - https://bitbucket.org/<org>/<repository>
	//
	// The url must have a maximum length of 256 characters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^https:\/\/(github.com|gitlab.com|bitbucket.org)\/[a-zA-Z0-9-]+\/[a-zA-Z0-9-]+(.git)?$`
	URL string `json:"url"`
	// revision is the git revision at which to clone the git repository
	// Can be used to clone a specific branch, tag or commit SHA.
	// Must be at most 256 characters in length.
	// When omitted the repository's default branch is used.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Revision string `json:"revision"`
	// contextDir is used to specify a directory within the repository to build the
	// component.
	// Must start with `/` and have a maximum length of 256 characters.
	// When omitted, the default value is to build from the root of the repository.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/`
	ContextDir string `json:"contextDir"`
}

// ConsoleSampleGitImportSourceService let the samples author define defaults
// for the Service created for this sample.
type ConsoleSampleGitImportSourceService struct {
	// targetPort is the port that the service listens on for HTTP requests.
	// This port will be used for Service created for this sample.
	// Port must be in the range 1 to 65535.
	// Default port is 8080.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +default:=8080
	TargetPort int32 `json:"targetPort,omitempty"`
}

// ConsoleSampleContainerImportSource let the user import a container image.
type ConsoleSampleContainerImportSource struct {
	// reference to a container image that provides a HTTP service.
	// The service must be exposed on the default port (8080) unless
	// otherwise configured with the port field.
	//
	// Supported formats:
	//   - <repository-name>/<image-name>
	//   - docker.io/<repository-name>/<image-name>
	//   - quay.io/<repository-name>/<image-name>
	//   - quay.io/<repository-name>/<image-name>@sha256:<image hash>
	//   - quay.io/<repository-name>/<image-name>:<tag>
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Image string `json:"image"`
	// service contains configuration for the Service resource created for this sample.
	// +optional
	// +kubebuilder:default={"targetPort": 8080}
	// +default:={"targetPort": 8080}
	Service ConsoleSampleContainerImportSourceService `json:"service"`
}

// ConsoleSampleContainerImportSourceService let the samples author define defaults
// for the Service created for this sample.
type ConsoleSampleContainerImportSourceService struct {
	// targetPort is the port that the service listens on for HTTP requests.
	// This port will be used for Service and Route created for this sample.
	// Port must be in the range 1 to 65535.
	// Default port is 8080.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +default:=8080
	TargetPort int32 `json:"targetPort,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type ConsoleSampleList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsoleSample `json:"items"`
}
