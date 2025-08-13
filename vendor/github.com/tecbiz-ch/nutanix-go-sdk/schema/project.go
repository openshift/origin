package schema

type ProjectListIntent struct {
	APIVersion string `json:"api_version,omitempty"`

	// entities
	Entities []*ProjectIntent `json:"entities,omitempty"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata,omitempty"`
}

type ProjectDefStatus struct {
	// Project description.
	Description string `json:"description,omitempty"`

	// message list
	MessageList []*MessageResource `json:"message_list"`

	// Project name.
	// Required: true
	Name *string `json:"name"`

	// resources
	// Required: true
	Resources *ProjectDefStatusResources `json:"resources"`

	// The state of the project entity.
	State string `json:"state,omitempty"`
}

type Project struct {
	// Project description.
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// Project name.
	// Required: true
	// Max Length: 64
	Name string `json:"name"`

	// resources
	// Required: true
	Resources *ProjectResources `json:"resources"`
}

type ProjectResources struct {

	// List of accounts associated with the project.
	AccountReferenceList []*Reference `json:"account_reference_list,omitempty"`

	// Optional default environment if one is specified
	DefaultEnvironmentReference *Reference `json:"default_environment_reference,omitempty"`

	// Optional default subnet if one is specified
	DefaultSubnetReference *Reference `json:"default_subnet_reference,omitempty"`

	// List of environments associated with the project.
	EnvironmentReferenceList []*Reference `json:"environment_reference_list,omitempty"`

	// List of external networks associated with the project.
	ExternalNetworkList []*ExternalNetwork `json:"external_network_list,omitempty"`

	// List of directory service user groups. These groups are not
	// managed by Nutanix.
	//
	ExternalUserGroupReferenceList []*Reference `json:"external_user_group_reference_list,omitempty"`

	// resource domain
	ResourceDomain *ResourceDomainSpec `json:"resource_domain,omitempty"`

	// List of subnets for the project.
	SubnetReferenceList []*Reference `json:"subnet_reference_list,omitempty"`

	// List of users in the project.
	UserReferenceList []*Reference `json:"user_reference_list,omitempty"`
}

type ExternalNetwork struct {
	// name
	Name string `json:"name,omitempty"`

	// uuid
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	UUID string `json:"uuid,omitempty"`
}

type ResourceDomainSpec struct {

	// The utilization limits for resource types
	Resources []*ResourceUtilizationSpec `json:"resources"`
}

type ResourceDomainResourcesStatus struct {
	// The utilization/limit for resource types
	// Required: true
	Resources []*ResourceUtilizationStatus `json:"resources"`
}

type ResourceUtilizationSpec struct {
	// The resource consumption limit
	Limit int64 `json:"limit,omitempty"`

	// The type of resource (i.e. storage, CPUs)
	// Required: true
	ResourceType string `json:"resource_type,omitempty"`
}

type ResourceUtilizationStatus struct {
	// The resource consumption limit (unspecified is unlimited)
	Limit int64 `json:"limit,omitempty"`

	// The type of resource (for example storage, CPUs)
	// Required: true
	ResourceType string `json:"resource_type,omitempty"`

	// The units of the resource type
	// Required: true
	Units string `json:"units,omitempty"`

	// The amount of resource consumed
	// Required: true
	Value int64 `json:"value,omitempty"`
}

type ProjectDefStatusResources struct {
	// List of accounts associated with the project.
	AccountReferenceList []*Reference `json:"account_reference_list,omitempty"`

	// Optional default environment if one is specified
	DefaultEnvironmentReference *Reference `json:"default_environment_reference,omitempty"`

	// Optional default subnet if one is specified
	DefaultSubnetReference *Reference `json:"default_subnet_reference,omitempty"`

	// List of environments associated with the project.
	EnvironmentReferenceList []*Reference `json:"environment_reference_list,omitempty"`

	// List of external network associated with the project.
	ExternalNetworkList []*ExternalNetwork `json:"external_network_list,omitempty"`

	// List of directory service groups reference. These
	// groups are not managed by Nutanix.
	//
	ExternalUserGroupReferenceList []*Reference `json:"external_user_group_reference_list,omitempty"`

	// Indicates if it is the default project. A default project is
	// managed by the system and cannot be renamed or removed.
	//
	IsDefault bool `json:"is_default,omitempty"`

	// resource domain
	ResourceDomain *ResourceDomainResourcesStatus `json:"resource_domain,omitempty"`

	// List of subnets for the project.
	SubnetReferenceList []*Reference `json:"subnet_reference_list,omitempty"`

	// List of users added directly to the project.
	//
	UserReferenceList []*Reference `json:"user_reference_list,omitempty"`
}

type ProjectIntent struct {
	// api version
	// Required: true
	APIVersion string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *Project `json:"spec,omitempty"`

	// status
	Status *ProjectDefStatus `json:"status,omitempty"`
}

type ProjectIntentRequest struct {
	// api version
	// Required: true
	APIVersion *string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *Project `json:"spec,omitempty"`
}
