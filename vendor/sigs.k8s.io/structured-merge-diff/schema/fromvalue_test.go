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

package schema_test

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/structured-merge-diff/schema"
	"sigs.k8s.io/structured-merge-diff/value"
)

func TestTypeRefFromValue(t *testing.T) {

	table := []struct {
		objYAML string
		typeRef string
	}{
		{
			objYAML: `[1.0]`,
			typeRef: `untyped: {}`,
		}, {
			objYAML: `null`,
			typeRef: `untyped: {}`,
		}, {
			objYAML: `a: a`,
			typeRef: `struct:
  fields:
  - name: a
    type:
      untyped: {}`,
		}, {
			objYAML: `{"q": {"y": 6, "b": [7, 8, 9]}}`,
			typeRef: `struct:
  fields:
  - name: q
    type:
      struct:
        fields:
        - name: y
          type:
            untyped: {}
        - name: b
          type:
            untyped: {}`,
		},
	}

	for _, tt := range table {
		tt := tt
		t.Run(tt.objYAML, func(t *testing.T) {
			t.Parallel()
			v, err := value.FromYAML([]byte(tt.objYAML))
			if err != nil {
				t.Fatalf("couldn't parse: %v", err)
			}
			got := schema.TypeRefFromValue(v)

			expected := schema.TypeRef{}
			err = yaml.Unmarshal([]byte(tt.typeRef), &expected)
			if err != nil {
				t.Fatalf("couldn't parse: %v", err)
			}

			if !reflect.DeepEqual(got, expected) {
				t.Errorf("wanted\n%+v\nbut got\n%+v\n", expected, got)
			}
		})
	}
}
