package v1

// AWSResourceReference is a reference to a specific AWS resource by ID, ARN, or filters.
// Only one of ID, ARN or Filters may be specified. Specifying more than one will result in
// a validation error.
// +union
// +kubebuilder:validation:XValidation:rule="has(self.type) && self.type == 'ID' ?  has(self.id) : !has(self.id)",message="id is required when type is ID, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.type) && self.type == 'ARN' ?  has(self.arn) : !has(self.arn)",message="arn is required when type is ARN, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.type) && self.type == 'Filters' ?  has(self.filters) : !has(self.filters)",message="filters is required when type is Filters, and forbidden otherwise"
type AWSResourceReference struct {
	// type determines how the reference will fetch the AWS resource.
	// +unionDiscriminator
	// +required
	Type AWSResourceReferenceType `json:"type"`
	// id of resource.
	// +optional
	ID *string `json:"id,omitempty"`
	// arn of resource.
	// +optional
	ARN *string `json:"arn,omitempty"`
	// filters is a set of filters used to identify a resource.
	// +optional
	// +listType=atomic
	Filters *[]AWSResourceFilter `json:"filters,omitempty"`
}

// AWSResourceReferenceType is an enumeration of different resource reference types.
// +kubebuilder:validation:Enum:="ID";"ARN";"Filters"
type AWSResourceReferenceType string

const (
	// AWSIDReferenceType is a resource reference based on the object ID.
	AWSIDReferenceType AWSResourceReferenceType = "ID"

	// AWSARNReferenceType is a resource reference based on the object ARN.
	AWSARNReferenceType AWSResourceReferenceType = "ARN"

	// AWSFiltersReferenceType is a resource reference based on filters.
	AWSFiltersReferenceType AWSResourceReferenceType = "Filters"
)

// AWSResourceFilter is a filter used to identify an AWS resource
type AWSResourceFilter struct {
	// name of the filter. Filter names are case-sensitive.
	// +required
	Name string `json:"name"`
	// values includes one or more filter values. Filter values are case-sensitive.
	// +optional
	// +listType=atomic
	Values []string `json:"values,omitempty"`
}
