/*
Copyright 2019 The Kubernetes Authors.
 
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
 
    http://www.apache.org/licenses/LICENSE-2.0
 
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
 
package merge_test
 
import (
	"testing"
 
	"sigs.k8s.io/structured-merge-diff/fieldpath"
	. "sigs.k8s.io/structured-merge-diff/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/typed"
)
 
var nestedTypeParser = func() typed.ParseableType {
	parser, err := typed.NewParser(`types:
- name: type
  struct:
    fields:
      - name: listOfLists
        type:
          namedType: listOfLists
      - name: listOfMaps
        type:
          namedType: listOfMaps
      - name: mapOfLists
        type:
          namedType: mapOfLists
      - name: mapOfMaps
        type:
          namedType: mapOfMaps
      - name: mapOfMapsRecursive
        type:
          namedType: mapOfMapsRecursive
- name: listOfLists
  list:
    elementType:
      struct:
        fields:
        - name: name
          type:
            scalar: string
        - name: value
          type:
            namedType: list
    elementRelationship: associative
    keys:
    - name
- name: list
  list:
    elementType:
      scalar: string
    elementRelationship: associative
- name: listOfMaps
  list:
    elementType:
      struct:
        fields:
        - name: name
          type:
            scalar: string
        - name: value
          type:
            namedType: map
    elementRelationship: associative
    keys:
    - name
- name: map
  map:
    elementType:
      scalar: string
    elementRelationship: associative
- name: mapOfLists
  map:
    elementType:
      namedType: list
    elementRelationship: associative
- name: mapOfMaps
  map:
    elementType:
      namedType: map
    elementRelationship: associative
- name: mapOfMapsRecursive
  map:
    elementType:
      namedType: mapOfMapsRecursive
    elementRelationship: associative
`)
	if err != nil {
		panic(err)
	}
	return parser.Type("type")
}()
 
func TestUpdateNestedType(t *testing.T) {
	tests := map[string]TestCase{
		"listOfLists_change_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - a
						  - c
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				listOfLists:
				- name: a
				  value:
				  - a
				  - c
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
						_P("listOfLists", _KBF("name", _SV("a")), "value", _SV("a")),
						_P("listOfLists", _KBF("name", _SV("a")), "value", _SV("c")),
					),
					APIVersion: "v1",
				},
			},
		},
		"listOfLists_change_key_and_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						listOfLists:
						- name: b
						  value:
						  - a
						  - c
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				listOfLists:
				- name: b
				  value:
				  - a
				  - c
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("b"))),
						_P("listOfLists", _KBF("name", _SV("b")), "name"),
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("a")),
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("c")),
					),
					APIVersion: "v1",
				},
			},
		},
		"listOfMaps_change_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						listOfMaps:
						- name: a
						  value:
						    b: "x"
						    c: "y"
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						listOfMaps:
						- name: a
						  value:
						    a: "x"
						    c: "z"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				listOfMaps:
				- name: a
				  value:
				    a: "x"
				    c: "z"
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfMaps", _KBF("name", _SV("a"))),
						_P("listOfMaps", _KBF("name", _SV("a")), "name"),
						_P("listOfMaps", _KBF("name", _SV("a")), "value", "a"),
						_P("listOfMaps", _KBF("name", _SV("a")), "value", "c"),
					),
					APIVersion: "v1",
				},
			},
		},
		"listOfMaps_change_key_and_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						listOfMaps:
						- name: a
						  value:
						    b: "x"
						    c: "y"
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						listOfMaps:
						- name: b
						  value:
						    a: "x"
						    c: "z"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				listOfMaps:
				- name: b
				  value:
				    a: "x"
				    c: "z"
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfMaps", _KBF("name", _SV("b"))),
						_P("listOfMaps", _KBF("name", _SV("b")), "name"),
						_P("listOfMaps", _KBF("name", _SV("b")), "value", "a"),
						_P("listOfMaps", _KBF("name", _SV("b")), "value", "c"),
					),
					APIVersion: "v1",
				},
			},
		},
		"mapOfLists_change_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						mapOfLists:
						  a:
						  - b
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						mapOfLists:
						  a:
						  - a
						  - c
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				mapOfLists:
				  a:
				  - a
				  - c
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfLists", "a"),
						_P("mapOfLists", "a", _SV("a")),
						_P("mapOfLists", "a", _SV("c")),
					),
					APIVersion: "v1",
				},
			},
		},
		"mapOfLists_change_key_and_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						mapOfLists:
						  a:
						  - b
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						mapOfLists:
						  b:
						  - a
						  - c
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				mapOfLists:
				  b:
				  - a
				  - c
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfLists", "b"),
						_P("mapOfLists", "b", _SV("a")),
						_P("mapOfLists", "b", _SV("c")),
					),
					APIVersion: "v1",
				},
			},
		},
		"mapOfMaps_change_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						mapOfMaps:
						  a:
						    b: "x"
						    c: "y"
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						mapOfMaps:
						  a:
						    a: "x"
						    c: "z"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				mapOfMaps:
				  a:
				    a: "x"
				    c: "z"
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMaps", "a"),
						_P("mapOfMaps", "a", "a"),
						_P("mapOfMaps", "a", "c"),
					),
					APIVersion: "v1",
				},
			},
		},
		"mapOfMaps_change_key_and_value": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						mapOfMaps:
						  a:
						    b: "x"
						    c: "y"
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						mapOfMaps:
						  b:
						    a: "x"
						    c: "z"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				mapOfMaps:
				  b:
				    a: "x"
				    c: "z"
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMaps", "b"),
						_P("mapOfMaps", "b", "a"),
						_P("mapOfMaps", "b", "c"),
					),
					APIVersion: "v1",
				},
			},
		},
		"mapOfMapsRecursive_change_middle_key": {
			Ops: []Operation{
				Apply{
					Manager: "default",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						      c:
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "default",
					Object: `
						mapOfMapsRecursive:
						  a:
						    d:
						      c:
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				mapOfMapsRecursive:
				  a:
				    d:
				      c:
			`,
			Managed: fieldpath.ManagedFields{
				"default": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "a"),
						_P("mapOfMapsRecursive", "a", "d"),
						_P("mapOfMapsRecursive", "a", "d", "c"),
					),
					APIVersion: "v1",
				},
			},
		},
	}
 
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(nestedTypeParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}
