/*
Copyright 2018 The Kubernetes Authors.

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

package generators

import (
	"errors"
	"reflect"
	"testing"
)

func TestSingleTagExtension(t *testing.T) {

	// Comments only contain one tag extension and one value.
	var tests = []struct {
		comments        []string
		extensionName   string
		extensionValues []string
	}{
		{
			comments:        []string{"+patchMergeKey=name"},
			extensionName:   "x-kubernetes-patch-merge-key",
			extensionValues: []string{"name"},
		},
		{
			comments:        []string{"+patchStrategy=merge"},
			extensionName:   "x-kubernetes-patch-strategy",
			extensionValues: []string{"merge"},
		},
		{
			comments:        []string{"+listType=atomic"},
			extensionName:   "x-kubernetes-list-type",
			extensionValues: []string{"atomic"},
		},
		{
			comments:        []string{"+listMapKey=port"},
			extensionName:   "x-kubernetes-list-map-keys",
			extensionValues: []string{"port"},
		},
		{
			comments:        []string{"+k8s:openapi-gen=x-kubernetes-member-tag:member_test"},
			extensionName:   "x-kubernetes-member-tag",
			extensionValues: []string{"member_test"},
		},
		{
			// Test that poorly formatted extensions aren't added.
			comments: []string{
				"+k8s:openapi-gen=x-kubernetes-no-value",
				"+k8s:openapi-gen=x-kubernetes-member-success:success",
				"+k8s:openapi-gen=x-kubernetes-wrong-separator;error",
			},
			extensionName:   "x-kubernetes-member-success",
			extensionValues: []string{"success"},
		},
	}
	for _, test := range tests {
		actual := parseExtensions(test.comments)[0]
		if actual.name != test.extensionName {
			t.Errorf("Extension Name: expected (%s), actual (%s)\n", test.extensionName, actual.name)
		}
		if !reflect.DeepEqual(actual.values, test.extensionValues) {
			t.Errorf("Extension Values: expected (%s), actual (%s)\n", test.extensionValues, actual.values)
		}
		if actual.hasMultipleValues() {
			t.Errorf("%s: hasMultipleValues() should be false\n", actual.name)
		}
	}

}

func TestMultipleTagExtensions(t *testing.T) {

	var tests = []struct {
		comments        []string
		extensionName   string
		extensionValues []string
	}{
		{
			comments: []string{
				"+listMapKey=port",
				"+listMapKey=protocol",
			},
			extensionName:   "x-kubernetes-list-map-keys",
			extensionValues: []string{"port", "protocol"},
		},
	}
	for _, test := range tests {
		actual := parseExtensions(test.comments)[0]
		if actual.name != test.extensionName {
			t.Errorf("Extension Name: expected (%s), actual (%s)\n", actual.name, test.extensionName)
		}
		if !reflect.DeepEqual(actual.values, test.extensionValues) {
			t.Errorf("Extension Values: expected (%s), actual (%s)\n", actual.values, test.extensionValues)
		}
		if !actual.hasMultipleValues() {
			t.Errorf("%s: hasMultipleValues() should be true\n", actual.name)
		}
	}

}

func TestExtensionAllowedValues(t *testing.T) {

	var tests = []struct {
		e   extension
		err error
	}{
		{
			e: extension{
				name:   "x-kubernetes-patch-strategy",
				values: []string{"merge"},
			},
			err: nil,
		},
		{
			// Validate multiple values.
			e: extension{
				name:   "x-kubernetes-patch-strategy",
				values: []string{"merge", "retainKeys"},
			},
			err: nil,
		},
		{
			// Every value must be allowed.
			e: extension{
				name:   "x-kubernetes-patch-strategy",
				values: []string{"disallowed", "merge"},
			},
			err: errors.New("x-kubernetes-patch-strategy: value(s) [disallowed merge] not allowed. Allowed values: [merge retainKeys]\n"),
		},
		{
			e: extension{
				name:   "x-kubernetes-patch-strategy",
				values: []string{"foo"},
			},
			err: errors.New("x-kubernetes-patch-strategy: value(s) [foo] not allowed. Allowed values: [merge retainKeys]\n"),
		},
		{
			e: extension{
				name:   "x-kubernetes-patch-merge-key",
				values: []string{"key1"},
			},
			err: nil,
		},
		{
			e: extension{
				name:   "x-kubernetes-list-type",
				values: []string{"atomic"},
			},
			err: nil,
		},
		{
			e: extension{
				name:   "x-kubernetes-list-type",
				values: []string{"not-allowed"},
			},
			err: errors.New("x-kubernetes-list-type: value(s) [not-allowed] not allowed. Allowed values: [atomic map set]\n"),
		},
	}
	for _, test := range tests {
		actualErr := test.e.validateAllowedValues()
		if !reflect.DeepEqual(test.err, actualErr) {
			t.Errorf("Expected: %v, Got: %v\n", test.err, actualErr)
		}
	}

}
