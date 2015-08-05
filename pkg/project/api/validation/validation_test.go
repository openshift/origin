package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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
						api.ProjectDescription: "This is a description",
						api.ProjectDisplayName: "hi",
					},
				},
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
						api.ProjectDescription: "This is a description",
						api.ProjectDisplayName: "hi",
					},
				},
			},
			// Should fail because the ID is invalid.
			numErrs: 1,
		},
		{
			name: "invalid id uppercase",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "AA",
				},
			},
			numErrs: 1,
		},
		{
			name: "valid id leading number",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "11",
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid id for create (< 2 characters)",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "h",
				},
			},
			numErrs: 1,
		},
		{
			name: "valid id for create (2+ characters)",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "hi",
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid id internal dots",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name: "1.a.1",
				},
			},
			numErrs: 1,
		},
		{
			name: "has namespace",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
					Annotations: map[string]string{
						api.ProjectDescription: "This is a description",
						api.ProjectDisplayName: "hi",
					},
				},
			},
			// Should fail because the namespace is supplied.
			numErrs: 1,
		},
		{
			name: "invalid display name",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						api.ProjectDescription: "This is a description",
						api.ProjectDisplayName: "h\t\ni",
					},
				},
			},
			// Should fail because the display name has \t \n
			numErrs: 1,
		},
		{
			name: "valid node selector",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						api.ProjectNodeSelector: "infra=true, env = test",
					},
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid node selector",
			project: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						api.ProjectNodeSelector: "infra, env = $test",
					},
				},
			},
			// Should fail because infra and $test doesn't satisfy the format
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
				api.ProjectDescription: "This is a description",
				api.ProjectDisplayName: "hi",
			},
		},
	}
	errs := ValidateProject(&project)
	if len(errs) != 0 {
		t.Errorf("Unexpected non-zero error list: %#v", errs)
	}
}

func TestValidateProjectUpdate(t *testing.T) {
	// Ensure we can update projects with short names, to make sure we can
	// proxy updates to namespaces created outside project validation
	project := &api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "project-name",
			ResourceVersion: "1",
			Annotations: map[string]string{
				api.ProjectDescription:  "This is a description",
				api.ProjectDisplayName:  "display name",
				api.ProjectNodeSelector: "infra=true, env = test",
			},
			Labels: map[string]string{"label-name": "value"},
		},
	}
	updateDisplayname := &api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "project-name",
			ResourceVersion: "1",
			Annotations: map[string]string{
				api.ProjectDescription:  "This is a description",
				api.ProjectDisplayName:  "display name change",
				api.ProjectNodeSelector: "infra=true, env = test",
			},
			Labels: map[string]string{"label-name": "value"},
		},
	}

	errs := ValidateProjectUpdate(updateDisplayname, project)
	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got %v", errs)
	}

	errorCases := map[string]struct {
		A api.Project
		T fielderrors.ValidationErrorType
		F string
	}{
		"change name": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "different",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
					Labels:          project.Labels,
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.name",
		},
		"invalid displayname": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						api.ProjectDescription:  "This is a description",
						api.ProjectDisplayName:  "display name\n",
						api.ProjectNodeSelector: "infra=true, env = test",
					},
					Labels: project.Labels,
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.annotations[" + api.ProjectDisplayName + "]",
		},
		"updating disallowed annotation": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						api.ProjectDescription:  "This is a description",
						api.ProjectDisplayName:  "display name",
						api.ProjectNodeSelector: "infra=true, env = test2",
					},
					Labels: project.Labels,
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.annotations[openshift.io/node-selector]",
		},
		"delete annotation": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						api.ProjectDescription: "This is a description",
						api.ProjectDisplayName: "display name",
					},
					Labels: project.Labels,
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.annotations[openshift.io/node-selector]",
		},
		"updating label": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
					Labels:          map[string]string{"label-name": "diff"},
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.labels[label-name]",
		},
		"deleting label": {
			A: api.Project{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
				},
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.labels[label-name]",
		},
	}
	for k, v := range errorCases {
		errs := ValidateProjectUpdate(&v.A, project)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}

}
