// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

func TestSchemaValidator_Validate_Pattern(t *testing.T) {
	var schemaJSON = `
{
    "properties": {
        "name": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        },
        "place": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        }
    },
    "required": [
        "name"
    ]
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]interface{}
	var inputJSON = `{"name": "Ivan"}`

	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	assert.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	input["place"] = json.Number("10")

	assert.Error(t, AgainstSchema(schema, input, strfmt.Default))

}

func TestSchemaValidator_PatternProperties(t *testing.T) {
	var schemaJSON = `
{
    "properties": {
        "name": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        }
	},
    "patternProperties": {
	  "address-[0-9]+": {
         "type": "string",
         "pattern": "^[\\s|a-z]+$"
	  }
    },
    "required": [
        "name"
    ],
	"additionalProperties": false
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]interface{}

	// ok
	var inputJSON = `{"name": "Ivan","address-1": "sesame street"}`
	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	assert.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	// fail pattern regexp
	input["address-1"] = "1, Sesame Street"
	assert.Error(t, AgainstSchema(schema, input, strfmt.Default))

	// fail patternProperties regexp
	inputJSON = `{"name": "Ivan","address-1": "sesame street","address-A": "address"}`
	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	assert.Error(t, AgainstSchema(schema, input, strfmt.Default))

}

func TestSchemaValidator_Panic(t *testing.T) {
	assert.PanicsWithValue(t, `Invalid schema provided to SchemaValidator: object has no field "pointer-to-nowhere"`, schemaValidatorPanicker)
}

func schemaValidatorPanicker() {
	var schemaJSON = `
{
    "$ref": "#/pointer-to-nowhere"
}`

	schema := new(spec.Schema)
	_ = json.Unmarshal([]byte(schemaJSON), schema)

	var input map[string]interface{}

	// ok
	var inputJSON = `{"name": "Ivan","address-1": "sesame street"}`
	_ = json.Unmarshal([]byte(inputJSON), &input)
	// panics
	_ = AgainstSchema(schema, input, strfmt.Default)
}

// Test edge cases in schemaValidator which are difficult
// to simulate with specs
func TestSchemaValidator_EdgeCases(t *testing.T) {
	var s *SchemaValidator

	res := s.Validate("123")
	assert.NotNil(t, res)
	assert.True(t, res.IsValid())

	s = NewSchemaValidator(nil, nil, "", strfmt.Default)
	assert.Nil(t, s)

	v := "ABC"
	b := s.Applies(v, reflect.String)
	assert.False(t, b)

	sp := spec.Schema{}
	b = s.Applies(&sp, reflect.Struct)
	assert.True(t, b)

	spp := spec.Float64Property()

	s = NewSchemaValidator(spp, nil, "", strfmt.Default)

	s.SetPath("path")
	assert.Equal(t, "path", s.Path)

	r := s.Validate(nil)
	assert.NotNil(t, r)
	assert.False(t, r.IsValid())

	// Validating json.Number data against number|float64
	j := json.Number("123")
	r = s.Validate(j)
	assert.True(t, r.IsValid())

	// Validating json.Number data against integer|int32
	spp = spec.Int32Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("123")
	r = s.Validate(j)
	assert.True(t, r.IsValid())

	bignum := swag.FormatFloat64(math.MaxFloat64)
	j = json.Number(bignum)
	r = s.Validate(j)
	assert.False(t, r.IsValid())

	// Validating incorrect json.Number data
	spp = spec.Float64Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("AXF")
	r = s.Validate(j)
	assert.False(t, r.IsValid())
}

func TestSchemaValidator_SchemaOptions(t *testing.T) {
	var schemaJSON = `
{
	"properties": {
		"spec": {
			"properties": {
				"replicas": {
					"type": "integer"
				}
			}
		}
	}
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]interface{}
	var inputJSON = `{"spec": {"items": ["foo", "bar"], "replicas": 1}}`
	assert.NoError(t, json.Unmarshal([]byte(inputJSON), &input))

	// ok
	s := NewSchemaValidator(schema, nil, "", strfmt.Default, EnableObjectArrayTypeCheck(false))
	result := s.Validate(input)
	assert.True(t, result.IsValid())

	// fail
	s = NewSchemaValidator(schema, nil, "", strfmt.Default, EnableObjectArrayTypeCheck(true))
	result = s.Validate(input)
	assert.False(t, result.IsValid())
}

func TestSchemaValidator_TypeArray_Issue83(t *testing.T) {
	var schemaJSON = `
{
	"type": "object"
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]interface{}
	var inputJSON = `{"type": "array"}`

	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	// default behavior: jsonschema
	assert.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	// swagger behavior
	assert.Error(t, AgainstSchema(schema, input, strfmt.Default, SwaggerSchema(true)))
}
