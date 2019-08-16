package schema

import (
	"errors"
	"regexp"
	"sync"

	"github.com/lestrrat-go/jsref"
)

const (
	// SchemaURL contains the JSON Schema URL
	SchemaURL = `http://json-schema.org/draft-04/schema`
	// HyperSchemaURL contains the JSON Hyper Schema URL
	HyperSchemaURL = `http://json-schema.org/draft-03/hyper-schema`
	// MIMEType contains the MIME used for a JSON Schema
	MIMEType = "application/schema+json"
)

// ErrExpectedArrayOfString is returned when we encounter
// something other than array of strings
var ErrExpectedArrayOfString = errors.New("invalid value: expected array of string")
// ErrInvalidStringArray is the same as ErrExpectedArrayOfString.
// This is here only for backwards compatibility
var ErrInvalidStringArray = ErrExpectedArrayOfString

// PrimitiveType represents a JSON Schema primitive type such as
// "string", "integer", etc.
type PrimitiveType int

// PrimitiveTypes is a list of PrimitiveType
type PrimitiveTypes []PrimitiveType

// Format represents one of the pre-defined JSON Schema formats
type Format string

// The list of pre-defined JSON Schema formats
const (
	FormatDateTime Format = "date-time"
	FormatEmail    Format = "email"
	FormatHostname Format = "hostname"
	FormatIPv4     Format = "ipv4"
	FormatIPv6     Format = "ipv6"
	FormatURI      Format = "uri"
)

// Number represents a "number" value in a JSON Schema, such as
// "minimum", "maximum", etc.
type Number struct {
	Val         float64
	Initialized bool
}

// Integer represents a "integer" value in a JSON Schema, such as
// "minLength", "maxLength", etc.
type Integer struct {
	Val         int
	Initialized bool
}

// Bool represents a "boolean" value in a JSON Schema, such as
// "exclusiveMinimum", "exclusiveMaximum", etc.
type Bool struct {
	Val         bool
	Default     bool
	Initialized bool
}

// The list of primitive types
const (
	UnspecifiedType PrimitiveType = iota
	NullType
	IntegerType
	StringType
	ObjectType
	ArrayType
	BooleanType
	NumberType
)

// SchemaList is a list of Schemas
type SchemaList []*Schema

// Schema represents a JSON Schema object
type Schema struct {
	parent          *Schema
	resolveLock     sync.Mutex
	resolvedSchemas map[string]interface{}
	resolver        *jsref.Resolver
	ID              string             `json:"id,omitempty"`
	Title           string             `json:"title,omitempty"`
	Description     string             `json:"description,omitempty"`
	Default         interface{}        `json:"default,omitempty"`
	Type            PrimitiveTypes     `json:"type,omitempty"`
	SchemaRef       string             `json:"$schema,omitempty"`
	Definitions     map[string]*Schema `json:"definitions,omitempty"`
	Reference       string             `json:"$ref,omitempty"`
	Format          Format             `json:"format,omitempty"`

	// NumericValidations
	MultipleOf       Number `json:"multipleOf,omitempty"`
	Minimum          Number `json:"minimum,omitempty"`
	Maximum          Number `json:"maximum,omitempty"`
	ExclusiveMinimum Bool   `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum Bool   `json:"exclusiveMaximum,omitempty"`

	// StringValidation
	MaxLength Integer        `json:"maxLength,omitempty"`
	MinLength Integer        `json:"minLength,omitempty"`
	Pattern   *regexp.Regexp `json:"pattern,omitempty"`

	// ArrayValidations
	AdditionalItems *AdditionalItems
	Items           *ItemSpec
	MinItems        Integer
	MaxItems        Integer
	UniqueItems     Bool

	// ObjectValidations
	MaxProperties        Integer                    `json:"maxProperties,omitempty"`
	MinProperties        Integer                    `json:"minProperties,omitempty"`
	Required             []string                   `json:"required,omitempty"`
	Dependencies         DependencyMap              `json:"dependencies,omitempty"`
	Properties           map[string]*Schema         `json:"properties,omitempty"`
	AdditionalProperties *AdditionalProperties      `json:"additionalProperties,omitempty"`
	PatternProperties    map[*regexp.Regexp]*Schema `json:"patternProperties,omitempty"`

	Enum   []interface{}          `json:"enum,omitempty"`
	AllOf  SchemaList             `json:"allOf,omitempty"`
	AnyOf  SchemaList             `json:"anyOf,omitempty"`
	OneOf  SchemaList             `json:"oneOf,omitempty"`
	Not    *Schema                `json:"not,omitempty"`
	Extras map[string]interface{} `json:"-"`
}

// AdditionalItems represents schema for additonalItems
type AdditionalItems struct {
	*Schema
}

// AdditionalProperties represents schema for additonalProperties
type AdditionalProperties struct {
	*Schema
}

// DependencyMap contains the dependencies defined within this schema.
// for a given dependency name, you can have either a schema or a
// list of property names
type DependencyMap struct {
	Names   map[string][]string
	Schemas map[string]*Schema
}

// ItemSpec represents a specification for `item` field
type ItemSpec struct {
	TupleMode bool // If this is true, the positions mean something. if false, len(Schemas) should be 1, and we should apply the same schema validation to all elements
	Schemas   SchemaList
}
