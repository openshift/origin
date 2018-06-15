package securitycontextconstraints

import (
	"reflect"
	"testing"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestCanonicalize(t *testing.T) {
	testCases := []struct {
		obj    *securityapi.SecurityContextConstraints
		expect *securityapi.SecurityContextConstraints
	}{
		{
			obj:    &securityapi.SecurityContextConstraints{},
			expect: &securityapi.SecurityContextConstraints{},
		},
		{
			obj: &securityapi.SecurityContextConstraints{
				Users: []string{"a"},
			},
			expect: &securityapi.SecurityContextConstraints{
				Users: []string{"a"},
			},
		},
		{
			obj: &securityapi.SecurityContextConstraints{
				Users:  []string{"a", "a"},
				Groups: []string{"b", "b"},
			},
			expect: &securityapi.SecurityContextConstraints{
				Users:  []string{"a"},
				Groups: []string{"b"},
			},
		},
		{
			obj: &securityapi.SecurityContextConstraints{
				Users:  []string{"a", "b", "a"},
				Groups: []string{"c", "d", "c"},
			},
			expect: &securityapi.SecurityContextConstraints{
				Users:  []string{"a", "b"},
				Groups: []string{"c", "d"},
			},
		},
	}
	for i, testCase := range testCases {
		Strategy.Canonicalize(testCase.obj)
		if !reflect.DeepEqual(testCase.expect, testCase.obj) {
			t.Errorf("%d: unexpected object: %#v", i, testCase.obj)
		}
	}
}
