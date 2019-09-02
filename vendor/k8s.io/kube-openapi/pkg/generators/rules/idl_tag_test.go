package rules

import (
	"reflect"
	"testing"

	"k8s.io/gengo/types"
)

func TestListTypeMissing(t *testing.T) {
	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expected []string
	}{
		{
			name:     "none",
			t:        &types.Type{},
			expected: []string{},
		},
		{
			name: "simple missing",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					types.Member{
						Name: "Containers",
						Type: &types.Type{
							Kind: types.Slice,
						},
					},
				},
			},
			expected: []string{"Containers"},
		},
		{
			name: "simple passing",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					types.Member{
						Name: "Containers",
						Type: &types.Type{
							Kind: types.Slice,
						},
						CommentLines: []string{"+listType=map"},
					},
				},
			},
			expected: []string{},
		},
	}

	rule := &ListTypeMissing{}
	for _, tc := range tcs {
		if violations, _ := rule.Validate(tc.t); !reflect.DeepEqual(violations, tc.expected) {
			t.Errorf("unexpected validation result: test name %v, want: %v, got: %v",
				tc.name, tc.expected, violations)
		}
	}

}
