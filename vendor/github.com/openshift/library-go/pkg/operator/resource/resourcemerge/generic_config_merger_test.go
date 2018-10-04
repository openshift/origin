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

func TestMergeProcessConfig(t *testing.T) {
	tests := []struct {
		name         string
		curr         string
		additional   string
		specialCases map[string]MergeFunc

		expected    string
		expectedErr string
	}{
		{
			name: "no conflict on missing typemeta",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
bravo: two
`,
			expected: `{"alpha":"first","apiVersion":"foo","bravo":"two","kind":"the-kind"}
`,
		},
		{
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			name: "no conflict on same typemeta",
			additional: `
apiVersion: foo
kind: the-kind
bravo: two
`,
			expected: `{"alpha":"first","apiVersion":"foo","bravo":"two","kind":"the-kind"}
`,
		},
		{
			name: "conflict on different typemeta 01",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
kind: the-other-kind
bravo: two
`,
			expectedErr: `/the-other-kind does not equal foo/the-kind`,
		},
		{
			name: "conflict on different typemeta 03",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
apiVersion: bar
bravo: two
`,
			expectedErr: `bar/ does not equal foo/the-kind`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := MergeProcessConfig(test.specialCases, []byte(test.curr), []byte(test.additional))
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing %q", test.expectedErr)
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err != nil && len(test.expectedErr) != 0 && !strings.Contains(err.Error(), test.expectedErr):
				t.Fatalf("expected %q, got %q", test.expectedErr, err)
			}
			if err != nil {
				return
			}

			if test.expected != string(actual) {
				t.Error(diff.StringDiff(test.expected, string(actual)))
			}
		})
	}
}
