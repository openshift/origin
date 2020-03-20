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

package generator

import (
	"reflect"
	"testing"
)

func TestTransitiveClosure(t *testing.T) {
	cases := []struct {
		name     string
		in       map[string][]string
		expected map[string][]string
	}{
		{
			name: "no transition",
			in: map[string][]string{
				"a": {"b"},
				"c": {"d"},
			},
			expected: map[string][]string{
				"a": {"b"},
				"c": {"d"},
			},
		},
		{
			name: "simple",
			in: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {"d"},
			},
			expected: map[string][]string{
				"a": {"b", "c", "d"},
				"b": {"c", "d"},
				"c": {"d"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := transitiveClosure(c.in)
			if !reflect.DeepEqual(c.expected, out) {
				t.Errorf("expected: %#v, got %#v", c.expected, out)
			}
		})
	}
}
