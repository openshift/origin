package overrides

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/admission"

	u "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	overridesapi "github.com/openshift/origin/pkg/build/controller/build/apis/overrides"

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
			overrides := BuildOverrides{config: &overridesapi.BuildOverridesConfig{ForcePull: true}}
			pod := u.Pod().WithBuild(t, test.build, "v1")
			err := overrides.ApplyOverrides((*v1.Pod)(pod))
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
				if pod.Spec.Containers[0].ImagePullPolicy != v1.PullAlways {
					t.Errorf("%s (%s): image pull policy is not PullAlways", test.name, op)
				}
				if pod.Spec.InitContainers[0].ImagePullPolicy != v1.PullAlways {
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

		admitter := BuildOverrides{overridesConfig}
		pod := u.Pod().WithBuild(t, u.Build().WithImageLabels(test.buildLabels).AsBuild(), "v1")
		err := admitter.ApplyOverrides((*v1.Pod)(pod))
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

func TestBuildOverrideNodeSelector(t *testing.T) {
	tests := []struct {
		name      string
		build     *buildapi.Build
		overrides map[string]string
		expected  map[string]string
	}{
		{
			name:      "build - full override",
			build:     u.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			overrides: map[string]string{"key1": "override1", "key2": "override2"},
			expected:  map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:      "build - partial override",
			build:     u.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			overrides: map[string]string{"key2": "override2"},
			expected:  map[string]string{"key1": "value1", "key2": "override2"},
		},
	}

	for _, test := range tests {
		overrides := BuildOverrides{config: &overridesapi.BuildOverridesConfig{NodeSelector: test.overrides}}
		pod := u.Pod().WithBuild(t, test.build, "v1")
		// normally the pod will have the nodeselectors from the build, due to the pod creation logic
		// in the build controller flow. fake it out here.
		pod.Spec.NodeSelector = test.build.Spec.NodeSelector
		err := overrides.ApplyOverrides((*v1.Pod)(pod))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}
		if len(pod.Spec.NodeSelector) != len(test.expected) {
			t.Errorf("%s: incorrect number of selectors, expected %v, got %v", test.name, test.expected, pod.Spec.NodeSelector)
		}
		for k, v := range pod.Spec.NodeSelector {
			if ev, ok := test.expected[k]; !ok || ev != v {
				t.Errorf("%s: incorrect selector value for key %s, expected %s, got %s", test.name, k, ev, v)
			}
		}
	}
}

func TestBuildOverrideAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		build       *buildapi.Build
		annotations map[string]string
		overrides   map[string]string
		expected    map[string]string
	}{
		{
			name:        "build - nil annotations",
			build:       u.Build().AsBuild(),
			annotations: nil,
			overrides:   map[string]string{"key1": "override1", "key2": "override2"},
			expected:    map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:        "build - full override",
			build:       u.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			overrides:   map[string]string{"key1": "override1", "key2": "override2"},
			expected:    map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:        "build - partial override",
			build:       u.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			overrides:   map[string]string{"key2": "override2"},
			expected:    map[string]string{"key1": "value1", "key2": "override2"},
		},
	}

	for _, test := range tests {
		overrides := BuildOverrides{config: &overridesapi.BuildOverridesConfig{Annotations: test.overrides}}
		pod := u.Pod().WithBuild(t, test.build, "v1")
		pod.Annotations = test.annotations
		err := overrides.ApplyOverrides((*v1.Pod)(pod))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}
		if len(pod.Annotations) != len(test.expected) {
			t.Errorf("%s: incorrect number of annotations, expected %v, got %v", test.name, test.expected, pod.Annotations)
		}
		for k, v := range pod.Annotations {
			if ev, ok := test.expected[k]; !ok || ev != v {
				t.Errorf("%s: incorrect annotation value for key %s, expected %s, got %s", test.name, k, ev, v)
			}
		}
	}
}
