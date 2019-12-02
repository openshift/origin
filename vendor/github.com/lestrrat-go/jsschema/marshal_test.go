package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lestrrat/go-jsschema"
	"github.com/lestrrat/go-jsschema/validator"
	"github.com/stretchr/testify/assert"
)

func TestMarshal(t *testing.T) {
	// roundTripSchemas are schemas that can be read and written back to the same
	// content. They also include an object that should match the schema to make
	// sure the schema is correctly written.
	var roundTripSchemas = []struct {
		Name       string
		Schema     string
		ValidValue interface{}
	}{{
		Name: "Integer",
		Schema: `{
  "type": "integer"
}`,
		ValidValue: int(0),
	}, {
		Name: "String",
		Schema: `{
  "type": "string"
}`,
		ValidValue: "value",
	}, {
		Name: "Object",
		Schema: `{
  "additionalProperties": false,
  "properties": {
    "attr": {
      "type": "integer"
    }
  },
  "type": "object"
}`,
		ValidValue: struct{ attr int }{10},
	}}
	for _, definition := range roundTripSchemas {
		t.Logf("Testing schema %s", definition.Name)
		s, err := schema.Read(strings.NewReader(definition.Schema))
		if !assert.NoError(t, err, "schema.Read should succeed") {
			return
		}
		v := validator.New(s)
		err = v.Validate(definition.ValidValue)
		if !assert.NoError(t, err, "ValidValue should Validate successfully") {
			return
		}
		output, err := json.MarshalIndent(s, "", "  ")
		if !assert.NoError(t, err, "json.Marshal should succeed") {
			return
		}
		if !assert.Equal(t, definition.Schema, string(output), "json.Marshal should result in the same value as the input.") {
			return
		}
	}
}
