package parameterizer

import (
	"reflect"
	"testing"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func TestAddParam(t *testing.T) {

	startingParams := func() []templateapi.Parameter {
		return []templateapi.Parameter{
			makeParameter("p1", "v1"),
			makeParameter("p2", "v2"),
			makeParameter("p3", "v3"),
		}
	}

	tests := []struct {
		params     []templateapi.Parameter // parameters to add
		expect     []string                // expecteded parameter names
		expectSize int                     // expected size of Params after adding
	}{
		{
			params:     []templateapi.Parameter{makeParameter("p1", "v1")},
			expect:     []string{"p1"},
			expectSize: 3, // Params should not grow because p1 already exists
		},
		{
			params:     []templateapi.Parameter{makeParameter("p4", "v4")},
			expect:     []string{"p4"},
			expectSize: 4, // Params should grow by 1
		},
		{
			params:     []templateapi.Parameter{makeParameter("p1", "v1a")},
			expect:     []string{"p1_1"},
			expectSize: 4,
		},
		{
			params: []templateapi.Parameter{
				makeParameter("p1", "v1a"),
				makeParameter("p1", "v1b"),
				makeParameter("p1", "v1c"),
				makeParameter("p1", "v1b"),
				makeParameter("p1", "v1a"),
			},
			expect:     []string{"p1_1", "p1_2", "p1_3", "p1_2", "p1_1"},
			expectSize: 6,
		},
	}
	for _, test := range tests {
		params := ParamsFromList(startingParams())
		actual := []string{}
		for _, p := range test.params {
			actual = append(actual, params.AddParam(p))
		}
		if !reflect.DeepEqual(actual, test.expect) {
			t.Errorf("params %#v: expected: %v, got %v", test.params, test.expect, actual)
			continue
		}
		if test.expectSize != len(params) {
			t.Errorf("params %#v:  expected size: %d, got %d", test.params, test.expectSize, len(params))
		}
	}
}
