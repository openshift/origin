package validation

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
				JSONBase:    kubeapi.JSONBase{Namespace: kubeapi.NamespaceDefault},
				DisplayName: "hi",
				Description: "This is a description",
			},
			// Should fail because the ID is missing.
			numErrs: 1,
		},
		{
			name: "invalid id",
			project: api.Project{
				JSONBase:    kubeapi.JSONBase{ID: "141-.124.$", Namespace: kubeapi.NamespaceDefault},
				DisplayName: "hi",
				Description: "This is a description",
			},
			// Should fail because the ID is invalid.
			numErrs: 1,
		},
		{
			name: "missing namespace",
			project: api.Project{
				JSONBase:    kubeapi.JSONBase{ID: "foo", Namespace: ""},
				DisplayName: "hi",
				Description: "This is a description",
			},
			// Should fail because the namespace is missing.
			numErrs: 1,
		},
		{
			name: "invalid namespace",
			project: api.Project{
				JSONBase:    kubeapi.JSONBase{ID: "foo", Namespace: "141-.124.$"},
				DisplayName: "hi",
				Description: "This is a description",
			},
			// Should fail because the namespace is missing.
			numErrs: 1,
		},
		{
			name: "invalid description",
			project: api.Project{
				JSONBase:    kubeapi.JSONBase{ID: "foo", Namespace: "foo"},
				DisplayName: "hi",
				Description: "This is a \n description",
			},
			// Should fail because the description has a \n
			numErrs: 1,
		},
		{
			name: "invalid display name",
			project: api.Project{
				JSONBase:    kubeapi.JSONBase{ID: "foo", Namespace: "foo"},
				DisplayName: "h\t\ni",
				Description: "This is a description",
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
		JSONBase:    kubeapi.JSONBase{ID: "foo", Namespace: kubeapi.NamespaceDefault},
		DisplayName: "hi",
		Description: "This is a description",
	}
	errs := ValidateProject(&project)
	if len(errs) != 0 {
		t.Errorf("Unexpected non-zero error list: %#v", errs)
	}
}
