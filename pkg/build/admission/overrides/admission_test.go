package overrides

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"

	overridesapi "github.com/openshift/origin/pkg/build/admission/overrides/api"
	u "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestBuildOverrideForcePull(t *testing.T) {
	tests := []struct {
		name  string
		build *buildapi.Build
	}{
		{
			name:  "build - custom",
			build: u.Build().WithCustomStrategy().AsBuild(),
		},
		{
			name:  "build - docker",
			build: u.Build().WithDockerStrategy().AsBuild(),
		},
		{
			name:  "build - source",
			build: u.Build().WithSourceStrategy().AsBuild(),
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			overrides := NewBuildOverrides(&overridesapi.BuildOverridesConfig{ForcePull: true})
			pod := u.Pod().WithBuild(t, test.build, "v1")
			err := overrides.Admit(pod.ToAttributes())
			if err != nil {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}
			build := pod.GetBuild(t)
			strategy := build.Spec.Strategy
			switch {
			case strategy.CustomStrategy != nil:
				if strategy.CustomStrategy.ForcePull == false {
					t.Errorf("%s (%s): force pull was false", test.name, op)
				}
				if pod.Spec.Containers[0].ImagePullPolicy != kapi.PullAlways {
					t.Errorf("%s (%s): image pull policy is not PullAlways", test.name, op)
				}
				if pod.Spec.InitContainers[0].ImagePullPolicy != kapi.PullAlways {
					t.Errorf("%s (%s): image pull policy is not PullAlways", test.name, op)
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

func TestLabelOverrides(t *testing.T) {
	tests := []struct {
		buildLabels    []buildapi.ImageLabel
		overrideLabels []buildapi.ImageLabel
		expected       []buildapi.ImageLabel
	}{
		{
			buildLabels:    nil,
			overrideLabels: nil,
			expected:       nil,
		},
		{
			buildLabels: nil,
			overrideLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			overrideLabels: nil,
			expected: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "public",
				},
			},
			overrideLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
			},
			overrideLabels: []buildapi.ImageLabel{
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
	}

	for i, test := range tests {
		overridesConfig := &overridesapi.BuildOverridesConfig{
			ImageLabels: test.overrideLabels,
		}

		admitter := NewBuildOverrides(overridesConfig)
		pod := u.Pod().WithBuild(t, u.Build().WithImageLabels(test.buildLabels).AsBuild(), "v1")
		err := admitter.Admit(pod.ToAttributes())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		build := pod.GetBuild(t)

		result := build.Spec.Output.ImageLabels
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("expected[%d]: %v, got: %v", i, test.expected, result)
		}
	}
}
