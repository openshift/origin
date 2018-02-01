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

package parameters

import (
	"reflect"
	"testing"

	_ "github.com/kubernetes-incubator/service-catalog/internal/test"
)

func TestParseVariableAssignments(t *testing.T) {
	testcases := []struct {
		Name, Raw, Variable, Value string
	}{
		{"simple", "a=b", "a", "b"},
		{"multiple equal signs", "c=abc1232===", "c", "abc1232==="},
		{"empty value", "d=", "d", ""},
		{"extra whitespace", " a = b ", "a", "b"},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {

			params := []string{tc.Raw}

			got, err := ParseVariableAssignments(params)
			if err != nil {
				t.Fatal(err)
			}

			want := map[string]string{tc.Variable: tc.Value}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("%s\nexpected:\n\t%v\ngot:\n\t%v\n", tc.Raw, want, got)
			}
		})
	}
}

func TestParseVariableAssignments_MissingVariableName(t *testing.T) {
	params := []string{"=b"}

	_, err := ParseVariableAssignments(params)
	if err == nil {
		t.Fatal("should have failed due to a missing variable name")
	}
}

func TestParseKeyMaps(t *testing.T) {
	testcases := []struct {
		Name, Raw, MapName, Key string
	}{
		{"simple", "a[b]", "a", "b"},
		{"multiple brackets signs", "c[[d]]", "c", "[d]"},
		{"extra whitespace", " a [ b ] ", "a", "b"},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {

			params := []string{tc.Raw}

			got, err := ParseKeyMaps(params)
			if err != nil {
				t.Fatal(err)
			}

			want := map[string]string{tc.MapName: tc.Key}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("%s\nexpected:\n\t%v\ngot:\n\t%v\n", tc.Raw, want, got)
			}
		})
	}
}

func TestParseKeyMaps_InvalidInput(t *testing.T) {
	testcases := []struct {
		Name, Raw string
	}{
		{"missing map", "[b]"},
		{"missing key", "a[]"},
	}
	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			params := []string{tc.Raw}

			result, err := ParseKeyMaps(params)
			if err == nil {
				t.Fatalf("expected parse to fail for %s but got %v", tc.Raw, result)
			}
		})
	}
}
