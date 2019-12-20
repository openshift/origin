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

package analysis

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveDefinition(t *testing.T) {
	sp := &spec.Swagger{}
	saveSchema(sp, "theName", spec.StringProperty())
	assert.Contains(t, sp.Definitions, "theName")
}

func TestNameFromRef(t *testing.T) {
	values := []struct{ Source, Expected string }{
		{"#/definitions/errorModel", "errorModel"},
		{"http://somewhere.com/definitions/errorModel", "errorModel"},
		{"http://somewhere.com/definitions/errorModel.json", "errorModel"},
		{"/definitions/errorModel", "errorModel"},
		{"/definitions/errorModel.json", "errorModel"},
		{"http://somewhere.com", "somewhereCom"},
		{"#", ""},
	}

	for _, v := range values {
		assert.Equal(t, v.Expected, nameFromRef(spec.MustCreateRef(v.Source)))
	}
}

func TestDefinitionName(t *testing.T) {
	values := []struct {
		Source, Expected string
		Definitions      spec.Definitions
	}{
		{"#/definitions/errorModel", "errorModel", map[string]spec.Schema(nil)},
		{"http://somewhere.com/definitions/errorModel", "errorModel", map[string]spec.Schema(nil)},
		{"#/definitions/errorModel", "errorModel", map[string]spec.Schema{"apples": *spec.StringProperty()}},
		{"#/definitions/errorModel", "errorModelOAIGen", map[string]spec.Schema{"errorModel": *spec.StringProperty()}},
		{"#/definitions/errorModel", "errorModelOAIGen1",
			map[string]spec.Schema{"errorModel": *spec.StringProperty(), "errorModelOAIGen": *spec.StringProperty()}},
		{"#", "oaiGen", nil},
	}

	for _, v := range values {
		u, _ := uniqifyName(v.Definitions, nameFromRef(spec.MustCreateRef(v.Source)))
		assert.Equal(t, v.Expected, u)
	}
}

var refFixture = []struct {
	Key string
	Ref spec.Ref
}{
	{"#/parameters/someParam/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/paths/~1some~1where~1{id}/parameters/1/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/responses/someResponse/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/paths/~1some~1where~1{id}/get/responses/default/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/paths/~1some~1where~1{id}/get/responses/200/schema", spec.MustCreateRef("#/definitions/record")},
	{"#/definitions/namedAgain", spec.MustCreateRef("#/definitions/named")},
	{"#/definitions/datedTag/allOf/1", spec.MustCreateRef("#/definitions/tag")},
	{"#/definitions/datedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
	{"#/definitions/datedTaggedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
	{"#/definitions/datedTaggedRecords/additionalItems", spec.MustCreateRef("#/definitions/tag")},
	{"#/definitions/otherRecords/items", spec.MustCreateRef("#/definitions/record")},
	{"#/definitions/tags/additionalProperties", spec.MustCreateRef("#/definitions/tag")},
	{"#/definitions/namedThing/properties/name", spec.MustCreateRef("#/definitions/named")},
}

func TestUpdateRef(t *testing.T) {
	bp := filepath.Join("fixtures", "external_definitions.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		for _, v := range refFixture {
			err := updateRef(sp, v.Key, v.Ref)
			if assert.NoError(t, err) {
				ptr, err := jsonpointer.New(v.Key[1:])
				if assert.NoError(t, err) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String())
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String())
						case *spec.SchemaOrBool:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String())
						case *spec.SchemaOrArray:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String())
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}
	}
}

func TestImportExternalReferences(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	// this fixture is the same as external_definitions.yml, but no more
	// checks if invalid construct is supported (i.e. $ref in parameters items)
	bp := filepath.Join(".", "fixtures", "external_definitions_valid.yml")
	sp, err := loadSpec(bp)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	opts := &FlattenOpts{
		Spec:     New(sp),
		BasePath: bp,
	}
	// NOTE(fredbi): now we no more expand, but merely resolve and iterate until there is no more ext ref
	// so calling importExternalReferences is no more idempotent
	_, err = importExternalReferences(opts)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	for i, v := range refFixture {
		// there is 1 notable difference with the updateRef test:
		if i == 5 {
			v.Ref = spec.MustCreateRef("#/definitions/tag")
		}

		ptr, erj := jsonpointer.New(v.Key[1:])
		if assert.NoErrorf(t, erj, "error on jsonpointer.New(%q)", v.Key[1:]) {
			vv, _, erg := ptr.Get(sp)
			if assert.NoErrorf(t, erg, "error on ptr.Get(p for key=%s)", v.Key[1:]) {
				switch tv := vv.(type) {
				case *spec.Schema:
					assert.Equal(t, v.Ref.String(), tv.Ref.String(), "for %s", v.Key)
				case spec.Schema:
					assert.Equal(t, v.Ref.String(), tv.Ref.String(), "for %s", v.Key)
				case *spec.SchemaOrBool:
					assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "for %s", v.Key)
				case *spec.SchemaOrArray:
					assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "for %s", v.Key)
				default:
					assert.Fail(t, "unknown type", "got %T", vv)
				}
			}
		}
		assert.Len(t, sp.Definitions, 11)
		assert.Contains(t, sp.Definitions, "tag")
		assert.Contains(t, sp.Definitions, "named")
		assert.Contains(t, sp.Definitions, "record")
	}

	// check the complete result for clarity
	bb, _ := json.MarshalIndent(sp, "", " ")
	assert.JSONEq(t, `{
         "swagger": "2.0",
         "info": {
          "title": "reference analysis",
          "version": "0.1.0"
         },
         "paths": {
          "/other/place": {
           "$ref": "external/pathItem.yml"
          },
          "/some/where/{id}": {
           "get": {
            "parameters": [
             {
              "$ref": "external/parameters.yml#/parameters/limitParam"
             },
             {
              "type": "array",
              "items": {
               "type": "string"
              },
              "name": "other",
              "in": "query"
             },
             {
              "name": "body",
              "in": "body",
              "schema": {
               "$ref": "#/definitions/record"
              }
             }
            ],
            "responses": {
             "200": {
              "schema": {
               "$ref": "#/definitions/tag"
              }
             },
             "404": {
              "$ref": "external/responses.yml#/responses/notFound"
             },
             "default": {
              "schema": {
               "$ref": "#/definitions/record"
              }
             }
            }
           },
           "parameters": [
            {
             "$ref": "external/parameters.yml#/parameters/idParam"
            },
            {
             "name": "bodyId",
             "in": "body",
             "schema": {
              "$ref": "#/definitions/record"
             }
            }
           ]
          }
         },
         "definitions": {
          "datedRecords": {
           "type": "array",
           "items": [
            {
             "type": "string",
             "format": "date-time"
            },
            {
             "$ref": "#/definitions/record"
            }
           ]
          },
          "datedTag": {
           "allOf": [
            {
             "type": "string",
             "format": "date"
            },
            {
             "$ref": "#/definitions/tag"
            }
           ]
          },
          "datedTaggedRecords": {
           "type": "array",
           "items": [
            {
             "type": "string",
             "format": "date-time"
            },
            {
             "$ref": "#/definitions/record"
            }
           ],
           "additionalItems": {
            "$ref": "#/definitions/tag"
           }
          },
          "named": {
           "type": "string"
          },
          "namedAgain": {
           "$ref": "#/definitions/named"
          },
          "namedThing": {
           "type": "object",
           "properties": {
            "name": {
             "$ref": "#/definitions/named"
            }
           }
          },
          "otherRecords": {
           "type": "array",
           "items": {
            "$ref": "#/definitions/record"
           }
          },
          "record": {
           "type": "object",
           "properties": {
            "createdAt": {
             "type": "string",
             "format": "date-time"
            }
           }
          },
          "records": {
           "type": "array",
           "items": [
            {
             "$ref": "#/definitions/record"
            }
           ]
          },
          "tag": {
           "type": "object",
           "properties": {
            "audit": {
             "$ref": "external/definitions.yml#/definitions/record"
            },
            "id": {
             "type": "integer",
             "format": "int64"
            },
            "value": {
             "type": "string"
            }
           }
          },
          "tags": {
           "type": "object",
           "additionalProperties": {
            "$ref": "#/definitions/tag"
           }
          }
         },
         "parameters": {
          "someParam": {
           "name": "someParam",
           "in": "body",
           "schema": {
            "$ref": "#/definitions/record"
           }
          }
         },
         "responses": {
          "someResponse": {
           "schema": {
            "$ref": "#/definitions/record"
           }
          }
         }
        }`, string(bb))

	// iterate again: this time all external schema $ref's should be reinlined
	opts.Spec.reload()
	_, err = importExternalReferences(&FlattenOpts{
		Spec:     New(sp),
		BasePath: bp,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	opts.Spec.reload()
	for _, ref := range opts.Spec.references.schemas {
		assert.True(t, ref.HasFragmentOnly)
	}

	// now try complete flatten
	sp = loadOrFail(t, bp)
	an := New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: true})
	assert.NoError(t, err)

	bbb, _ := json.MarshalIndent(an.spec, "", " ")
	//t.Logf("%s", string(bbb))
	assert.JSONEq(t, `{
		"swagger": "2.0",
         "info": {
          "title": "reference analysis",
          "version": "0.1.0"
         },
         "paths": {
          "/other/place": {
           "get": {
            "description": "Used to see if a codegen can render all the possible parameter variations for a header param",
            "tags": [
             "testcgen"
            ],
            "summary": "many model variations",
            "operationId": "modelOp",
            "responses": {
             "default": {
              "description": "Generic Out"
             }
            }
           }
          },
          "/some/where/{id}": {
           "get": {
            "parameters": [
             {
              "type": "integer",
              "format": "int32",
              "name": "limit",
              "in": "query"
             },
             {
              "type": "array",
              "items": {
               "type": "string"
              },
              "name": "other",
              "in": "query"
             },
             {
              "name": "body",
              "in": "body",
              "schema": {
               "$ref": "#/definitions/record"
              }
             }
            ],
            "responses": {
             "200": {
              "schema": {
               "$ref": "#/definitions/tag"
              }
             },
             "404": {
              "schema": {
               "$ref": "#/definitions/error"
              }
             },
             "default": {
              "schema": {
               "$ref": "#/definitions/record"
              }
             }
            }
           },
           "parameters": [
            {
             "type": "integer",
             "format": "int32",
             "name": "id",
             "in": "path"
            },
            {
             "name": "bodyId",
             "in": "body",
             "schema": {
              "$ref": "#/definitions/record"
             }
            }
           ]
          }
         },
         "definitions": {
          "error": {
           "type": "object",
           "required": [
            "id",
            "message"
           ],
           "properties": {
            "id": {
             "type": "integer",
             "format": "int64",
             "readOnly": true
            },
            "message": {
             "type": "string",
             "readOnly": true
            }
           }
          },
          "named": {
           "type": "string"
          },
          "record": {
           "type": "object",
           "properties": {
            "createdAt": {
             "type": "string",
             "format": "date-time"
            }
           }
          },
          "tag": {
           "type": "object",
           "properties": {
            "audit": {
             "$ref": "#/definitions/record"
            },
            "id": {
             "type": "integer",
             "format": "int64"
            },
            "value": {
             "type": "string"
            }
           }
          }
         }
        }`, string(bbb))
}

func TestRewriteSchemaRef(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		for i, v := range refFixture {
			err := rewriteSchemaToRef(sp, v.Key, v.Ref)
			if assert.NoError(t, err) {
				ptr, err := jsonpointer.New(v.Key[1:])
				if assert.NoError(t, err) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrBool:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrArray:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "at %d for %s", i, v.Key)
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}
	}
}

func TestSplitKey(t *testing.T) {

	type KeyFlag uint64

	const (
		isOperation KeyFlag = 1 << iota
		isDefinition
		isSharedOperationParam
		isOperationParam
		isOperationResponse
		isDefaultResponse
		isStatusCodeResponse
	)

	values := []struct {
		Key         string
		Flags       KeyFlag
		PathItemRef spec.Ref
		PathRef     spec.Ref
		Name        string
	}{
		{
			"#/paths/~1some~1where~1{id}/parameters/1/schema",
			isOperation | isSharedOperationParam,
			spec.Ref{},
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/2/schema",
			isOperation | isOperationParam,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/default/schema",
			isOperation | isOperationResponse | isDefaultResponse,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"Default",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			isOperation | isOperationResponse | isStatusCodeResponse,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"OK",
		},
		{
			"#/definitions/namedAgain",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"namedAgain",
		},
		{
			"#/definitions/datedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedRecords",
		},
		{
			"#/definitions/datedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedRecords",
		},
		{
			"#/definitions/datedTaggedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedTaggedRecords",
		},
		{
			"#/definitions/datedTaggedRecords/additionalItems",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedTaggedRecords",
		},
		{
			"#/definitions/otherRecords/items",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"otherRecords",
		},
		{
			"#/definitions/tags/additionalProperties",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"tags",
		},
		{
			"#/definitions/namedThing/properties/name",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"namedThing",
		},
	}

	for i, v := range values {
		parts := keyParts(v.Key)
		pref := parts.PathRef()
		piref := parts.PathItemRef()
		assert.Equal(t, v.PathRef.String(), pref.String(), "pathRef: %s at %d", v.Key, i)
		assert.Equal(t, v.PathItemRef.String(), piref.String(), "pathItemRef: %s at %d", v.Key, i)

		if v.Flags&isOperation != 0 {
			assert.True(t, parts.IsOperation(), "isOperation: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperation(), "isOperation: %s at %d", v.Key, i)
		}
		if v.Flags&isDefinition != 0 {
			assert.True(t, parts.IsDefinition(), "isDefinition: %s at %d", v.Key, i)
			assert.Equal(t, v.Name, parts.DefinitionName(), "definition name: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsDefinition(), "isDefinition: %s at %d", v.Key, i)
			if v.Name != "" {
				assert.Equal(t, v.Name, parts.ResponseName(), "response name: %s at %d", v.Key, i)
			}
		}
		if v.Flags&isOperationParam != 0 {
			assert.True(t, parts.IsOperationParam(), "isOperationParam: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperationParam(), "isOperationParam: %s at %d", v.Key, i)
		}
		if v.Flags&isSharedOperationParam != 0 {
			assert.True(t, parts.IsSharedOperationParam(), "isSharedOperationParam: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsSharedOperationParam(), "isSharedOperationParam: %s at %d", v.Key, i)
		}
		if v.Flags&isOperationResponse != 0 {
			assert.True(t, parts.IsOperationResponse(), "isOperationResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperationResponse(), "isOperationResponse: %s at %d", v.Key, i)
		}
		if v.Flags&isDefaultResponse != 0 {
			assert.True(t, parts.IsDefaultResponse(), "isDefaultResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsDefaultResponse(), "isDefaultResponse: %s at %d", v.Key, i)
		}
		if v.Flags&isStatusCodeResponse != 0 {
			assert.True(t, parts.IsStatusCodeResponse(), "isStatusCodeResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsStatusCodeResponse(), "isStatusCodeResponse: %s at %d", v.Key, i)
		}
	}
}

func definitionPtr(key string) string {
	if !strings.HasPrefix(key, "#/definitions") {
		return key
	}
	return strings.Join(strings.Split(key, "/")[:3], "/")
}

func TestNamesFromKey(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		values := []struct {
			Key   string
			Names []string
		}{
			{"#/paths/~1some~1where~1{id}/parameters/1/schema",
				[]string{"GetSomeWhereID params body", "PostSomeWhereID params body"}},
			{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", []string{"GetSomeWhereID params body"}},
			{"#/paths/~1some~1where~1{id}/get/responses/default/schema", []string{"GetSomeWhereID Default body"}},
			{"#/paths/~1some~1where~1{id}/get/responses/200/schema", []string{"GetSomeWhereID OK body"}},
			{"#/definitions/namedAgain", []string{"namedAgain"}},
			{"#/definitions/datedTag/allOf/1", []string{"datedTag allOf 1"}},
			{"#/definitions/datedRecords/items/1", []string{"datedRecords tuple 1"}},
			{"#/definitions/datedTaggedRecords/items/1", []string{"datedTaggedRecords tuple 1"}},
			{"#/definitions/datedTaggedRecords/additionalItems", []string{"datedTaggedRecords tuple additionalItems"}},
			{"#/definitions/otherRecords/items", []string{"otherRecords items"}},
			{"#/definitions/tags/additionalProperties", []string{"tags additionalProperties"}},
			{"#/definitions/namedThing/properties/name", []string{"namedThing name"}},
		}

		for i, v := range values {
			ptr, err := jsonpointer.New(definitionPtr(v.Key)[1:])
			if assert.NoError(t, err) {
				vv, _, err := ptr.Get(sp)
				if assert.NoError(t, err) {
					switch tv := vv.(type) {
					case *spec.Schema:
						aschema, err := Schema(SchemaOpts{Schema: tv, Root: sp, BasePath: bp})
						if assert.NoError(t, err) {
							names := namesFromKey(keyParts(v.Key), aschema, opRefsByRef(gatherOperations(New(sp), nil)))
							assert.Equal(t, v.Names, names, "for %s at %d", v.Key, i)
						}
					case spec.Schema:
						aschema, err := Schema(SchemaOpts{Schema: &tv, Root: sp, BasePath: bp})
						if assert.NoError(t, err) {
							names := namesFromKey(keyParts(v.Key), aschema, opRefsByRef(gatherOperations(New(sp), nil)))
							assert.Equal(t, v.Names, names, "for %s at %d", v.Key, i)
						}
					default:
						assert.Fail(t, "unknown type", "got %T", vv)
					}
				}
			}
		}
	}
}

func TestDepthFirstSort(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	values := []string{
		// Added shared parameters and responses
		"#/parameters/someParam/schema/properties/createdAt",
		"#/parameters/someParam/schema",
		"#/responses/someResponse/schema/properties/createdAt",
		"#/responses/someResponse/schema",
		"#/paths/~1some~1where~1{id}/parameters/1/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/parameters/1/schema",
		"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/get/parameters/2/schema",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/id",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/value",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema",
		"#/paths/~1some~1where~1{id}/get/responses/404/schema",
		"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/get/responses/default/schema",
		"#/definitions/datedRecords/items/1/properties/createdAt",
		"#/definitions/datedTaggedRecords/items/1/properties/createdAt",
		"#/definitions/namedThing/properties/name/properties/id",
		"#/definitions/records/items/0/properties/createdAt",
		"#/definitions/datedTaggedRecords/additionalItems/properties/id",
		"#/definitions/datedTaggedRecords/additionalItems/properties/value",
		"#/definitions/otherRecords/items/properties/createdAt",
		"#/definitions/tags/additionalProperties/properties/id",
		"#/definitions/tags/additionalProperties/properties/value",
		"#/definitions/datedRecords/items/0",
		"#/definitions/datedRecords/items/1",
		"#/definitions/datedTag/allOf/0",
		"#/definitions/datedTag/allOf/1",
		"#/definitions/datedTag/properties/id",
		"#/definitions/datedTag/properties/value",
		"#/definitions/datedTaggedRecords/items/0",
		"#/definitions/datedTaggedRecords/items/1",
		"#/definitions/namedAgain/properties/id",
		"#/definitions/namedThing/properties/name",
		"#/definitions/pneumonoultramicroscopicsilicovolcanoconiosisAntidisestablishmentarianism/properties/" +
			"floccinaucinihilipilificationCreatedAt",
		"#/definitions/records/items/0",
		"#/definitions/datedTaggedRecords/additionalItems",
		"#/definitions/otherRecords/items",
		"#/definitions/tags/additionalProperties",
		"#/definitions/datedRecords",
		"#/definitions/datedTag",
		"#/definitions/datedTaggedRecords",
		"#/definitions/namedAgain",
		"#/definitions/namedThing",
		"#/definitions/otherRecords",
		"#/definitions/pneumonoultramicroscopicsilicovolcanoconiosisAntidisestablishmentarianism",
		"#/definitions/records",
		"#/definitions/tags",
	}
	if assert.NoError(t, err) {
		a := New(sp)
		result := sortDepthFirst(a.allSchemas)
		assert.Equal(t, values, result)
	}
}

func TestBuildNameWithReservedKeyWord(t *testing.T) {
	s := splitKey([]string{"definitions", "fullview", "properties", "properties"})
	startIdx := 2
	segments := []string{"fullview"}
	newName := s.BuildName(segments, startIdx, nil)
	assert.Equal(t, "fullview properties", newName)
	s = splitKey([]string{"definitions", "fullview",
		"properties", "properties", "properties", "properties", "properties", "properties"})
	newName = s.BuildName(segments, startIdx, nil)
	assert.Equal(t, "fullview properties properties properties", newName)
}

func TestNameInlinedSchemas(t *testing.T) {
	values := []struct {
		Key      string
		Location string
		Ref      spec.Ref
	}{
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/2/properties/name",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2Name"),
		},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/1",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/1",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems1"),
		},

		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/2",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/2",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/2/properties/name",
			"#/definitions/getSomeWhereIdOKBodyRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems2Name"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/1",
			"#/definitions/getSomeWhereIdOKBodyRecord/items/1",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems1"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/2",
			"#/definitions/getSomeWhereIdOKBodyRecord/items/2",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems2"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record",
			"#/definitions/getSomeWhereIdOKBody/properties/record",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecord"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBody"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/2/properties/name",
			"#/definitions/getSomeWhereIdDefaultBodyRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems2Name"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/1",
			"#/definitions/getSomeWhereIdDefaultBodyRecord/items/1",
			spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems1"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/2",
			"#/definitions/getSomeWhereIdDefaultBodyRecord/items/2",
			spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems2"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record",
			"#/definitions/getSomeWhereIdDefaultBody/properties/record",
			spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecord"),
		},

		{"#/paths/~1some~1where~1{id}/get/responses/default/schema",
			"#/paths/~1some~1where~1{id}/get/responses/default/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBody"),
		},
		// maps:
		//{"#/definitions/nestedThing/properties/record/items/2/allOf/1/additionalProperties",
		//"#/definitions/nestedThingRecordItems2AllOf1/additionalProperties",
		//spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1AdditionalProperties"),
		// },

		//{"#/definitions/nestedThing/properties/record/items/2/allOf/1",
		//"#/definitions/nestedThingRecordItems2/allOf/1",
		//spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1"),
		//},
		{"#/definitions/nestedThing/properties/record/items/2/properties/name",
			"#/definitions/nestedThingRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2Name"),
		},

		{"#/definitions/nestedThing/properties/record/items/1",
			"#/definitions/nestedThingRecord/items/1",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems1"),
		},

		{"#/definitions/nestedThing/properties/record/items/2",
			"#/definitions/nestedThingRecord/items/2",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2"),
		},

		{"#/definitions/datedRecords/items/1",
			"#/definitions/datedRecords/items/1",
			spec.MustCreateRef("#/definitions/datedRecordsItems1"),
		},

		{"#/definitions/datedTaggedRecords/items/1",
			"#/definitions/datedTaggedRecords/items/1",
			spec.MustCreateRef("#/definitions/datedTaggedRecordsItems1"),
		},

		{"#/definitions/namedThing/properties/name",
			"#/definitions/namedThing/properties/name",
			spec.MustCreateRef("#/definitions/namedThingName"),
		},

		{"#/definitions/nestedThing/properties/record",
			"#/definitions/nestedThing/properties/record",
			spec.MustCreateRef("#/definitions/nestedThingRecord"),
		},

		{"#/definitions/records/items/0",
			"#/definitions/records/items/0",
			spec.MustCreateRef("#/definitions/recordsItems0"),
		},

		{"#/definitions/datedTaggedRecords/additionalItems",
			"#/definitions/datedTaggedRecords/additionalItems",
			spec.MustCreateRef("#/definitions/datedTaggedRecordsItemsAdditionalItems"),
		},

		{"#/definitions/otherRecords/items",
			"#/definitions/otherRecords/items",
			spec.MustCreateRef("#/definitions/otherRecordsItems"),
		},

		{"#/definitions/tags/additionalProperties",
			"#/definitions/tags/additionalProperties",
			spec.MustCreateRef("#/definitions/tagsAdditionalProperties"),
		},
	}

	bp := filepath.Join("fixtures", "nested_inline_schemas.yml")
	sp := loadOrFail(t, bp)

	ere := spec.ExpandSpec(sp, &spec.ExpandOptions{
		RelativeBase: bp,
		SkipSchemas:  true,
	})
	if !assert.NoError(t, ere) {
		t.FailNow()
		return
	}

	ern := nameInlinedSchemas(&FlattenOpts{
		Spec:     New(sp),
		BasePath: bp,
	})
	if !assert.NoError(t, ern) {
		t.FailNow()
		return
	}

	for i, v := range values {
		ptr, err := jsonpointer.New(v.Location[1:])
		if assert.NoError(t, err, "at %d for %s", i, v.Key) {
			vv, _, err := ptr.Get(sp)

			if assert.NoError(t, err, "at %d for %s", i, v.Key) {
				switch tv := vv.(type) {
				case *spec.Schema:
					assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
				case spec.Schema:
					assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
				case *spec.SchemaOrBool:
					var sRef spec.Ref
					if tv != nil && tv.Schema != nil {
						sRef = tv.Schema.Ref
					}
					assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
				case *spec.SchemaOrArray:
					var sRef spec.Ref
					if tv != nil && tv.Schema != nil {
						sRef = tv.Schema.Ref
					}
					assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
				default:
					assert.Fail(t, "unknown type", "got %T", vv)
				}
			}
		}
	}

	for k, rr := range New(sp).allSchemas {
		if !strings.HasPrefix(k, "#/responses") && !strings.HasPrefix(k, "#/parameters") {
			if rr.Schema != nil && rr.Schema.Ref.String() == "" && !rr.TopLevel {
				asch, err := Schema(SchemaOpts{Schema: rr.Schema, Root: sp, BasePath: bp})
				if assert.NoError(t, err, "for key: %s", k) {
					if !asch.IsSimpleSchema && !asch.IsArray && !asch.IsMap {
						assert.Fail(t, "not a top level schema", "for key: %s", k)
					}
				}
			}
		}
	}
}

func TestFlatten(t *testing.T) {
	cwd, _ := os.Getwd()
	bp := filepath.Join(cwd, "fixtures", "flatten.yml")
	sp, err := loadSpec(bp)
	values := []struct {
		Key      string
		Location string
		Ref      spec.Ref
		Expected interface{}
	}{
		{
			"#/responses/notFound/schema",
			"#/responses/notFound/schema",
			spec.MustCreateRef("#/definitions/error"),
			nil,
		},
		{
			"#/paths/~1some~1where~1{id}/parameters/0",
			"#/paths/~1some~1where~1{id}/parameters/0/name",
			spec.Ref{},
			"id",
		},
		{
			"#/paths/~1other~1place",
			"#/paths/~1other~1place/get/operationId",
			spec.Ref{},
			"modelOp",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/0",
			"#/paths/~1some~1where~1{id}/get/parameters/0/name",
			spec.Ref{},
			"limit",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/1",
			"#/paths/~1some~1where~1{id}/get/parameters/1/name",
			spec.Ref{},
			"some",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/2",
			"#/paths/~1some~1where~1{id}/get/parameters/2/name",
			spec.Ref{},
			"other",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/3",
			"#/paths/~1some~1where~1{id}/get/parameters/3/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBody"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/200",
			"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBody"),
			"",
		},
		{
			"#/definitions/namedAgain",
			"",
			spec.MustCreateRef("#/definitions/named"),
			"",
		},
		{
			"#/definitions/namedThing/properties/name",
			"",
			spec.MustCreateRef("#/definitions/named"),
			"",
		},
		{
			"#/definitions/namedThing/properties/namedAgain",
			"",
			spec.MustCreateRef("#/definitions/namedAgain"),
			"",
		},
		{
			"#/definitions/datedRecords/items/1",
			"",
			spec.MustCreateRef("#/definitions/record"),
			"",
		},
		{
			"#/definitions/otherRecords/items",
			"",
			spec.MustCreateRef("#/definitions/record"),
			"",
		},
		{
			"#/definitions/tags/additionalProperties",
			"",
			spec.MustCreateRef("#/definitions/tag"),
			"",
		},
		{
			"#/definitions/datedTag/allOf/1",
			"",
			spec.MustCreateRef("#/definitions/tag"),
			"",
		},
		/* Maps are now considered simple schemas
		{
			"#/definitions/nestedThingRecordItems2/allOf/1",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1"),
			"",
		},
		*/
		{
			"#/definitions/nestedThingRecord/items/1",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems1"),
			"",
		},
		{
			"#/definitions/nestedThingRecord/items/2",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2"),
			"",
		},
		{
			"#/definitions/nestedThing/properties/record",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecord"),
			"",
		},
		{
			"#/definitions/named",
			"#/definitions/named/type",
			spec.Ref{},
			spec.StringOrArray{"string"},
		},
		{
			"#/definitions/error",
			"#/definitions/error/properties/id/type",
			spec.Ref{},
			spec.StringOrArray{"integer"},
		},
		{
			"#/definitions/record",
			"#/definitions/record/properties/createdAt/format",
			spec.Ref{},
			"date-time",
		},
		{
			"#/definitions/getSomeWhereIdOKBody",
			"#/definitions/getSomeWhereIdOKBody/properties/record",
			spec.MustCreateRef("#/definitions/nestedThing"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBody",
			"#/definitions/getSomeWhereIdParamsBody/properties/record",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecord"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecord",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/1",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems1"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecord",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/2",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2/allOf/0/format",
			spec.Ref{},
			"date",
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2Name",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2Name/properties/createdAt/format",
			spec.Ref{},
			"date-time",
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2Name"),
			"date",
		},
	}
	if assert.NoError(t, err) {
		err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp})
		if assert.NoError(t, err) {
			for i, v := range values {
				pk := v.Key[1:]
				if v.Location != "" {
					pk = v.Location[1:]
				}
				ptr, err := jsonpointer.New(pk)
				if assert.NoError(t, err, "at %d for %s", i, v.Key) {
					d, _, err := ptr.Get(sp)
					if assert.NoError(t, err) {
						if v.Ref.String() != "" {
							switch s := d.(type) {
							case *spec.Schema:
								assert.Equal(t, v.Ref.String(), s.Ref.String(), "at %d for %s", i, v.Key)
							case spec.Schema:
								assert.Equal(t, v.Ref.String(), s.Ref.String(), "at %d for %s", i, v.Key)
							case *spec.SchemaOrArray:
								var sRef spec.Ref
								if s != nil && s.Schema != nil {
									sRef = s.Schema.Ref
								}
								assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
							case *spec.SchemaOrBool:
								var sRef spec.Ref
								if s != nil && s.Schema != nil {
									sRef = s.Schema.Ref
								}
								assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
							default:
								assert.Fail(t, "unknown type", "got %T at %d for %s", d, i, v.Key)
							}
						} else {
							assert.Equal(t, v.Expected, d)
						}
					}
				}
			}
		}
	}
}

func TestFlatten_oaigenFull(t *testing.T) {
	defer log.SetOutput(os.Stdout)

	cwd, _ := os.Getwd()
	bp := filepath.Join(cwd, "fixtures", "oaigen", "fixture-oaigen.yaml")
	sp, err := loadSpec(bp)
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)
	//Debug = true
	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: true, Minimal: false, RemoveUnused: false})
	msg := logCapture.String()

	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	//bbb, _ := json.MarshalIndent(sp, "", " ")
	//t.Logf("%s", string(bbb))

	if !assert.Containsf(t, msg, "warning: duplicate flattened definition name resolved as aAOAIGen",
		"Expected log message") {
		t.Logf("Captured log: %s", msg)
	}
	if !assert.NotContainsf(t, msg, "warning: duplicate flattened definition name resolved as uniqueName2OAIGen",
		"Expected log message") {
		t.Logf("Captured log: %s", msg)
	}
	res := getInPath(t, sp, "/some/where", "/get/responses/204/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/uniqueName1"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/204/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/d"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/206/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/a"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/304/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/transitive11"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/205/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/b"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/200/schema")
	assert.JSONEqf(t, `{"type": "integer"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/default/schema")
	// pointer expanded
	assert.JSONEqf(t, `{"type": "integer"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "a")
	assert.JSONEqf(t,
		`{"type": "object", "properties": { "a": { "$ref": "#/definitions/aAOAIGen" }}}`,
		res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "aA")
	assert.JSONEqf(t, `{"type": "string", "format": "date"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "aAOAIGen")
	assert.JSONEqf(t, `{
		"type": "object",
		   "properties": {
		    "b": {
		     "type": "integer"
		 }},
		 "x-go-gen-location": "models"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "bB")
	assert.JSONEqf(t, `{"type": "string", "format": "date-time"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "bItems")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "c": {
		     "type": "integer"
		    }
		   },
		   "x-go-gen-location": "models"
	}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "b")
	assert.JSONEqf(t, `{
		   "type": "array",
		   "items": {
			   "$ref": "#/definitions/bItems"
		   }
	}`, res, "Expected a ref in response")

	res = getDefinition(t, sp, "myBody")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "aA": {
		     "$ref": "#/definitions/aA"
		    },
		    "prop1": {
		     "type": "integer"
		    }
		   }
	}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "uniqueName2")
	assert.JSONEqf(t, `{"$ref": "#/definitions/notUniqueName2"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "notUniqueName2")
	assert.JSONEqf(t, `{
		  "type": "object",
		   "properties": {
		    "prop6": {
		     "type": "integer"
		    }
		   }
	   }`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "uniqueName1")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "prop5": {
		     "type": "integer"
		    }}}`, res, "Expected a simple schema for response")

	// allOf container: []spec.Schema
	res = getDefinition(t, sp, "getWithSliceContainerDefaultBody")
	assert.JSONEqf(t, `{
		"allOf": [
		    {
		     "$ref": "#/definitions/uniqueName3"
		    },
		    {
		     "$ref": "#/definitions/getWithSliceContainerDefaultBodyAllOf1"
		    }
		   ],
		   "x-go-gen-location": "operations"
		    }`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "getWithSliceContainerDefaultBodyAllOf1")
	assert.JSONEqf(t, `{
		"type": "object",
		   "properties": {
		    "prop8": {
		     "type": "string"
		    }
		   },
		   "x-go-gen-location": "models"
		    }`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "getWithTupleContainerDefaultBody")
	assert.JSONEqf(t, `{
		   "type": "array",
		   "items": [
		    {
		     "$ref": "#/definitions/uniqueName3"
		    },
		    {
		     "$ref": "#/definitions/getWithSliceContainerDefaultBodyAllOf1"
		    }
		   ],
		   "x-go-gen-location": "operations"
		    }`, res, "Expected a simple schema for response")

	// with container SchemaOrArray
	res = getDefinition(t, sp, "getWithTupleConflictDefaultBody")
	assert.JSONEqf(t, `{
		   "type": "array",
		   "items": [
		    {
		     "$ref": "#/definitions/uniqueName4"
		    },
		    {
		     "$ref": "#/definitions/getWithTupleConflictDefaultBodyItems1"
		    }
		   ],
		   "x-go-gen-location": "operations"
	}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "getWithTupleConflictDefaultBodyItems1")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "prop10": {
		     "type": "string"
		    }
		   },
		   "x-go-gen-location": "models"
	}`, res, "Expected a simple schema for response")
}

func TestFlatten_oaigenMinimal(t *testing.T) {
	defer log.SetOutput(os.Stdout)

	cwd, _ := os.Getwd()
	bp := filepath.Join(cwd, "fixtures", "oaigen", "fixture-oaigen.yaml")
	sp, err := loadSpec(bp)
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)
	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}
	//bb, _ := json.MarshalIndent(sp, "", " ")
	//t.Log(string(bb))

	msg := logCapture.String()
	//t.Log(msg)
	if !assert.NotContainsf(t, msg,
		"warning: duplicate flattened definition name resolved as aAOAIGen", "Expected log message") {
		t.Logf("Captured log: %s", msg)
	}
	if !assert.NotContainsf(t, msg,
		"warning: duplicate flattened definition name resolved as uniqueName2OAIGen", "Expected log message") {
		t.Logf("Captured log: %s", msg)
	}
	res := getInPath(t, sp, "/some/where", "/get/responses/204/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/uniqueName1"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/204/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/d"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/206/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/a"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/304/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/transitive11"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/get/responses/205/schema")
	assert.JSONEqf(t, `{"$ref": "#/definitions/b"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/200/schema")
	assert.JSONEqf(t, `{"type": "integer"}`, res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where", "/post/responses/default/schema")
	// This JSON pointer is expanded
	assert.JSONEqf(t, `{"type": "integer"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "aA")
	assert.JSONEqf(t, `{"type": "string", "format": "date"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "a")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "a": {
		     "type": "object",
		     "properties": {
		      "b": {
		       "type": "integer"
		      }
		     }
		    }
		   }
		  }`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "bB")
	assert.JSONEqf(t, `{"type": "string", "format": "date-time"}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "bItems")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "c": {
		     "type": "integer"
		    }
		   },
		   "x-go-gen-location": "models"
	}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "b")
	assert.JSONEqf(t, `{
		   "type": "array",
		   "items": {
			   "$ref": "#/definitions/bItems"
		   }
	}`, res, "Expected a ref in response")

	res = getDefinition(t, sp, "myBody")
	assert.JSONEqf(t, `{
		   "type": "object",
		   "properties": {
		    "aA": {
		     "$ref": "#/definitions/aA"
		    },
		    "prop1": {
		     "type": "integer"
		    }
		   }
	}`, res, "Expected a simple schema for response")

	res = getDefinition(t, sp, "uniqueName2")
	assert.JSONEqf(t, `{"$ref": "#/definitions/notUniqueName2"}`, res, "Expected a simple schema for response")

	// with allOf container: []spec.Schema
	res = getInPath(t, sp, "/with/slice/container", "/get/responses/default/schema")
	assert.JSONEqf(t, `{
 			"allOf": [
		        {
		         "$ref": "#/definitions/uniqueName3"
		        },
				{
			     "$ref": "#/definitions/getWithSliceContainerDefaultBodyAllOf1"
				}
		       ]
	}`, res, "Expected a simple schema for response")

	// with tuple container
	res = getInPath(t, sp, "/with/tuple/container", "/get/responses/default/schema")
	assert.JSONEqf(t, `{
		       "type": "array",
		       "items": [
		        {
		         "$ref": "#/definitions/uniqueName3"
		        },
		        {
		         "$ref": "#/definitions/getWithSliceContainerDefaultBodyAllOf1"
		        }
		       ]
	}`, res, "Expected a simple schema for response")

	// with SchemaOrArray container
	res = getInPath(t, sp, "/with/tuple/conflict", "/get/responses/default/schema")
	assert.JSONEqf(t, `{
		       "type": "array",
		       "items": [
		        {
		         "$ref": "#/definitions/uniqueName4"
		        },
		        {
		         "type": "object",
		         "properties": {
		          "prop10": {
		           "type": "string"
		          }
		         }
		        }
		       ]
	}`, res, "Expected a simple schema for response")
}

func loadOrFail(t *testing.T, bp string) *spec.Swagger {
	cwd, _ := os.Getwd()
	sp, err := loadSpec(filepath.Join(cwd, bp))
	if !assert.NoError(t, err) {
		t.FailNow()
		return nil
	}
	return sp
}

func assertNoOAIGen(t *testing.T, bp string, sp *spec.Swagger) bool {
	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)
	err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: true, Minimal: false, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.Fail()
		return false
	}
	msg := logCapture.String()
	assert.NotContains(t, msg, "warning")

	for k := range sp.Definitions {
		if !assert.NotContains(t, k, "OAIGen") {
			t.Fail()
			return false
		}
	}
	return true
}

func TestFlatten_oaigen_1260(t *testing.T) {
	// test fixture from issue go-swagger/go-swagger#1260
	bp := filepath.Join("fixtures", "oaigen", "test3-swagger.yaml")
	sp := loadOrFail(t, bp)
	assert.Truef(t, assertNoOAIGen(t, bp, sp), "did not expect an OAIGen definition here")
}

func TestFlatten_oaigen_1260bis(t *testing.T) {
	// test fixture from issue go-swagger/go-swagger#1260
	bp := filepath.Join("fixtures", "oaigen", "test3-bis-swagger.yaml")
	sp := loadOrFail(t, bp)
	assert.Truef(t, assertNoOAIGen(t, bp, sp), "did not expect an OAIGen definition here")
}

func TestFlatten_oaigen_1260ter(t *testing.T) {
	// test fixture from issue go-swagger/go-swagger#1260
	bp := filepath.Join("fixtures", "oaigen", "test3-ter-swagger.yaml")
	sp := loadOrFail(t, bp)
	assert.Truef(t, assertNoOAIGen(t, bp, sp), "did not expect an OAIGen definition here")
}

func getDefinition(t *testing.T, sp *spec.Swagger, key string) string {
	d, ok := sp.Definitions[key]
	if !assert.Truef(t, ok, "Expected definition for %s", key) {
		t.FailNow()
	}
	res, _ := json.Marshal(d)
	return string(res)
}

func getInPath(t *testing.T, sp *spec.Swagger, path, key string) string {
	ptr, erp := jsonpointer.New(key)
	if !assert.NoError(t, erp, "at %s no key", key) {
		t.FailNow()
	}
	d, _, erg := ptr.Get(sp.Paths.Paths[path])
	if !assert.NoError(t, erg, "at %s no value for %s", path, key) {
		t.FailNow()
	}
	res, _ := json.Marshal(d)
	return string(res)
}

func TestMoreNameInlinedSchemas(t *testing.T) {
	bp := filepath.Join("fixtures", "more_nested_inline_schemas.yml")
	sp := loadOrFail(t, bp)

	err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: true, Minimal: false, RemoveUnused: false})
	assert.NoError(t, err)

	res := getInPath(t, sp, "/some/where/{id}", "/post/responses/200/schema")
	assert.JSONEqf(t,
		`{"type": "object", "additionalProperties":`+
			`{ "type": "object", "additionalProperties": { "type": "object", "additionalProperties":`+
			` { "$ref":`+
			` "#/definitions/postSomeWhereIdOKBodyAdditionalPropertiesAdditionalPropertiesAdditionalProperties"}}}}`,
		res, "Expected a simple schema for response")

	res = getInPath(t, sp, "/some/where/{id}", "/post/responses/204/schema")
	assert.JSONEqf(t, `{
		       "type": "object",
		       "additionalProperties": {
		        "type": "array",
		        "items": {
		         "type": "object",
		         "additionalProperties": {
		          "type": "array",
		          "items": {
		           "type": "object",
		           "additionalProperties": {
		            "type": "array",
		            "items": {
						"$ref":`+
		`"#/definitions/`+
		`postSomeWhereIdNoContentBodyAdditionalPropertiesItemsAdditionalPropertiesItemsAdditionalPropertiesItems"
		            }
		           }
		          }
		         }
		        }
		       }
		   }`, res, "Expected a simple schema for response")

}

func TestRemoveUnused(t *testing.T) {
	bp := filepath.Join("fixtures", "oaigen", "fixture-oaigen.yaml")
	sp := loadOrFail(t, bp)

	err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: false, Minimal: true, RemoveUnused: true})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	assert.Nil(t, sp.Parameters)
	assert.Nil(t, sp.Responses)

	bp = filepath.Join("fixtures", "parameters", "fixture-parameters.yaml")
	sp = loadOrFail(t, bp)
	an := New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: false, Minimal: true, RemoveUnused: true})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	assert.Nil(t, sp.Parameters)
	assert.Nil(t, sp.Responses)

	op, ok := an.OperationFor("GET", "/some/where")
	assert.True(t, ok)
	assert.Lenf(t, op.Parameters, 4, "Expected 4 parameters expanded for this operation")
	assert.Lenf(t, an.ParamsFor("GET", "/some/where"), 7,
		"Expected 7 parameters (with default) expanded for this operation")

	op, ok = an.OperationFor("PATCH", "/some/remote")
	assert.True(t, ok)
	assert.Lenf(t, op.Parameters, 1, "Expected 1 parameter expanded for this operation")
	assert.Lenf(t, an.ParamsFor("PATCH", "/some/remote"), 2,
		"Expected 2 parameters (with default) expanded for this operation")

	_, ok = sp.Definitions["unused"]
	assert.False(t, ok, "Did not expect to find #/definitions/unused")

	bp = filepath.Join("fixtures", "parameters", "fixture-parameters.yaml")
	sp, err = loadSpec(bp)
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}
	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)

	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Verbose: true, Minimal: false, RemoveUnused: true})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	msg := logCapture.String()
	if !assert.Containsf(t, msg, "info: removing unused definition: unused", "Expected log message") {
		t.Logf("Captured log: %s", msg)
	}

	assert.Nil(t, sp.Parameters)
	assert.Nil(t, sp.Responses)
	_, ok = sp.Definitions["unused"]
	assert.Falsef(t, ok, "Did not expect to find #/definitions/unused")
}

func TestOperationIDs(t *testing.T) {
	bp := filepath.Join("fixtures", "operations", "fixture-operations.yaml")
	sp := loadOrFail(t, bp)

	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: false, Minimal: false, RemoveUnused: false})
	assert.NoError(t, err)

	res := gatherOperations(New(sp), []string{"getSomeWhere", "getSomeWhereElse"})
	_, ok := res["getSomeWhere"]
	assert.Truef(t, ok, "Expected to find operation")
	_, ok = res["getSomeWhereElse"]
	assert.Truef(t, ok, "Expected to find operation")
	_, ok = res["postSomeWhere"]
	assert.Falsef(t, ok, "Did not expect to find operation")

	op, ok := an.OperationFor("GET", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("getSomeWhereElse"), 2)

	op, ok = an.OperationFor("POST", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("postSomeWhereElse"), 1)

	op, ok = an.OperationFor("PUT", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("putSomeWhereElse"), 1)

	op, ok = an.OperationFor("PATCH", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("patchSomeWhereElse"), 1)

	op, ok = an.OperationFor("DELETE", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("deleteSomeWhereElse"), 1)

	op, ok = an.OperationFor("HEAD", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("headSomeWhereElse"), 1)

	op, ok = an.OperationFor("OPTIONS", "/some/where/else")
	assert.True(t, ok)
	assert.NotNil(t, op)
	assert.Len(t, an.ParametersFor("optionsSomeWhereElse"), 1)

	assert.Len(t, an.ParametersFor("outOfThisWorld"), 0)
}

func TestFlatten_Pointers(t *testing.T) {
	defer log.SetOutput(os.Stdout)

	bp := filepath.Join("fixtures", "pointers", "fixture-pointers.yaml")
	sp := loadOrFail(t, bp)

	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}
	//bb, _ := json.MarshalIndent(sp, "", " ")
	//t.Log(string(bb))
	msg := logCapture.String()
	if !assert.NotContains(t, msg, "warning") {
		t.Log(msg)
	}

	// re-analyse and check all $ref's point to #/definitions
	bn := New(sp)
	for _, r := range bn.AllRefs() {
		assert.True(t, path.Dir(r.String()) == definitionsPath)
	}
}

// unit test guards in flatten not easily testable with actual specs
func TestFlatten_ErrorHandling(t *testing.T) {
	bp := filepath.Join("fixtures", "errors", "fixture-unexpandable.yaml")

	// invalid spec expansion
	sp := loadOrFail(t, bp)

	err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Expand: true})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	// reload original spec
	sp = loadOrFail(t, bp)
	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Expand: false})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	bp = filepath.Join("fixtures", "errors", "fixture-unexpandable-2.yaml")
	sp = loadOrFail(t, bp)
	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Expand: false})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	// reload original spec
	sp = loadOrFail(t, bp)
	err = Flatten(FlattenOpts{Spec: New(sp), BasePath: bp, Minimal: true, Expand: false})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	// reload original spec
	sp = loadOrFail(t, bp)
	err = rewriteSchemaToRef(sp, "#/invalidPointer/key", spec.Ref{})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	err = rewriteParentRef(sp, "#/invalidPointer/key", spec.Ref{})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	err = updateRef(sp, "#/invalidPointer/key", spec.Ref{})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	err = updateRefWithSchema(sp, "#/invalidPointer/key", &spec.Schema{})
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	_, _, err = getPointerFromKey(sp, "#/invalidPointer/key")
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	_, _, err = getPointerFromKey(sp, "--->#/invalidJsonPointer")
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	_, _, _, err = getParentFromKey(sp, "#/invalidPointer/key")
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	_, _, _, err = getParentFromKey(sp, "--->#/invalidJsonPointer")
	if !assert.Errorf(t, err, "Expected a failure") {
		t.FailNow()
		return
	}

	assert.NotPanics(t, saveNilSchema)
}

func saveNilSchema() {
	cwd, _ := os.Getwd()
	bp := filepath.Join(cwd, "fixtures", "errors", "fixture-unexpandable-2.yaml")
	sp, _ := loadSpec(bp)
	saveSchema(sp, "ThisNilSchema", nil)
}

func TestFlatten_UnitGuards(t *testing.T) {
	parts := keyParts("#/nowhere/arbitrary/pointer")
	res := genLocation(parts)
	assert.Equal(t, "", res)

	res = parts.DefinitionName()
	assert.Equal(t, "", res)

	res = parts.ResponseName()
	assert.Equal(t, "", res)

	b := parts.isKeyName(-1)
	assert.False(t, b)

}

func TestFlatten_PointersLoop(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	bp := filepath.Join("fixtures", "pointers", "fixture-pointers-loop.yaml")
	sp := loadOrFail(t, bp)

	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	if !assert.Error(t, err) {
		t.FailNow()
		return
	}
}

func TestFlatten_Bitbucket(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	bp := filepath.Join("fixtures", "bugs", "bitbucket.json")
	sp := loadOrFail(t, bp)

	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	// reload original spec
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: false, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}

	// reload original spec
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Expand: true, RemoveUnused: false})
	if !assert.NoError(t, err) {
		t.FailNow()
		return
	}
	// reload original spec
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Expand: true, RemoveUnused: true})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Len(t, sp.Definitions, 2) // only 2 remaining refs after expansion: circular $ref
	_, ok := sp.Definitions["base_commit"]
	assert.True(t, ok)
	_, ok = sp.Definitions["repository"]
	assert.True(t, ok)
}

func TestFlatten_Issue_1602(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	// $ref as schema to #/responses or #/parameters

	// minimal repro test case
	bp := filepath.Join("fixtures", "bugs", "1602", "fixture-1602-1.yaml")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	// reload spec
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: false, Minimal: false, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	// reload spec
	// with  prior expansion, a pseudo schema is produced
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: false, Minimal: false, Expand: true,
		RemoveUnused: false})
	assert.NoError(t, err)

	// full testcase
	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-full.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: false, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-1.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-2.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-3.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-4.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-5.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	bp = filepath.Join("fixtures", "bugs", "1602", "fixture-1602-6.yaml")
	sp = loadOrFail(t, bp)
	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)
}

func TestFlatten_Issue_1614(t *testing.T) {
	var logCapture bytes.Buffer
	log.SetOutput(&logCapture)
	defer log.SetOutput(os.Stdout)

	// $ref as schema to #/responses or #/parameters
	// test warnings

	bp := filepath.Join("fixtures", "bugs", "1614", "gitea.yaml")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)
	msg := logCapture.String()
	if !assert.Containsf(t, msg, `warning: found $ref "#/responses/empty" (response) interpreted as schema`,
		"Expected log message") {
		t.Logf("Captured log: %s", msg)
	}
	if !assert.Containsf(t, msg, `warning: found $ref "#/responses/forbidden" (response) interpreted as schema`,
		"Expected log message") {
		t.Logf("Captured log: %s", msg)
	}

	// check responses subject to warning have been expanded
	bbb, _ := json.Marshal(sp)
	assert.NotContains(t, string(bbb), `#/responses/forbidden`)
	assert.NotContains(t, string(bbb), `#/responses/empty`)
}

func TestFlatten_Issue_1621(t *testing.T) {
	// repeated remote refs

	// minimal repro test case
	bp := filepath.Join("fixtures", "bugs", "1621", "fixture-1621.yaml")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)

	sch1 := sp.Paths.Paths["/v4/users/"].Get.Responses.StatusCodeResponses[200].Schema
	bbb, _ := json.Marshal(sch1)
	assert.JSONEq(t, `{
			 "type": "array",
			 "items": {
			  "$ref": "#/definitions/v4UserListItem"
			 }
		 }`, string(bbb))

	sch2 := sp.Paths.Paths["/v4/user/"].Get.Responses.StatusCodeResponses[200].Schema
	bbb, _ = json.Marshal(sch2)
	assert.JSONEq(t, `{
			 "$ref": "#/definitions/v4UserListItem"
			 }`, string(bbb))

	sch3 := sp.Paths.Paths["/v4/users/{email}/"].Get.Responses.StatusCodeResponses[200].Schema
	bbb, _ = json.Marshal(sch3)
	assert.JSONEq(t, `{
			 "$ref": "#/definitions/v4UserListItem"
			 }`, string(bbb))
}

// wrapWindowsPath adapts path expectations for tests running on windows
func wrapWindowsPath(p string) string {
	if goruntime.GOOS != "windows" {
		return p
	}
	pp := filepath.FromSlash(p)
	if !filepath.IsAbs(p) && []rune(pp)[0] == '\\' {
		pp, _ = filepath.Abs(p)
		u, _ := url.Parse(pp)
		return u.String()
	}
	return pp
}

func Test_NormalizePath(t *testing.T) {
	values := []struct{ Source, Expected string }{
		{"#/definitions/A", "#/definitions/A"},
		{"http://somewhere.com/definitions/A", "http://somewhere.com/definitions/A"},
		{wrapWindowsPath("/definitions/A"), wrapWindowsPath("/definitions/A")}, // considered absolute on unix but not on windows
		{wrapWindowsPath("/definitions/errorModel.json") + "#/definitions/A", wrapWindowsPath("/definitions/errorModel.json") + "#/definitions/A"},
		{"http://somewhere.com", "http://somewhere.com"},
		{wrapWindowsPath("./definitions/definitions.yaml") + "#/definitions/A", wrapWindowsPath("/abs/to/spec/definitions/definitions.yaml") + "#/definitions/A"},
		{"#", wrapWindowsPath("/abs/to/spec")},
	}

	for _, v := range values {
		assert.Equal(t, v.Expected, normalizePath(spec.MustCreateRef(v.Source),
			&FlattenOpts{BasePath: wrapWindowsPath("/abs/to/spec/spec.json")}))
	}
}

func TestFlatten_Issue_1796(t *testing.T) {
	// remote cyclic ref
	bp := filepath.Join("fixtures", "bugs", "1796", "queryIssue.json")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)
	//bbb, _ := json.MarshalIndent(an.spec, "", " ")
	//t.Logf("%s", string(bbb))
	//t.Logf("%s", an.AllDefinitionReferences())

	// assert all $ref match  "$ref": "#/definitions/something"
	for _, ref := range an.AllReferences() {
		assert.True(t, strings.HasPrefix(ref, "#/definitions"))
	}
}

func TestFlatten_Issue_1767(t *testing.T) {
	// remote cyclic ref again
	bp := filepath.Join("fixtures", "bugs", "1767", "fixture-1767.yaml")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)
	// assert all $ref match  "$ref": "#/definitions/something"
	for _, ref := range an.AllReferences() {
		assert.True(t, strings.HasPrefix(ref, "#/definitions"))
	}
}

func TestFlatten_Issue_1774(t *testing.T) {
	// remote cyclic ref again
	bp := filepath.Join("fixtures", "bugs", "1774", "def_api.yaml")
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: false, Expand: false,
		RemoveUnused: false})
	assert.NoError(t, err)
	//bbb, _ := json.MarshalIndent(an.spec, "", " ")
	//t.Logf("%s", string(bbb))
	//t.Logf("%s", an.AllDefinitionReferences())
	// assert all $ref match  "$ref": "#/definitions/something"
	for _, ref := range an.AllReferences() {
		assert.True(t, strings.HasPrefix(ref, "#/definitions"))
	}
}

func TestFlatten_1429(t *testing.T) {
	// nested / remote $ref in response / param schemas
	// issue go-swagger/go-swagger#1429
	bp := filepath.Join("fixtures", "bugs", "1429", "swagger.yaml")
	sp := loadOrFail(t, bp)

	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	assert.NoError(t, err)
}

func TestRebaseRef(t *testing.T) {
	assert.Equal(t, "#/definitions/abc", rebaseRef("#/definitions/base", "#/definitions/abc"))
	assert.Equal(t, "#/definitions/abc", rebaseRef("", "#/definitions/abc"))
	assert.Equal(t, "#/definitions/abc", rebaseRef(".", "#/definitions/abc"))
	assert.Equal(t, "otherfile#/definitions/abc", rebaseRef("file#/definitions/base", "otherfile#/definitions/abc"))
	assert.Equal(t, wrapWindowsPath("../otherfile")+"#/definitions/abc", rebaseRef(wrapWindowsPath("../file")+"#/definitions/base", wrapWindowsPath("./otherfile")+"#/definitions/abc"))
	assert.Equal(t, wrapWindowsPath("../otherfile")+"#/definitions/abc", rebaseRef(wrapWindowsPath("../file")+"#/definitions/base", wrapWindowsPath("otherfile")+"#/definitions/abc"))
	assert.Equal(t, wrapWindowsPath("local/remote/otherfile")+"#/definitions/abc", rebaseRef(wrapWindowsPath("local/file")+"#/definitions/base", wrapWindowsPath("remote/otherfile")+"#/definitions/abc"))
	assert.Equal(t, wrapWindowsPath("local/remote/otherfile.yaml"), rebaseRef(wrapWindowsPath("local/file.yaml"), wrapWindowsPath("remote/otherfile.yaml")))

	assert.Equal(t, "file#/definitions/abc", rebaseRef("file#/definitions/base", "#/definitions/abc"))

	// with remote
	assert.Equal(t, "https://example.com/base#/definitions/abc", rebaseRef("https://example.com/base", "https://example.com/base#/definitions/abc"))
	assert.Equal(t, "https://example.com/base#/definitions/abc", rebaseRef("https://example.com/base", "#/definitions/abc"))
	assert.Equal(t, "https://example.com/base#/dir/definitions/abc", rebaseRef("https://example.com/base", "#/dir/definitions/abc"))
	assert.Equal(t, "https://example.com/base/dir/definitions/abc", rebaseRef("https://example.com/base/spec.yaml", "dir/definitions/abc"))
	assert.Equal(t, "https://example.com/base/dir/definitions/abc", rebaseRef("https://example.com/base/", "dir/definitions/abc"))
	assert.Equal(t, "https://example.com/dir/definitions/abc", rebaseRef("https://example.com/base", "dir/definitions/abc"))
}

func TestFlatten_1851(t *testing.T) {
	// nested / remote $ref in response / param schemas
	// issue go-swagger/go-swagger#1851
	bp := filepath.Join("fixtures", "bugs", "1851", "fixture-1851.yaml")
	sp := loadOrFail(t, bp)

	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	assert.NoError(t, err)
	var jazon []byte

	//bbb, _ := json.MarshalIndent(an.spec, "", " ")
	//t.Logf("%s", string(bbb))
	serverDefinition, ok := an.spec.Definitions["server"]
	assert.True(t, ok)
	serverStatusDefinition, ok := an.spec.Definitions["serverStatus"]
	assert.True(t, ok)
	serverStatusProperty, ok := serverDefinition.Properties["Status"]
	assert.True(t, ok)
	jazon, _ = json.Marshal(serverStatusProperty)
	assert.JSONEq(t, `{"$ref": "#/definitions/serverStatus"}`, string(jazon))
	jazon, _ = json.Marshal(serverStatusDefinition)
	assert.JSONEq(t, `{
         "type": "string",
         "enum": [
          "OK",
          "Not OK"
         ]
	 }`, string(jazon))

	// additional test case: this one used to work
	bp = filepath.Join("fixtures", "bugs", "1851", "fixture-1851-2.yaml")
	sp = loadOrFail(t, bp)

	an = New(sp)
	err = Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	assert.NoError(t, err)
	serverDefinition, ok = an.spec.Definitions["Server"]
	assert.True(t, ok)
	serverStatusDefinition, ok = an.spec.Definitions["ServerStatus"]
	assert.True(t, ok)
	serverStatusProperty, ok = serverDefinition.Properties["Status"]
	assert.True(t, ok)
	jazon, _ = json.Marshal(serverStatusProperty)
	assert.JSONEq(t, `{"$ref": "#/definitions/ServerStatus"}`, string(jazon))
	jazon, _ = json.Marshal(serverStatusDefinition)
	assert.JSONEq(t, `{
         "type": "string",
         "enum": [
          "OK",
          "Not OK"
         ]
	 }`, string(jazon))
}

var (
	rex    = regexp.MustCompile(`"\$ref":\s*"(.+)"`)
	oairex = regexp.MustCompile(`oiagen`)
)

func checkRefs(t *testing.T, spec *spec.Swagger, expectNoConflict bool) {
	// all $ref resolve locally
	jazon, _ := json.MarshalIndent(spec, "", " ")
	m := rex.FindAllStringSubmatch(string(jazon), -1)
	if assert.NotNil(t, m) {
		for _, matched := range m {
			subMatch := matched[1]
			assert.True(t, strings.HasPrefix(subMatch, "#/definitions/"),
				"expected $ref to be inlined, got: %s", matched[0])
		}
	}

	if expectNoConflict {
		// no naming conflict
		m := oairex.FindAllStringSubmatch(string(jazon), -1)
		assert.Empty(t, m)
	}
}

func testFlattenWithDefaults(t *testing.T, bp string) *Spec {
	sp := loadOrFail(t, bp)
	an := New(sp)
	err := Flatten(FlattenOpts{Spec: an, BasePath: bp, Verbose: true, Minimal: true, RemoveUnused: false})
	require.NoError(t, err)
	return an
}

func TestFlatten_RemoteAbsolute(t *testing.T) {
	// this one has simple remote ref pattern
	an := testFlattenWithDefaults(t, filepath.Join("fixtures", "bugs", "remote-absolute", "swagger-mini.json"))
	checkRefs(t, an.spec, true)

	// this has no remote ref
	an = testFlattenWithDefaults(t, filepath.Join("fixtures", "bugs", "remote-absolute", "swagger.json"))
	checkRefs(t, an.spec, true)

	// this one has local ref, no naming conflict (same as previous but with external ref imported)
	an = testFlattenWithDefaults(t, filepath.Join("fixtures", "bugs", "remote-absolute", "swagger-with-local-ref.json"))
	checkRefs(t, an.spec, true)

	// this one has remote ref, no naming conflict (same as previous but with external ref imported)
	an = testFlattenWithDefaults(t, filepath.Join("fixtures", "bugs", "remote-absolute", "swagger-with-remote-only-ref.json"))
	checkRefs(t, an.spec, true)

	// this one has both remote and local ref with naming conflict.
	// This creates some "oiagen" definitions to address naming conflict, which are removed by the oaigen pruning process (reinlined / merged with parents).
	an = testFlattenWithDefaults(t, filepath.Join("fixtures", "bugs", "remote-absolute", "swagger-with-ref.json"))
	checkRefs(t, an.spec, false)
}
