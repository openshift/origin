package defaults

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	internalcore "k8s.io/kubernetes/pkg/apis/core"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/controller/common"
	testutil "github.com/openshift/origin/pkg/build/controller/common/testutil"
	buildutil "github.com/openshift/origin/pkg/build/util"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func TestProxyDefaults(t *testing.T) {
	defaultsConfig := &configapi.BuildDefaultsConfig{
		GitHTTPProxy:  "http",
		GitHTTPSProxy: "https",
		GitNoProxy:    "no",
	}

	admitter := BuildDefaults{defaultsConfig}
	pod := testutil.Pod().WithBuild(t, testutil.Build().WithDockerStrategy().AsBuild())
	err := admitter.ApplyDefaults((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, err := common.GetBuildFromPod((*corev1.Pod)(pod))
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
	defaultsConfig := &configapi.BuildDefaultsConfig{
		Env: []internalcore.EnvVar{
			{
				Name:  "VAR1",
				Value: "VALUE1",
			},
			{
				Name:  "VAR2",
				Value: "VALUE2",
			},
			{
				Name:  "GIT_SSL_NO_VERIFY",
				Value: "true",
			},
		},
	}

	admitter := BuildDefaults{defaultsConfig}
	pod := testutil.Pod().WithBuild(t, testutil.Build().WithSourceStrategy().AsBuild())
	err := admitter.ApplyDefaults((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, err := common.GetBuildFromPod((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := buildutil.GetBuildEnv(build)
	var1found, var2found := false, false
	for _, ev := range env {
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

	gitSSL := false
	for _, ev := range pod.Spec.Containers[0].Env {
		if ev.Name == "VAR1" || ev.Name == "VAR2" {
			t.Errorf("non whitelisted key %v found", ev.Name)
		}
		if ev.Name == "GIT_SSL_NO_VERIFY" {
			gitSSL = true
		}
	}
	if !gitSSL {
		t.Errorf("GIT_SSL_NO_VERIFY key not found")
	}

	// custom builds should have the defaulted env vars applied to the build pod
	pod = testutil.Pod().WithBuild(t, testutil.Build().WithCustomStrategy().AsBuild())
	err = admitter.ApplyDefaults((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var1found, var2found, gitSSL = false, false, false
	for _, ev := range pod.Spec.Containers[0].Env {
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
		if ev.Name == "GIT_SSL_NO_VERIFY" {
			gitSSL = true
		}
	}

	if !var1found {
		t.Errorf("VAR1 not found")
	}
	if !var2found {
		t.Errorf("VAR2 not found")
	}
	if !gitSSL {
		t.Errorf("GIT_SSL_NO_VERIFY key not found")
	}
}

func TestIncrementalDefaults(t *testing.T) {
	bool_t := true
	defaultsConfig := &configapi.BuildDefaultsConfig{
		SourceStrategyDefaults: &configapi.SourceStrategyDefaultsConfig{
			Incremental: &bool_t,
		},
	}

	admitter := BuildDefaults{defaultsConfig}

	pod := testutil.Pod().WithBuild(t, testutil.Build().WithSourceStrategy().AsBuild())
	err := admitter.ApplyDefaults((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, err := common.GetBuildFromPod((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*build.Spec.Strategy.SourceStrategy.Incremental {
		t.Errorf("failed to default incremental to true")
	}

	build = testutil.Build().WithSourceStrategy().AsBuild()
	bool_f := false
	build.Spec.Strategy.SourceStrategy.Incremental = &bool_f
	pod = testutil.Pod().WithBuild(t, build)
	err = admitter.ApplyDefaults((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build, err = common.GetBuildFromPod((*corev1.Pod)(pod))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *build.Spec.Strategy.SourceStrategy.Incremental {
		t.Errorf("should not have overridden incremental to true")
	}

}

func TestLabelDefaults(t *testing.T) {
	tests := []struct {
		buildLabels   []buildv1.ImageLabel
		defaultLabels []buildv1.ImageLabel
		expected      []buildv1.ImageLabel
	}{
		{
			buildLabels:   nil,
			defaultLabels: nil,
			expected:      nil,
		},
		{
			buildLabels: nil,
			defaultLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
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
			buildLabels: []buildv1.ImageLabel{
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
			expected: []buildv1.ImageLabel{
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
			buildLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
			},
			defaultLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "public",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
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
			buildLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
			},
			defaultLabels: []buildv1.ImageLabel{
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
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
		internalLabels := []buildapi.ImageLabel{}
		for _, l := range test.defaultLabels {
			internalLabel := buildapi.ImageLabel{}
			if err := legacyscheme.Scheme.Convert(&l, &internalLabel, nil); err != nil {
				panic(err)
			}
			internalLabels = append(internalLabels, internalLabel)
		}
		defaultsConfig := &configapi.BuildDefaultsConfig{
			ImageLabels: internalLabels,
		}

		admitter := BuildDefaults{defaultsConfig}
		pod := testutil.Pod().WithBuild(t, testutil.Build().WithImageLabels(test.buildLabels).AsBuild())
		err := admitter.ApplyDefaults((*corev1.Pod)(pod))
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
		build    *buildv1.Build
		defaults map[string]string
		expected map[string]string
	}{
		{
			name:     "build - full add",
			build:    testutil.Build().AsBuild(),
			defaults: map[string]string{"key1": "default1", "key2": "default2"},
			expected: map[string]string{"key1": "default1", "key2": "default2"},
		},
		{
			name:     "build - ignored",
			build:    testutil.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			defaults: map[string]string{"key1": "default1", "key2": "default2"},
			expected: map[string]string{"key1": "value1"},
		},
		{
			name:     "build - empty(non-nil) nodeselector",
			build:    testutil.Build().WithNodeSelector(map[string]string{}).AsBuild(),
			defaults: map[string]string{"key1": "default1"},
			expected: map[string]string{},
		},
	}

	for _, test := range tests {
		defaults := BuildDefaults{Config: &configapi.BuildDefaultsConfig{NodeSelector: test.defaults}}
		pod := testutil.Pod().WithBuild(t, test.build)
		// normally the pod will have the nodeselectors from the build, due to the pod creation logic
		// in the build controller flow. fake it out here.
		pod.Spec.NodeSelector = test.build.Spec.NodeSelector
		err := defaults.ApplyDefaults((*corev1.Pod)(pod))
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
		build       *buildv1.Build
		annotations map[string]string
		defaults    map[string]string
		expected    map[string]string
	}{
		{
			name:        "build - nil annotations",
			build:       testutil.Build().AsBuild(),
			annotations: nil,
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "default1", "key2": "default2"},
		},
		{
			name:        "build - full add",
			build:       testutil.Build().AsBuild(),
			annotations: map[string]string{"key3": "value3"},
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "default1", "key2": "default2", "key3": "value3"},
		},
		{
			name:        "build - partial add",
			build:       testutil.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			defaults:    map[string]string{"key1": "default1", "key2": "default2"},
			expected:    map[string]string{"key1": "value1", "key2": "default2"},
		},
	}

	for _, test := range tests {
		defaults := BuildDefaults{Config: &configapi.BuildDefaultsConfig{Annotations: test.defaults}}
		pod := testutil.Pod().WithBuild(t, test.build)
		pod.Annotations = test.annotations
		err := defaults.ApplyDefaults((*corev1.Pod)(pod))
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
func TestResourceDefaults(t *testing.T) {
	tests := map[string]struct {
		DefaultResource  kapi.ResourceRequirements
		BuildResource    corev1.ResourceRequirements
		ExpectedResource corev1.ResourceRequirements
	}{
		"BuildDefaults plugin and Build object both defined resource limits and requests": {
			DefaultResource: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("20"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("2G"),
				},
			},
			BuildResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("40"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("4G"),
				},
			},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("40"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("4G"),
				},
			},
		},
		"BuildDefaults plugin defined limits and requests, Build object defined resource requests": {
			DefaultResource: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("20"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("2G"),
				},
			},
			BuildResource: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
			},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
			},
		},
		"BuildDefaults plugin defined limits and requests, Build object defined resource limits": {
			DefaultResource: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("20"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("2G"),
				},
			},
			BuildResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
			},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("3G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("2G"),
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("20"),
				},
			},
		},
		"BuildDefaults plugin defined nothing, Build object defined resource limits": {
			DefaultResource: kapi.ResourceRequirements{},
			BuildResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("20"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("2G"),
				},
			},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("20"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("2G"),
				},
			},
		},
		"BuildDefaults plugin and Build object defined nothing": {
			DefaultResource:  kapi.ResourceRequirements{},
			BuildResource:    corev1.ResourceRequirements{},
			ExpectedResource: corev1.ResourceRequirements{},
		},
		"BuildDefaults plugin defined limits and requests, Build object defined nothing": {
			DefaultResource: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("20"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("2G"),
				},
			},
			BuildResource: corev1.ResourceRequirements{},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("20"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("2G"),
				},
			},
		},
		"BuildDefaults plugin defined part of limits and requests, Build object defined part of limits and  requests": {
			DefaultResource: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU): resource.MustParse("10"),
				},
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("2G"),
				},
			},
			BuildResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU): resource.MustParse("30"),
				},
			},
			ExpectedResource: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("30"),
					corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("2G"),
				},
			},
		},
	}

	for name, test := range tests {
		defaults := BuildDefaults{Config: &configapi.BuildDefaultsConfig{Resources: test.DefaultResource}}

		build := testutil.Build().WithSourceStrategy().AsBuild()
		build.Spec.Resources = test.BuildResource
		pod := testutil.Pod().WithBuild(t, build)

		// normally the buildconfig resources would be applied to the pod
		// when it was created, but this pod didn't get created by the normal
		// pod creation flow, so fake this out.
		for i := range pod.Spec.InitContainers {
			pod.Spec.InitContainers[i].Resources = test.BuildResource
		}
		for i := range pod.Spec.Containers {
			pod.Spec.Containers[i].Resources = test.BuildResource
		}
		err := defaults.ApplyDefaults((*corev1.Pod)(pod))
		if err != nil {
			t.Fatalf("%v :unexpected error: %v", name, err)
		}
		build, err = common.GetBuildFromPod((*corev1.Pod)(pod))
		if err != nil {
			t.Fatalf("%v :unexpected error: %v", name, err)
		}
		if !kapihelper.Semantic.DeepEqual(test.ExpectedResource, build.Spec.Resources) {
			t.Fatalf("%v:Build resource expected expected=actual, \nexpected: %#v\n\nactual: %#v\n", name, test.ExpectedResource,
				build.Spec.Resources)
		}

		allContainers := append([]corev1.Container{}, pod.Spec.Containers...)
		allContainers = append(allContainers, pod.Spec.InitContainers...)
		for i, c := range allContainers {
			if !kapihelper.Semantic.DeepEqual(test.ExpectedResource, c.Resources) {
				t.Fatalf("%v: Pod container %d resource expected expected=actual, got expected:\n%#v\nactual:\n%#v", name, i, test.ExpectedResource, c.Resources)
			}
		}
	}
}

func TestSetBuildLogLevel(t *testing.T) {
	build := testutil.Build().WithSourceStrategy()
	pod := testutil.Pod().WithEnvVar("BUILD", "foo")
	setPodLogLevelFromBuild((*corev1.Pod)(pod), build.AsBuild())

	if len(pod.Spec.Containers[0].Args) == 0 {
		t.Errorf("Builds pod loglevel was not set")
	}

	if pod.Spec.Containers[0].Args[0] != "--loglevel=0" {
		t.Errorf("Default build pod loglevel was not set to 0")
	}

	build = testutil.Build().WithSourceStrategy()
	pod = testutil.Pod().WithEnvVar("BUILD", "foo")
	build.Spec.Strategy.SourceStrategy.Env = []corev1.EnvVar{{Name: "BUILD_LOGLEVEL", Value: "7", ValueFrom: nil}}
	setPodLogLevelFromBuild((*corev1.Pod)(pod), build.AsBuild())

	if pod.Spec.Containers[0].Args[0] != "--loglevel=7" {
		t.Errorf("Build pod loglevel was not transferred from BUILD_LOGLEVEL environment variable: %#v", pod)
	}

}
