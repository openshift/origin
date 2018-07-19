package resourcemerge

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name         string
		curr         map[string]interface{}
		additional   map[string]interface{}
		specialCases map[string]MergeFunc

		expected    map[string]interface{}
		expectedErr string
	}{
		{
			name: "add non-conflicting",
			curr: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": "one",
				},
			},
			additional: map[string]interface{}{
				"bravo": map[string]interface{}{
					"banana": "two",
					"cake": map[string]interface{}{
						"armadillo": "uno",
					},
				},
				"charlie": "third",
			},

			expected: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple":  "one",
					"banana": "two",
					"cake": map[string]interface{}{
						"armadillo": "uno",
					},
				},
				"charlie": "third",
			},
		},
		{
			name: "add conflicting, replace type",
			curr: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": "one",
				},
			},
			additional: map[string]interface{}{
				"bravo": map[string]interface{}{
					"apple": map[string]interface{}{
						"armadillo": "uno",
					},
				},
			},

			expected: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": map[string]interface{}{
						"armadillo": "uno",
					},
				},
			},
		},
		{
			name: "nil out",
			curr: map[string]interface{}{
				"alpha": "first",
			},
			additional: map[string]interface{}{
				"alpha": nil,
			},

			expected: map[string]interface{}{
				"alpha": nil,
			},
		},
		{
			name: "force empty",
			curr: map[string]interface{}{
				"alpha": "first",
			},
			additional: map[string]interface{}{
				"alpha": "",
			},

			expected: map[string]interface{}{
				"alpha": "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := mergeConfig(test.curr, test.additional, "", test.specialCases)
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing %q", test.expectedErr)
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err != nil && len(test.expectedErr) != 0 && !strings.Contains(err.Error(), test.expectedErr):
				t.Fatalf("expected %q, got %q", test.expectedErr, err)
			}

			if !reflect.DeepEqual(test.expected, test.curr) {
				t.Error(diff.ObjectDiff(test.expected, test.curr))
			}
		})
	}
}
