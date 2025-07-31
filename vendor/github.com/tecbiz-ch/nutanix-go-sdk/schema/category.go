package schema

type CategoryKey struct {
	// api version
	APIVersion string `json:"api_version,omitempty"`

	// capabilities
	Capabilities *Capabilities `json:"capabilities,omitempty"`

	// Description of the category.
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// Name of the category.
	// Required: true
	// Max Length: 64
	Name string `json:"name"`
}

type Capabilities struct {
	// Cardinality of the category key.
	Cardinality *int64 `json:"cardinality,omitempty"`
}

type CategoryKeyList struct {
	// api version
	APIVersion *string `json:"api_version,omitempty"`

	// entities
	Entities []*CategoryKeyStatus `json:"entities"`

	// metadata
	Metadata *ListMetadata `json:"metadata,omitempty"`
}

type CategoryKeyStatus struct {

	// api version
	APIVersion string `json:"api_version,omitempty"`

	// capabilities
	Capabilities *Capabilities `json:"capabilities,omitempty"`

	// Description of the category.
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// Name of the category.
	// Required: true
	// Max Length: 64
	Name string `json:"name"`

	// Specifying whether its a system defined category.
	// Read Only: true
	SystemDefined bool `json:"system_defined,omitempty"`

	Values []string `json:"-,"`
}

// CategoryStatus represents The status of a REST API call. Only used when there is a failure to report.
type CategoryStatus struct {
	APIVersion *string `json:"api_version,omitempty"`

	// The HTTP error code.
	Code *int64 `json:"code,omitempty"`

	// The kind name
	Kind *string `json:"kind,omitempty"`

	MessageList []*MessageResource `json:"message_list,omitempty"`

	State *string `json:"state,omitempty"`
}

// CategoryValueList represents Category Value list response.
type CategoryValueList struct {
	APIVersion *string `json:"api_version,omitempty"`

	Entities []*CategoryValueStatus `json:"entities,omitempty"`

	Metadata *ListMetadata `json:"metadata,omitempty"`
}

// CategoryValueStatus represents Category value definition.
type CategoryValueStatus struct {

	// API version.
	APIVersion string `json:"api_version,omitempty"`

	// Description of the category value.
	Description string `json:"description,omitempty"`

	// The name of the category.
	Name string `json:"name,omitempty"`

	// Specifying whether its a system defined category.
	SystemDefined bool `json:"system_defined,omitempty"`

	// The value of the category.
	Value string `json:"value,omitempty"`
}

// CategoryFilter represents A category filter.
type CategoryFilter struct {

	// List of kinds associated with this filter.
	KindList []*string `json:"kind_list,omitempty"`

	// A list of category key and list of values.
	Params map[string][]string `json:"params,omitempty"`

	// The type of the filter being used.
	Type *string `json:"type,omitempty"`
}

// CategoryQueryInput represents Categories query input object.
type CategoryQueryInput struct {

	// API version.
	APIVersion *string `json:"api_version,omitempty"`

	CategoryFilter *CategoryFilter `json:"category_filter,omitempty"`

	// The maximum number of members to return per group.
	GroupMemberCount *int64 `json:"group_member_count,omitempty"`

	// The offset into the total member set to return per group.
	GroupMemberOffset *int64 `json:"group_member_offset,omitempty"`

	// TBD: USED_IN - to get policies in which specified categories are used. APPLIED_TO - to get entities attached to
	// specified categories.
	UsageType *string `json:"usage_type,omitempty"`
}

// CategoryQueryResponseMetadata represents Response metadata.
type CategoryQueryResponseMetadata struct {

	// The maximum number of records to return per group.
	GroupMemberCount *int64 `json:"group_member_count,omitempty"`

	// The offset into the total records set to return per group.
	GroupMemberOffset *int64 `json:"group_member_offset,omitempty"`

	// Total number of matched results.
	TotalMatches *int64 `json:"total_matches,omitempty"`

	// TBD: USED_IN - to get policies in which specified categories are used. APPLIED_TO - to get entities attached to specified categories.
	UsageType *string `json:"usage_type,omitempty"`
}

// EntityReference Reference to an entity.
type EntityReference struct {

	// Categories for the entity.
	Categories map[string]string `json:"categories,omitempty"`

	// Kind of the reference.
	Kind string `json:"kind,omitempty"`

	// Name of the entity.
	Name string `json:"name,omitempty"`

	// The type of filter being used. (Options : CATEGORIES_MATCH_ALL , CATEGORIES_MATCH_ANY)
	Type string `json:"type,omitempty"`

	// UUID of the entity.
	UUID string `json:"uuid,omitempty"`
}

// CategoryQueryResponseResults ...
type CategoryQueryResponseResults struct {

	// List of entity references.
	EntityAnyReferenceList []*EntityReference `json:"entity_any_reference_list,omitempty"`

	// Total number of filtered results.
	FilteredEntityCount int64 `json:"filtered_entity_count,omitempty"`

	// The entity kind.
	Kind string `json:"kind,omitempty"`

	// Total number of the matched results.
	TotalEntityCount int64 `json:"total_entity_count,omitempty"`
}

// CategoryQueryResponse represents Categories query response object.
type CategoryQueryResponse struct {

	// API version.
	APIVersion string `json:"api_version,omitempty"`

	Metadata *CategoryQueryResponseMetadata `json:"metadata,omitempty"`

	Results []*CategoryQueryResponseResults `json:"results,omitempty"`
}

// CategoryValue represents Category value definition.
type CategoryValue struct {

	// API version.
	APIVersion string `json:"api_version,omitempty"`

	// Description of the category value.
	Description string `json:"description,omitempty" `

	// Value for the category.
	Value string `json:"value,omitempty"`
}
