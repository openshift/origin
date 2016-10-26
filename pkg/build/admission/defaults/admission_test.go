package defaults

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	u "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestProxyDefaults(t *testing.T) {
	defaultsConfig := &defaultsapi.BuildDefaultsConfig{
		GitHTTPProxy:  "http",
		GitHTTPSProxy: "https",
		GitNoProxy:    "no",
	}

	admitter := BuildDefaults{defaultsConfig}
	pod := u.Pod().WithBuild(t, u.Build().WithDockerStrategy().AsBuild(), "v1")
	err := admitter.ApplyDefaults((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, _, err := buildadmission.GetBuildFromPod((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if build.Spec.Source.Git.HTTPProxy == nil || len(*build.Spec.Source.Git.HTTPProxy) == 0 || *build.Spec.Source.Git.HTTPProxy != "http" {
		t.Errorf("failed to find http proxy in git source")
	}
	if build.Spec.Source.Git.HTTPSProxy == nil || len(*build.Spec.Source.Git.HTTPSProxy) == 0 || *build.Spec.Source.Git.HTTPSProxy != "https" {
		t.Errorf("failed to find http proxy in git source")
	}
	if build.Spec.Source.Git.NoProxy == nil || len(*build.Spec.Source.Git.NoProxy) == 0 || *build.Spec.Source.Git.NoProxy != "no" {
		t.Errorf("failed to find no proxy setting in git source")
	}
}

func TestEnvDefaults(t *testing.T) {
	defaultsConfig := &defaultsapi.BuildDefaultsConfig{
		Env: []kapi.EnvVar{
			{
				Name:  "VAR1",
				Value: "VALUE1",
			},
			{
				Name:  "VAR2",
				Value: "VALUE2",
			},
		},
	}

	admitter := BuildDefaults{defaultsConfig}
	pod := u.Pod().WithBuild(t, u.Build().WithSourceStrategy().AsBuild(), "v1")
	err := admitter.ApplyDefaults((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, _, err := buildadmission.GetBuildFromPod((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := getBuildEnv(build)
	var1found, var2found := false, false
	for _, ev := range *env {
		if ev.Name == "VAR1" {
			if ev.Value != "VALUE1" {
				t.Errorf("unexpected value %s", ev.Value)
			}
			var1found = true
		}
		if ev.Name == "VAR2" {
			if ev.Value != "VALUE2" {
				t.Errorf("unexpected value %s", ev.Value)
			}
			var2found = true
		}
	}
	if !var1found {
		t.Errorf("VAR1 not found")
	}
	if !var2found {
		t.Errorf("VAR2 not found")
	}
}

func TestIncrementalDefaults(t *testing.T) {
	bool_t := true
	defaultsConfig := &defaultsapi.BuildDefaultsConfig{
		SourceStrategyDefaults: &defaultsapi.SourceStrategyDefaultsConfig{
			Incremental: &bool_t,
		},
	}

	admitter := BuildDefaults{defaultsConfig}

	pod := u.Pod().WithBuild(t, u.Build().WithSourceStrategy().AsBuild(), "v1")
	err := admitter.ApplyDefaults((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, _, err := buildadmission.GetBuildFromPod((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*build.Spec.Strategy.SourceStrategy.Incremental {
		t.Errorf("failed to default incremental to true")
	}

	build = u.Build().WithSourceStrategy().AsBuild()
	bool_f := false
	build.Spec.Strategy.SourceStrategy.Incremental = &bool_f
	pod = u.Pod().WithBuild(t, build, "v1")
	err = admitter.ApplyDefaults((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, _, err = buildadmission.GetBuildFromPod((*kapi.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *build.Spec.Strategy.SourceStrategy.Incremental {
		t.Errorf("should not have overridden incremental to true")
	}

}

func TestLabelDefaults(t *testing.T) {
	tests := []struct {
		buildLabels   []buildapi.ImageLabel
		defaultLabels []buildapi.ImageLabel
		expected      []buildapi.ImageLabel
	}{
		{
			buildLabels:   nil,
			defaultLabels: nil,
			expected:      nil,
		},
		{
			buildLabels: nil,
			defaultLabels: []buildapi.ImageLabel{
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
			defaultLabels: nil,
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
			defaultLabels: []buildapi.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "public",
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
			defaultLabels: []buildapi.ImageLabel{
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
		defaultsConfig := &defaultsapi.BuildDefaultsConfig{
			ImageLabels: test.defaultLabels,
		}

		admitter := BuildDefaults{defaultsConfig}
		pod := u.Pod().WithBuild(t, u.Build().WithImageLabels(test.buildLabels).AsBuild(), "v1")
		err := admitter.ApplyDefaults((*kapi.Pod)(pod))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		build := pod.GetBuild(t)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result := build.Spec.Output.ImageLabels
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("expected[%d]: %v, got: %v", i, test.expected, result)
		}
	}
}

func TestBuildDefaultsNodeSelector(t *testing.T) {
	tests := []struct {
		name     string
		build    *buildapi.Build
		defaults map[string]string
		expected map[string]string
	}{
		{
			name:     "build - full add",
			build:    u.Build().AsBuild(),
			defaults: map[string]string{"key1": "default1", "key2": "default2"},
			expected: map[string]string{"key1": "default1", "key2": "default2"},
		},
		{
			name:     "build - ignored",
			build:    u.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			defaults: map[string]string{"key1": "default1", "key2": "default2"},
			expected: map[string]string{"key1": "value1"},
		},
		{
			name:     "build - empty(non-nil) nodeselector",
			build:    u.Build().WithNodeSelector(map[string]string{}).AsBuild(),
			defaults: map[string]string{"key1": "default1"},
			expected: map[string]string{},
		},
	}

	for _, test := range tests {
		defaults := BuildDefaults{config: &defaultsapi.BuildDefaultsConfig{NodeSelector: test.defaults}}
		pod := u.Pod().WithBuild(t, test.build, "v1")
		// normally the pod will have the nodeselectors from the build, due to the pod creation logic
		// in the build controller flow. fake it out here.
		pod.Spec.NodeSelector = test.build.Spec.NodeSelector
		err := defaults.ApplyDefaults((*kapi.Pod)(pod))
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

func TestBuildDefaultsAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		build       *buildapi.Build
		annotations map[string]string
		defaults    map[string]string
		expected    map[string]string
	}{
		{
			name:        "build - nil annotations",
			build:       u.Build().AsBuild(),
			annotations: nil,
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "default1", "key2": "default2"},
		},
		{
			name:        "build - full add",
			build:       u.Build().AsBuild(),
			annotations: map[string]string{"key3": "value3"},
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "default1", "key2": "default2", "key3": "value3"},
		},
		{
			name:        "build - partial add",
			build:       u.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "value1", "key2": "default2"},
		},
	}

	for _, test := range tests {
		defaults := BuildDefaults{config: &defaultsapi.BuildDefaultsConfig{Annotations: test.defaults}}
		pod := u.Pod().WithBuild(t, test.build, "v1")
		pod.Annotations = test.annotations
		err := defaults.ApplyDefaults((*kapi.Pod)(pod))
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
