package schema

import (
	"time"
)

type IdempotenceIdentifiers struct {

	// The client identifier string.
	ClientIdentifier string `json:"client_identifier,omitempty"`

	// The number of idempotence identifiers provided.
	// Required: true
	Count *int64 `json:"count"`

	// UTC date and time in RFC-3339 format of the expiration time (with reference to system time). Value is creation time + valid_duration
	// Format: date-time
	ExpirationTime *time.Time `json:"expiration_time,omitempty"`

	// uuid list
	// Required: true
	UUIDList []string `json:"uuid_list"`
}

type IdempotenceIdentifiersInput struct {

	// The client identifier string.
	ClientIdentifier string `json:"client_identifier,omitempty"`

	// The number of idempotence identifiers provided.
	// Required: true
	// Maximum: 4096
	// Minimum: 1
	Count *int64 `json:"count"`

	// Number of minutes from creation time for which idempotence identifier uuid list is valid.
	// Maximum: 527040
	// Minimum: 1
	ValidDurationInMinutes int64 `json:"valid_duration_in_minutes,omitempty"`
}

// Address represents the Host address.
type Address struct {

	// Fully qualified domain name.
	FQDN string `json:"fqdn,omitempty"`

	// IPV4 address.
	IP string `json:"ip,omitempty"`

	// IPV6 address.
	IPV6 string `json:"ipv6,omitempty"`

	// Port Number
	Port int64 `json:"port,omitempty"`
}

// Reference ...
type Reference struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	UUID string `json:"uuid,omitempty"`
}

// Metadata Metadata The kind metadata
type Metadata struct {
	LastUpdateTime       *time.Time          `json:"last_update_time,omitempty"`
	Kind                 string              `json:"kind"`
	UUID                 string              `json:"uuid,omitempty"`
	ProjectReference     *Reference          `json:"project_reference,omitempty"`
	CreationTime         *time.Time          `json:"creation_time,omitempty"`
	SpecVersion          int64               `json:"spec_version"`
	SpecHash             string              `json:"spec_hash,omitempty"`
	OwnerReference       *Reference          `json:"owner_reference,omitempty"`
	Categories           map[string]string   `json:"categories,omitempty"`
	CategoriesMapping    map[string][]string `json:"categories_mapping,omitempty"`
	Name                 string              `json:"name,omitempty"`
	UseCategoriesMapping bool                `json:"use_categories_mapping,omitempty"`
}

// MessageResource ...
type MessageResource struct {

	// Custom key-value details relevant to the status.
	Details interface{} `json:"details,omitempty"`

	// If state is ERROR, a message describing the error.
	Message string `json:"message"`

	// If state is ERROR, a machine-readable snake-cased *string.
	Reason string `json:"reason"`
}

// ErrorResponse ...
type ErrorResponse struct {
	APIVersion  string            `json:"api_version,omitempty"`
	Code        int64             `json:"code,omitempty"`
	MessageList []MessageResource `json:"message_list"`
	State       string            `json:"state"`
}

// ExecutionContext ...
type ExecutionContext struct {
	TaskUUID interface{} `json:"task_uuid,omitempty"`
}

// DSMetadata All api calls that return a list will have this metadata block as input
type DSMetadata struct {

	// The filter in FIQL syntax used for the results.
	Filter string `json:"filter,omitempty"`

	// The kind name
	Kind string `json:"kind,omitempty"`

	// The number of records to retrieve relative to the offset
	Length *int64 `json:"length,omitempty"`

	// Offset from the start of the entity list
	Offset *int64 `json:"offset,omitempty"`

	// The attribute to perform sort on
	SortAttribute string `json:"sort_attribute,omitempty"`

	// The sort order in which results are returned
	SortOrder string `json:"sort_order,omitempty"`
}
