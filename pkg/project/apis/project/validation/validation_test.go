package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

func TestValidateProject(t *testing.T) {
	testCases := []struct {
		name    string
		project projectapi.Project
		numErrs int
		errText string
	}{
		{
			name: "missing id",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						oapi.OpenShiftDescription: "This is a description",
						oapi.OpenShiftDisplayName: "hi",
					},
				},
			},
			// Should fail because the ID is missing.
			numErrs: 1,
		},
		{
			name: "invalid id",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "141-.124.$",
					Annotations: map[string]string{
						oapi.OpenShiftDescription: "This is a description",
						oapi.OpenShiftDisplayName: "hi",
					},
				},
			},
			// Should fail because the ID is invalid.
			numErrs: 1,
		},
		{
			name: "invalid id uppercase",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "AA",
				},
			},
			numErrs: 1,
		},
		{
			name: "valid id leading number",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "11",
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid id for create (< 2 characters)",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h",
				},
			},
			numErrs: 1,
		},
		{
			name: "valid id for create (2+ characters)",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hi",
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid id for create (> 63 characters)",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "werthtyghyghgfdvfffghjiuyhnjhgfvdddddcfgtytgfredswazsxdeeerfvgtyhbj",
				},
			},
			numErrs: 1,
		},
		{
			name: "invalid id start with dash",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "-pro",
				},
			},
			numErrs: 1,
		},
		{
			name: "invalid id end wih dash",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pro-",
				},
			},
			numErrs: 1,
		},
		{
			name: "invalid id internal dots",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "1.a.1",
				},
			},
			numErrs: 1,
		},
		{
			name: "has namespace",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
					Annotations: map[string]string{
						oapi.OpenShiftDescription: "This is a description",
						oapi.OpenShiftDisplayName: "hi",
					},
				},
			},
			// Should fail because the namespace is supplied.
			numErrs: 1,
		},
		{
			name: "invalid display name",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						oapi.OpenShiftDescription: "This is a description",
						oapi.OpenShiftDisplayName: "h\t\ni",
					},
				},
			},
			// Should fail because the display name has \t \n
			numErrs: 1,
		},
		{
			name: "valid node selector",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						projectapi.ProjectNodeSelector: "infra=true, env = test",
					},
				},
			},
			numErrs: 0,
		},
		{
			name: "invalid node selector",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						projectapi.ProjectNodeSelector: "infra, env = $test",
					},
				},
			},
			// Should fail because infra and $test doesn't satisfy the format
			numErrs: 1,
		},
		{
			name: "valid node selector",
			project: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "",
					Annotations: map[string]string{
						projectapi.ProjectNodeSelector: "env,qa",
					},
				},
			},
			// Should fail because ProjectNodeSelector is invalid
			errText: `nodeSelector: Invalid value: "env,qa": must be a valid label selector`,
			numErrs: 1,
		},
	}

	for _, tc := range testCases {
		errs := ValidateProject(&tc.project)
		if len(errs) != tc.numErrs {
			t.Errorf("Unexpected error list for case %q: %+v", tc.name, errs)
		}
		for _, v := range errs {
			if tc.errText != "" && (tc.errText != string(v.Error())) {
				t.Errorf("Unexpected error text %q for %q", string(v.Error()), tc.errText)
			}
		}
	}

	project := projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				oapi.OpenShiftDescription: "This is a description",
				oapi.OpenShiftDisplayName: "hi",
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
	project := &projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "project-name",
			ResourceVersion: "1",
			Annotations: map[string]string{
				oapi.OpenShiftDescription:      "This is a description",
				oapi.OpenShiftDisplayName:      "display name",
				projectapi.ProjectNodeSelector: "infra=true, env = test",
			},
			Labels: map[string]string{"label-name": "value"},
		},
	}
	updateDisplayname := &projectapi.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "project-name",
			ResourceVersion: "1",
			Annotations: map[string]string{
				oapi.OpenShiftDescription:      "This is a description",
				oapi.OpenShiftDisplayName:      "display name change",
				projectapi.ProjectNodeSelector: "infra=true, env = test",
			},
			Labels: map[string]string{"label-name": "value"},
		},
	}

	errs := ValidateProjectUpdate(updateDisplayname, project)
	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got %v", errs)
	}

	errorCases := map[string]struct {
		A projectapi.Project
		T field.ErrorType
		F string
	}{
		"change name": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "different",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
					Labels:          project.Labels,
				},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"invalid displayname": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						oapi.OpenShiftDescription:      "This is a description",
						oapi.OpenShiftDisplayName:      "display name\n",
						projectapi.ProjectNodeSelector: "infra=true, env = test",
					},
					Labels: project.Labels,
				},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.annotations[" + oapi.OpenShiftDisplayName + "]",
		},
		"updating disallowed annotation": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						oapi.OpenShiftDescription:      "This is a description",
						oapi.OpenShiftDisplayName:      "display name",
						projectapi.ProjectNodeSelector: "infra=true, env = test2",
					},
					Labels: project.Labels,
				},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.annotations[openshift.io/node-selector]",
		},
		"delete annotation": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations: map[string]string{
						oapi.OpenShiftDescription: "This is a description",
						oapi.OpenShiftDisplayName: "display name",
					},
					Labels: project.Labels,
				},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.annotations[openshift.io/node-selector]",
		},
		"updating label": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
					Labels:          map[string]string{"label-name": "diff"},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.labels[label-name]",
		},
		"deleting label": {
			A: projectapi.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "project-name",
					ResourceVersion: "1",
					Annotations:     project.Annotations,
				},
			},
			T: field.ErrorTypeInvalid,
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
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}

}
