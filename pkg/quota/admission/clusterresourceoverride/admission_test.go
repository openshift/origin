package clusterresourceoverride

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/validation"

	_ "github.com/openshift/origin/pkg/api/install"
)

const (
	yamlConfig = `
apiVersion: v1
kind: ClusterResourceOverrideConfig
limitCPUToMemoryPercent: 100
cpuRequestToLimitPercent: 10
memoryRequestToLimitPercent: 25
`
	invalidConfig = `
apiVersion: v1
kind: ClusterResourceOverrideConfig
cpuRequestToLimitPercent: 200
`
	invalidConfig2 = `
apiVersion: v1
kind: ClusterResourceOverrideConfig
`
)

var (
	deserializedYamlConfig = &api.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     100,
		CPURequestToLimitPercent:    10,
		MemoryRequestToLimitPercent: 25,
	}
)

func TestConfigReader(t *testing.T) {
	initialConfig := testConfig(10, 20, 30)
	serializedConfig, serializationErr := configapilatest.WriteYAML(initialConfig)
	if serializationErr != nil {
		t.Fatalf("WriteYAML: config serialize failed: %v", serializationErr)
	}

	tests := []struct {
		name           string
		config         io.Reader
		expectErr      bool
		expectNil      bool
		expectInvalid  bool
		expectedConfig *api.ClusterResourceOverrideConfig
	}{
		{
			name:      "process nil config",
			config:    nil,
			expectNil: true,
		}, {
			name:           "deserialize initialConfig yaml",
			config:         bytes.NewReader(serializedConfig),
			expectedConfig: initialConfig,
		}, {
			name:      "completely broken config",
			config:    bytes.NewReader([]byte("asdfasdfasdF")),
			expectErr: true,
		}, {
			name:           "deserialize yamlConfig",
			config:         bytes.NewReader([]byte(yamlConfig)),
			expectedConfig: deserializedYamlConfig,
		}, {
			name:          "choke on out-of-bounds ratio",
			config:        bytes.NewReader([]byte(invalidConfig)),
			expectInvalid: true,
			expectErr:     true,
		}, {
			name:          "complain about no settings",
			config:        bytes.NewReader([]byte(invalidConfig2)),
			expectInvalid: true,
			expectErr:     true,
		},
	}
	for _, test := range tests {
		config, err := ReadConfig(test.config)
		if test.expectErr && err == nil {
			t.Errorf("%s: expected error", test.name)
		} else if !test.expectErr && err != nil {
			t.Errorf("%s: expected no error, saw %v", test.name, err)
		}
		if err == nil {
			if test.expectNil && config != nil {
				t.Errorf("%s: expected nil config, but saw: %v", test.name, config)
			} else if !test.expectNil && config == nil {
				t.Errorf("%s: expected config, but got nil", test.name)
			}
		}
		if config != nil {
			if test.expectedConfig != nil && *test.expectedConfig != *config {
				t.Errorf("%s: expected %v from reader, but got %v", test.name, test.expectErr, config)
			}
			if err := validation.Validate(config); test.expectInvalid && len(err) == 0 {
				t.Errorf("%s: expected validation to fail, but it passed", test.name)
			} else if !test.expectInvalid && len(err) > 0 {
				t.Errorf("%s: expected validation to pass, but it failed with %v", test.name, err)
			}
		}
	}
}

func TestLimitRequestAdmission(t *testing.T) {
	tests := []struct {
		name               string
		config             *api.ClusterResourceOverrideConfig
		pod                *kapi.Pod
		expectedMemRequest resource.Quantity
		expectedCpuLimit   resource.Quantity
		expectedCpuRequest resource.Quantity
		namespace          *kapi.Namespace
	}{
		{
			name:               "ignore pods that have no memory limit specified",
			config:             testConfig(100, 50, 50),
			pod:                testBestEffortPod(),
			expectedMemRequest: resource.MustParse("0"),
			expectedCpuLimit:   resource.MustParse("0"),
			expectedCpuRequest: resource.MustParse("0"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "test floor for memory and cpu",
			config:             testConfig(100, 50, 50),
			pod:                testPod("1Mi", "0", "0", "0"),
			expectedMemRequest: resource.MustParse("1Mi"),
			expectedCpuLimit:   resource.MustParse("1m"),
			expectedCpuRequest: resource.MustParse("1m"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "nil config",
			config:             nil,
			pod:                testPod("1", "1", "1", "1"),
			expectedMemRequest: resource.MustParse("1"),
			expectedCpuLimit:   resource.MustParse("1"),
			expectedCpuRequest: resource.MustParse("1"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "all values are adjusted",
			config:             testConfig(100, 50, 50),
			pod:                testPod("1Gi", "0", "2000m", "0"),
			expectedMemRequest: resource.MustParse("512Mi"),
			expectedCpuLimit:   resource.MustParse("1"),
			expectedCpuRequest: resource.MustParse("500m"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "just requests are adjusted",
			config:             testConfig(0, 50, 50),
			pod:                testPod("10Mi", "0", "50m", "0"),
			expectedMemRequest: resource.MustParse("5Mi"),
			expectedCpuLimit:   resource.MustParse("50m"),
			expectedCpuRequest: resource.MustParse("25m"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "project annotation disables overrides",
			config:             testConfig(0, 50, 50),
			pod:                testPod("10Mi", "0", "50m", "0"),
			expectedMemRequest: resource.MustParse("0"),
			expectedCpuLimit:   resource.MustParse("50m"),
			expectedCpuRequest: resource.MustParse("0"),
			namespace:          fakeNamespace(false),
		},
		{
			name:               "large values don't overflow",
			config:             testConfig(100, 50, 50),
			pod:                testPod("1Ti", "0", "0", "0"),
			expectedMemRequest: resource.MustParse("512Gi"),
			expectedCpuLimit:   resource.MustParse("1024"),
			expectedCpuRequest: resource.MustParse("512"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "little values mess things up",
			config:             testConfig(500, 10, 10),
			pod:                testPod("1.024Mi", "0", "0", "0"),
			expectedMemRequest: resource.MustParse("1Mi"),
			expectedCpuLimit:   resource.MustParse("5m"),
			expectedCpuRequest: resource.MustParse("1m"),
			namespace:          fakeNamespace(true),
		},
		{
			name:               "test fractional memory requests round up",
			config:             testConfig(500, 10, 60),
			pod:                testPod("512Mi", "0", "0", "0"),
			expectedMemRequest: resource.MustParse("307Mi"),
			expectedCpuLimit:   resource.MustParse("2.5"),
			expectedCpuRequest: resource.MustParse("250m"),
			namespace:          fakeNamespace(true),
		},
	}

	for _, test := range tests {
		c, err := newClusterResourceOverride(fake.NewSimpleClientset(), test.config)
		if err != nil {
			t.Errorf("%s: config de/serialize failed: %v", test.name, err)
			continue
		}
		c.(*clusterResourceOverridePlugin).SetProjectCache(fakeProjectCache(test.namespace))
		attrs := admission.NewAttributesRecord(test.pod, nil, unversioned.GroupVersionKind{}, test.namespace.Name, "name", kapi.Resource("pods").WithVersion("version"), "", admission.Create, fakeUser())
		if err = c.Admit(attrs); err != nil {
			t.Errorf("%s: admission controller returned error: %v", test.name, err)
			continue
		}
		resources := test.pod.Spec.InitContainers[0].Resources // only test one container
		if actual := resources.Requests[kapi.ResourceMemory]; test.expectedMemRequest.Cmp(actual) != 0 {
			t.Errorf("%s: memory requests do not match; %v should be %v", test.name, actual, test.expectedMemRequest)
		}
		if actual := resources.Requests[kapi.ResourceCPU]; test.expectedCpuRequest.Cmp(actual) != 0 {
			t.Errorf("%s: cpu requests do not match; %v should be %v", test.name, actual, test.expectedCpuRequest)
		}
		if actual := resources.Limits[kapi.ResourceCPU]; test.expectedCpuLimit.Cmp(actual) != 0 {
			t.Errorf("%s: cpu limits do not match; %v should be %v", test.name, actual, test.expectedCpuLimit)
		}

		resources = test.pod.Spec.Containers[0].Resources // only test one container
		if actual := resources.Requests[kapi.ResourceMemory]; test.expectedMemRequest.Cmp(actual) != 0 {
			t.Errorf("%s: memory requests do not match; %v should be %v", test.name, actual, test.expectedMemRequest)
		}
		if actual := resources.Requests[kapi.ResourceCPU]; test.expectedCpuRequest.Cmp(actual) != 0 {
			t.Errorf("%s: cpu requests do not match; %v should be %v", test.name, actual, test.expectedCpuRequest)
		}
		if actual := resources.Limits[kapi.ResourceCPU]; test.expectedCpuLimit.Cmp(actual) != 0 {
			t.Errorf("%s: cpu limits do not match; %v should be %v", test.name, actual, test.expectedCpuLimit)
		}
	}
}

func testBestEffortPod() *kapi.Pod {
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			InitContainers: []kapi.Container{
				{
					Resources: kapi.ResourceRequirements{},
				},
			},
			Containers: []kapi.Container{
				{
					Resources: kapi.ResourceRequirements{},
				},
			},
		},
	}
}

func testPod(memLimit string, memRequest string, cpuLimit string, cpuRequest string) *kapi.Pod {
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			InitContainers: []kapi.Container{
				{
					Resources: kapi.ResourceRequirements{
						Limits: kapi.ResourceList{
							kapi.ResourceCPU:    resource.MustParse(cpuLimit),
							kapi.ResourceMemory: resource.MustParse(memLimit),
						},
						Requests: kapi.ResourceList{
							kapi.ResourceCPU:    resource.MustParse(cpuRequest),
							kapi.ResourceMemory: resource.MustParse(memRequest),
						},
					},
				},
			},
			Containers: []kapi.Container{
				{
					Resources: kapi.ResourceRequirements{
						Limits: kapi.ResourceList{
							kapi.ResourceCPU:    resource.MustParse(cpuLimit),
							kapi.ResourceMemory: resource.MustParse(memLimit),
						},
						Requests: kapi.ResourceList{
							kapi.ResourceCPU:    resource.MustParse(cpuRequest),
							kapi.ResourceMemory: resource.MustParse(memRequest),
						},
					},
				},
			},
		},
	}
}

func fakeUser() user.Info {
	return &user.DefaultInfo{
		Name: "testuser",
	}
}

var nsIndex = 0

func fakeNamespace(pluginEnabled bool) *kapi.Namespace {
	nsIndex++
	ns := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("fakeNS%d", nsIndex),
			Annotations: map[string]string{},
		},
	}
	if !pluginEnabled {
		ns.Annotations[clusterResourceOverrideAnnotation] = "false"
	}
	return ns
}

func fakeProjectCache(ns *kapi.Namespace) *projectcache.ProjectCache {
	store := projectcache.NewCacheStore(cache.MetaNamespaceKeyFunc)
	store.Add(ns)
	return projectcache.NewFake((&ktestclient.Fake{}).Namespaces(), store, "")
}

func testConfig(lc2mr int64, cr2lr int64, mr2lr int64) *api.ClusterResourceOverrideConfig {
	return &api.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     lc2mr,
		CPURequestToLimitPercent:    cr2lr,
		MemoryRequestToLimitPercent: mr2lr,
	}
}
