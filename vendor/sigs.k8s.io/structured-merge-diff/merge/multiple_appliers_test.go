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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"sigs.k8s.io/structured-merge-diff/fieldpath"
	. "sigs.k8s.io/structured-merge-diff/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/merge"
	"sigs.k8s.io/structured-merge-diff/typed"
)

func TestMultipleAppliersSet(t *testing.T) {
	tests := map[string]TestCase{
		"remove_one": {
			Ops: []Operation{
				Apply{
					Manager:    "apply-one",
					APIVersion: "v1",
					Object: `
						list:
						- name: a
						- name: b
					`,
				},
				Apply{
					Manager:    "apply-two",
					APIVersion: "v2",
					Object: `
						list:
						- name: c
					`,
				},
				Apply{
					Manager:    "apply-one",
					APIVersion: "v3",
					Object: `
						list:
						- name: a
					`,
				},
			},
			Object: `
				list:
				- name: a
				- name: c
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("a"))),
						_P("list", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("c"))),
						_P("list", _KBF("name", _SV("c")), "name"),
					),
					APIVersion: "v2",
				},
			},
		},
		"same_value_no_conflict": {
			Ops: []Operation{
				Apply{
					Manager:    "apply-one",
					APIVersion: "v1",
					Object: `
						list:
						- name: a
						  value: 0
					`,
				},
				Apply{
					Manager:    "apply-two",
					APIVersion: "v2",
					Object: `
						list:
						- name: a
						  value: 0
					`,
				},
			},
			Object: `
				list:
				- name: a
				  value: 0
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("a"))),
						_P("list", _KBF("name", _SV("a")), "name"),
						_P("list", _KBF("name", _SV("a")), "value"),
					),
					APIVersion: "v1",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("a"))),
						_P("list", _KBF("name", _SV("a")), "name"),
						_P("list", _KBF("name", _SV("a")), "value"),
					),
					APIVersion: "v2",
				},
			},
		},
		"change_value_yes_conflict": {
			Ops: []Operation{
				Apply{
					Manager:    "apply-one",
					APIVersion: "v1",
					Object: `
						list:
						- name: a
						  value: 0
					`,
				},
				Apply{
					Manager:    "apply-two",
					APIVersion: "v2",
					Object: `
						list:
						- name: a
						  value: 1
					`,
					Conflicts: merge.Conflicts{
						merge.Conflict{Manager: "apply-one", Path: _P("list", _KBF("name", _SV("a")), "value")},
					},
				},
			},
			Object: `
				list:
				- name: a
				  value: 0
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("a"))),
						_P("list", _KBF("name", _SV("a")), "name"),
						_P("list", _KBF("name", _SV("a")), "value"),
					),
					APIVersion: "v1",
				},
			},
		},
		"remove_one_keep_one": {
			Ops: []Operation{
				Apply{
					Manager:    "apply-one",
					APIVersion: "v1",
					Object: `
						list:
						- name: a
						- name: b
						- name: c
					`,
				},
				Apply{
					Manager:    "apply-two",
					APIVersion: "v2",
					Object: `
						list:
						- name: c
						- name: d
					`,
				},
				Apply{
					Manager:    "apply-one",
					APIVersion: "v3",
					Object: `
						list:
						- name: a
					`,
				},
			},
			Object: `
				list:
				- name: a
				- name: c
				- name: d
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("a"))),
						_P("list", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("list", _KBF("name", _SV("c"))),
						_P("list", _KBF("name", _SV("d"))),
						_P("list", _KBF("name", _SV("c")), "name"),
						_P("list", _KBF("name", _SV("d")), "name"),
					),
					APIVersion: "v2",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(associativeListParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMultipleAppliersNestedType(t *testing.T) {
	tests := map[string]TestCase{
		"remove_one_keep_one_with_two_sub_items": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						listOfLists:
						- name: b
						  value:
						  - d
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				listOfLists:
				- name: a
				- name: b
				  value:
				  - d
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("b"))),
						_P("listOfLists", _KBF("name", _SV("b")), "name"),
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("d")),
					),
					APIVersion: "v2",
				},
			},
		},
		"remove_one_keep_one_with_dangling_subitem": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						listOfLists:
						- name: b
						  value:
						  - d
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
						  - d
						  - e
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				listOfLists:
				- name: a
				- name: b
				  value:
				  - d
				  - e
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("b"))),
						_P("listOfLists", _KBF("name", _SV("b")), "name"),
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("d")),
					),
					APIVersion: "v2",
				},
				"controller": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("e")),
					),
					APIVersion: "v2",
				},
			},
		},
		"remove_one_with_dangling_subitem_keep_one": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
						- name: b
						  value:
						  - c
						  - d
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				listOfLists:
				- name: a
				  value:
				  - b
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
						_P("listOfLists", _KBF("name", _SV("a")), "value", _SV("b")),
					),
					APIVersion: "v2",
				},
			},
		},
		"remove_one_with_managed_subitem_keep_one": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller",
					Object: `
						listOfLists:
						- name: a
						  value:
						  - b
						- name: b
						  value:
						  - c
						  - d
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				listOfLists:
				- name: a
				  value:
				  - b
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
						_P("listOfLists", _KBF("name", _SV("a")), "value", _SV("b")),
					),
					APIVersion: "v2",
				},
			},
		},
		"remove_one_keep_one_with_sub_item": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
						- name: b
						  value:
						  - c
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						listOfLists:
						- name: b
						  value:
						  - d
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						listOfLists:
						- name: a
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				listOfLists:
				- name: a
				- name: b
				  value:
				  - d
			`,
			Managed: fieldpath.ManagedFields{
				"apply-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("a"))),
						_P("listOfLists", _KBF("name", _SV("a")), "name"),
					),
					APIVersion: "v3",
				},
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("listOfLists", _KBF("name", _SV("b"))),
						_P("listOfLists", _KBF("name", _SV("b")), "name"),
						_P("listOfLists", _KBF("name", _SV("b")), "value", _SV("d")),
					),
					APIVersion: "v2",
				},
			},
		},
		"multiple_appliers_recursive_map": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						  c:
						    d:
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						mapOfMapsRecursive:
						  a:
						  c:
						    d:
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller-one",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						      c:
						  c:
						    d:
						      e:
					`,
					APIVersion: "v3",
				},
				Update{
					Manager: "controller-two",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						      c:
						        d:
						  c:
						    d:
						      e:
						        f:
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller-one",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						      c:
						        d:
						          e:
						  c:
						    d:
						      e:
						        f:
						          g:
					`,
					APIVersion: "v3",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						mapOfMapsRecursive:
					`,
					APIVersion: "v4",
				},
			},
			Object: `
				mapOfMapsRecursive:
				  a:
				  c:
				    d:
				      e:
				        f:
				          g:
			`,
			Managed: fieldpath.ManagedFields{
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "a"),
						_P("mapOfMapsRecursive", "c"),
						_P("mapOfMapsRecursive", "c", "d"),
					),
					APIVersion: "v2",
				},
				"controller-one": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "c", "d", "e"),
						_P("mapOfMapsRecursive", "c", "d", "e", "f", "g"),
					),
					APIVersion: "v3",
				},
				"controller-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "c", "d", "e", "f"),
					),
					APIVersion: "v2",
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

func TestMultipleAppliersRealConversion(t *testing.T) {
	tests := map[string]TestCase{
		"multiple_appliers_recursive_map_real_conversion": {
			Ops: []Operation{
				Apply{
					Manager: "apply-one",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						  c:
						    d:
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply-two",
					Object: `
						mapOfMapsRecursive:
						  aa:
						  cc:
						    dd:
					`,
					APIVersion: "v2",
				},
				Update{
					Manager: "controller",
					Object: `
						mapOfMapsRecursive:
						  aaa:
						    bbb:
						      ccc:
						        ddd:
						  ccc:
						    ddd:
						      eee:
						        fff:
					`,
					APIVersion: "v3",
				},
				Apply{
					Manager: "apply-one",
					Object: `
						mapOfMapsRecursive:
					`,
					APIVersion: "v4",
				},
			},
			Object: `
				mapOfMapsRecursive:
				  aaaa:
				  cccc:
				    dddd:
				      eeee:
				        ffff:
			`,
			Managed: fieldpath.ManagedFields{
				"apply-two": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "aa"),
						_P("mapOfMapsRecursive", "cc"),
						_P("mapOfMapsRecursive", "cc", "dd"),
					),
					APIVersion: "v2",
				},
				"controller": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "ccc", "ddd", "eee"),
						_P("mapOfMapsRecursive", "ccc", "ddd", "eee", "fff"),
					),
					APIVersion: "v3",
				},
			},
		},
		"appliers_remove_from_controller_real_conversion": {
			Ops: []Operation{
				Update{
					Manager: "controller",
					Object: `
						mapOfMapsRecursive:
						  a:
						    b:
						      c:
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "apply",
					Object: `
						mapOfMapsRecursive:
						  aa:
						    bb:
						  cc:
						    dd:
					`,
					APIVersion: "v2",
				},
				Apply{
					Manager: "apply",
					Object: `
						mapOfMapsRecursive:
						  aaa:
						  ccc:
					`,
					APIVersion: "v3",
				},
			},
			Object: `
				mapOfMapsRecursive:
				  aaa:
				  ccc:
			`,
			Managed: fieldpath.ManagedFields{
				"controller": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive"),
						_P("mapOfMapsRecursive", "a"),
					),
					APIVersion: "v1",
				},
				"apply": &fieldpath.VersionedSet{
					Set: _NS(
						_P("mapOfMapsRecursive", "aaa"),
						_P("mapOfMapsRecursive", "ccc"),
					),
					APIVersion: "v3",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.TestWithConverter(nestedTypeParser, repeatingConverter{nestedTypeParser}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// repeatingConverter repeats a single letterkey v times, where v is the version.
type repeatingConverter struct{
	typed.ParseableType
}

var _ merge.Converter = repeatingConverter{}

var missingVersionError error = fmt.Errorf("cannot convert to invalid version")

// Convert implements merge.Converter
func (r repeatingConverter) Convert(v typed.TypedValue, version fieldpath.APIVersion) (typed.TypedValue, error) {
	if len(version) < 2 || string(version)[0] != 'v' {
		return nil, missingVersionError
	}
	versionNumber, err := strconv.Atoi(string(version)[1:len(version)])
	if err != nil {
		return nil, missingVersionError
	}
	y, err := v.AsValue().ToYAML()
	if err != nil {
		return nil, err
	}
	str := string(y)
	var str2 string
	for i, line := range strings.Split(str, "\n") {
		if i == 0 {
			str2 = line
		} else {
			spaces := strings.Repeat(" ", countLeadingSpace(line))
			if len(spaces) == 0 {
				break
			}
			c := line[len(spaces):len(spaces)+1]
			c = strings.Repeat(c, versionNumber)
			str2 = fmt.Sprintf("%v\n%v%v:", str2, spaces, c)
		}
	}
	v2, err := r.ParseableType.FromYAML(typed.YAMLObject(str2))
	if err != nil {
		return nil, err
	}
	return v2, nil
}

func countLeadingSpace(line string) int {
        spaces := 0
        for _, letter := range line {
                if letter == ' ' {
                        spaces++
                } else {
                        break
                }
        }
        return spaces
}

// Convert implements merge.Converter
func (r repeatingConverter) IsMissingVersionError(err error) bool {
	return err == missingVersionError
}
