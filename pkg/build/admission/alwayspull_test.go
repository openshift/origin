package admission

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestAlwaysPullBuildImagesForBuilds(t *testing.T) {
	tests := []struct {
		name          string
		build         *buildapi.Build
		expectAccept  bool
		expectedError string
	}{
		{
			name: "build - custom",
			build: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						CustomStrategy: &buildapi.CustomBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			expectAccept: true,
		},
		{
			name: "build - docker",
			build: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			expectAccept: true,
		},
		{
			name: "build - source",
			build: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							ForcePull: false,
						},
					},
				},
			},
			expectAccept: true,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			c := NewAlwaysPullBuildImages()
			attrs := admission.NewAttributesRecord(test.build, "Build", "default", "name", "builds", "", op, nil)
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

			strategy := test.build.Spec.Strategy
			switch {
			case strategy.CustomStrategy != nil:
				if strategy.CustomStrategy.ForcePull == false {
					t.Errorf("%s (%s): force pull was false", test.name, op)
				}
			case strategy.DockerStrategy != nil:
				if strategy.DockerStrategy.ForcePull == false {
					t.Errorf("%s (%s): force pull was false", test.name, op)
				}
			case strategy.SourceStrategy != nil:
				if strategy.SourceStrategy.ForcePull == false {
					t.Errorf("%s (%s): force pull was false", test.name, op)
				}
			}

		}
	}
}

func TestAlwaysPullBuildImagesForBuildRequests(t *testing.T) {
	request := &buildapi.BuildRequest{}
	ops := []admission.Operation{admission.Create, admission.Update}
	for _, op := range ops {
		c := NewAlwaysPullBuildImages()
		attrs := admission.NewAttributesRecord(request, "BuildRequest", "default", "name", "buildrequests", "", op, nil)
		err := c.Admit(attrs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if request.Annotations == nil {
			t.Fatal("unexpected nil annotations")
		}

		if e, a := "true", request.Annotations[buildapi.BuildAlwaysPullImagesAnnotation]; e != a {
			t.Fatalf("unexpected value for %s: %q", buildapi.BuildAlwaysPullImagesAnnotation, a)
		}
	}
}

func TestAlwaysPullBuildImagesForOtherResources(t *testing.T) {
	build := &buildapi.Build{
		Spec: buildapi.BuildSpec{
			Strategy: buildapi.BuildStrategy{
				SourceStrategy: &buildapi.SourceBuildStrategy{
					ForcePull: false,
				},
			},
		},
	}

	bc := &buildapi.BuildConfig{
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						ForcePull: false,
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		kind        string
		resource    string
		subresource string
		object      runtime.Object
		expectError bool
	}{
		{
			name:     "other resource",
			kind:     "Foo",
			resource: "foos",
			object:   build,
		},
		{
			name:        "build subresource",
			kind:        "Build",
			resource:    "builds",
			subresource: "clone",
			object:      build,
		},
		{
			name:        "non-build object",
			kind:        "Build",
			resource:    "builds",
			object:      &buildapi.BuildConfig{},
			expectError: true,
		},
		{
			name:        "non-buildrequest object",
			kind:        "BuildRequest",
			resource:    "buildrequests",
			object:      &buildapi.BuildConfig{},
			expectError: true,
		},
		{
			name:     "build config",
			kind:     "BuildConfig",
			resource: "buildconfigs",
			object:   bc,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			handler := NewAlwaysPullBuildImages()

			err := handler.Admit(admission.NewAttributesRecord(test.object, test.kind, "default", "name", test.resource, test.subresource, op, nil))

			if test.expectError {
				if err == nil {
					t.Errorf("%s (%s): unexpected nil error", test.name, op)
				}
				continue
			}

			if err != nil {
				t.Errorf("%s (%s): unexpected error: %v", test.name, op, err)
				continue
			}

			if build.Spec.Strategy.SourceStrategy.ForcePull != false {
				t.Errorf("%s (%s): build force pull should be false", test.name, op)
			}
			if bc.Spec.Strategy.SourceStrategy.ForcePull != false {
				t.Errorf("%s (%s): build config force pull should be false", test.name, op)
			}
		}
	}
}
