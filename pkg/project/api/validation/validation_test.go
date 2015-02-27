package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/project/api"
)

func TestValidateProject(t *testing.T) {
	testCases := []struct {
		name    string
		project api.Project
		numErrs int
	}{
		{
			name: "missing id",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{
						"description": "This is a description",
					},
				},
				DisplayName: "hi",
			},
			// Should fail because the ID is missing.
			numErrs: 1,
		},
		{
			name: "invalid id",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "141-.124.$",
					Annotations: map[string]string{
						"description": "This is a description",
					},
				},
				DisplayName: "hi",
			},
			// Should fail because the ID is invalid.
			numErrs: 1,
		},
		{
			name: "invalid id uppercase",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "A",
				},
			},
			numErrs: 1,
		},
		{
			name: "valid id leading number",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "1",
				},
			},
			numErrs: 0,
		},
		{
			name: "valid id internal dots",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "1.a.1",
				},
			},
			numErrs: 0,
		},
		{
			name: "has namespace",
			project: api.Project{
				ObjectMeta:  kapi.ObjectMeta{Name: "foo", Namespace: "foo"},
				DisplayName: "hi",
			},
			// Should fail because the namespace is supplied.
			numErrs: 1,
		},
		{
			name: "invalid display name",
			project: api.Project{
				ObjectMeta:  kapi.ObjectMeta{Name: "foo", Namespace: ""},
				DisplayName: "h\t\ni",
			},
			// Should fail because the display name has \t \n
			numErrs: 1,
		},
	}

	for _, tc := range testCases {
		errs := ValidateProject(&tc.project)
		if len(errs) != tc.numErrs {
			t.Errorf("Unexpected error list for case %q: %+v", tc.name, errs)
		}
	}

	project := api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"description": "This is a description",
			},
		},
		DisplayName: "hi",
	}
	errs := ValidateProject(&project)
	if len(errs) != 0 {
		t.Errorf("Unexpected non-zero error list: %#v", errs)
	}
}
