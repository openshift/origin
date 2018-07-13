package templateinstances

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func TestDefaultMigrations(t *testing.T) {
	testCases := []struct {
		name   string
		input  metav1.TypeMeta
		output metav1.TypeMeta
	}{
		{
			name:   "legacy-dc",
			input:  metav1.TypeMeta{"DeploymentConfig", "v1"},
			output: metav1.TypeMeta{"DeploymentConfig", "apps.openshift.io/v1"},
		},
		{
			name:   "lazy-dc",
			input:  metav1.TypeMeta{"DeploymentConfig", ""},
			output: metav1.TypeMeta{"DeploymentConfig", "apps.openshift.io/v1"},
		},
		{
			name:   "ok-dc",
			input:  metav1.TypeMeta{"DeploymentConfig", "apps.openshift.io/v1"},
			output: metav1.TypeMeta{"DeploymentConfig", "apps.openshift.io/v1"},
		},
		{
			name:   "legacy-bc",
			input:  metav1.TypeMeta{"BuildConfig", "v1"},
			output: metav1.TypeMeta{"BuildConfig", "build.openshift.io/v1"},
		},
		{
			name:   "lazy-bc",
			input:  metav1.TypeMeta{"BuildConfig", ""},
			output: metav1.TypeMeta{"BuildConfig", "build.openshift.io/v1"},
		},
		{
			name:   "ok-bc",
			input:  metav1.TypeMeta{"BuildConfig", "build.openshift.io/v1"},
			output: metav1.TypeMeta{"BuildConfig", "build.openshift.io/v1"},
		},
		{
			name:   "legacy-build",
			input:  metav1.TypeMeta{"Build", "v1"},
			output: metav1.TypeMeta{"Build", "build.openshift.io/v1"},
		},
		{
			name:   "lazy-build",
			input:  metav1.TypeMeta{"Build", ""},
			output: metav1.TypeMeta{"Build", "build.openshift.io/v1"},
		},
		{
			name:   "ok-build",
			input:  metav1.TypeMeta{"Build", "build.openshift.io/v1"},
			output: metav1.TypeMeta{"Build", "build.openshift.io/v1"},
		},
		{
			name:   "legacy-route",
			input:  metav1.TypeMeta{"Route", "v1"},
			output: metav1.TypeMeta{"Route", "route.openshift.io/v1"},
		},
		{
			name:   "lazy-route",
			input:  metav1.TypeMeta{"Route", ""},
			output: metav1.TypeMeta{"Route", "route.openshift.io/v1"},
		},
		{
			name:   "ok-route",
			input:  metav1.TypeMeta{"Route", "route.openshift.io/v1"},
			output: metav1.TypeMeta{"Route", "route.openshift.io/v1"},
		},
		{
			name:   "legacy-other",
			input:  metav1.TypeMeta{"Cheddar", "v1"},
			output: metav1.TypeMeta{"Cheddar", "v1"},
		},
		{
			name:   "ok-other",
			input:  metav1.TypeMeta{"Cheddar", "cheese/v1alpha1"},
			output: metav1.TypeMeta{"Cheddar", "cheese/v1alpha1"},
		},
	}

	opts := MigrateTemplateInstancesOptions{
		transforms: transforms,
	}

	for _, tc := range testCases {
		tc := tc // copy the iteration variable to a non-iteration memory location
		t.Run(tc.name, func(t *testing.T) {
			oldTI := &templateapi.TemplateInstance{
				Status: templateapi.TemplateInstanceStatus{
					Objects: []templateapi.TemplateInstanceObject{
						{
							Ref: kapi.ObjectReference{
								APIVersion: tc.input.APIVersion,
								Kind:       tc.input.Kind,
								Name:       tc.name,
							},
						},
					},
				},
			}

			reporter, err := opts.checkAndTransform(oldTI)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			expectedChanged := tc.input != tc.output
			if reporter.Changed() != expectedChanged {
				t.Errorf("expected changed to be: %v, but changed=%v", expectedChanged, reporter.Changed())
			}
			newVersionKind := metav1.TypeMeta{
				APIVersion: oldTI.Status.Objects[0].Ref.APIVersion,
				Kind:       oldTI.Status.Objects[0].Ref.Kind,
			}
			if newVersionKind != tc.output {
				t.Errorf("expected the template instance to be updated to %v, yet it ended up as %v", tc.output, newVersionKind)
			}
		})

	}
}
