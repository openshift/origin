package admission

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestAlwaysPullBuildImagesAdmission(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		resource       string
		object         runtime.Object
		responseObject runtime.Object
		expectAccept   bool
		expectedError  string
	}{
		{
			name: "build - custom",
			object: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						CustomStrategy: &buildapi.CustomBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			kind:         "Build",
			resource:     buildsResource,
			expectAccept: true,
		},
		{
			name: "build - docker",
			object: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			kind:         "Build",
			resource:     buildsResource,
			expectAccept: true,
		},
		{
			name: "build - source",
			object: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			kind:         "Build",
			resource:     buildsResource,
			expectAccept: true,
		},
		{
			name: "build config - custom",
			object: &buildapi.BuildConfig{
				Spec: buildapi.BuildConfigSpec{
					BuildSpec: buildapi.BuildSpec{
						Strategy: buildapi.BuildStrategy{
							CustomStrategy: &buildapi.CustomBuildStrategy{
								ForcePull: false,
							},
						},
					},
				},
			},
			kind:         "BuildConfig",
			resource:     buildConfigsResource,
			expectAccept: true,
		},
		{
			name: "build config - docker",
			object: &buildapi.BuildConfig{
				Spec: buildapi.BuildConfigSpec{
					BuildSpec: buildapi.BuildSpec{
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{
								ForcePull: false,
							},
						},
					},
				},
			},
			kind:         "BuildConfig",
			resource:     buildConfigsResource,
			expectAccept: true,
		},
		{
			name: "build config - source",
			object: &buildapi.BuildConfig{
				Spec: buildapi.BuildConfigSpec{
					BuildSpec: buildapi.BuildSpec{
						Strategy: buildapi.BuildStrategy{
							SourceStrategy: &buildapi.SourceBuildStrategy{
								ForcePull: false,
							},
						},
					},
				},
			},
			kind:         "BuildConfig",
			resource:     buildConfigsResource,
			expectAccept: true,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			c := NewAlwaysPullBuildImages()
			attrs := admission.NewAttributesRecord(test.object, test.kind, "default", "name", test.resource, "", op, nil)
			err := c.Admit(attrs)
			if err != nil && test.expectAccept {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}

			if !apierrors.IsForbidden(err) && !test.expectAccept {
				if (len(test.expectedError) != 0) || (test.expectedError == err.Error()) {
					continue
				}
				t.Errorf("%s: expecting reject error, got %v", test.name, err)
			}

			var strategy *buildapi.BuildStrategy
			switch obj := test.object.(type) {
			case *buildapi.Build:
				strategy = &obj.Spec.Strategy
			case *buildapi.BuildConfig:
				strategy = &obj.Spec.Strategy
			}

			switch {
			case strategy.CustomStrategy != nil:
				if strategy.CustomStrategy.ForcePull == false {
					t.Errorf("%s: force pull was false")
				}
			case strategy.DockerStrategy != nil:
				if strategy.DockerStrategy.ForcePull == false {
					t.Errorf("%s: force pull was false")
				}
			case strategy.SourceStrategy != nil:
				if strategy.SourceStrategy.ForcePull == false {
					t.Errorf("%s: force pull was false")
				}
			}

		}
	}
}
