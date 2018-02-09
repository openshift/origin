package integration

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	_ "github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride/install"
)

func TestClusterResourceOverridePluginWithNoLimits(t *testing.T) {
	config := &overrideapi.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     100,
		CPURequestToLimitPercent:    50,
		MemoryRequestToLimitPercent: 50,
	}
	kubeClientset, fn := setupClusterResourceOverrideTest(t, config)
	defer fn()
	podHandler := kubeClientset.Core().Pods(testutil.Namespace())

	// test with no limits object present

	podCreated, err := podHandler.Create(testClusterResourceOverridePod("limitless", "2Gi", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Limits.Memory(); memory.Cmp(resource.MustParse("2Gi")) != 0 {
		t.Errorf("limitlesspod: Memory limit did not match expected 2Gi: %#v", memory)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Requests.Memory(); memory.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("limitlesspod: Memory req did not match expected 1Gi: %#v", memory)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Limits.Cpu(); cpu.Cmp(resource.MustParse("2")) != 0 {
		t.Errorf("limitlesspod: CPU limit did not match expected 2 core: %#v", cpu)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Requests.Cpu(); cpu.Cmp(resource.MustParse("1")) != 0 {
		t.Errorf("limitlesspod: CPU req did not match expected 1 core: %#v", cpu)
	}
}

func TestClusterResourceOverridePluginWithLimits(t *testing.T) {
	config := &overrideapi.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     100,
		CPURequestToLimitPercent:    50,
		MemoryRequestToLimitPercent: 50,
	}
	kubeClientset, fn := setupClusterResourceOverrideTest(t, config)
	defer fn()
	podHandler := kubeClientset.Core().Pods(testutil.Namespace())
	limitHandler := kubeClientset.Core().LimitRanges(testutil.Namespace())

	// test with limits object with defaults;
	// I wanted to test with a limits object without defaults to see limits forbid an empty resource spec,
	// but found that if defaults aren't set in the limit object, something still fills them in.
	// note: defaults are only used when quantities are *missing*, not when they are 0
	limitItem := kapi.LimitRangeItem{
		Type:                 kapi.LimitTypeContainer,
		Max:                  testResourceList("2Gi", "2"),
		Min:                  testResourceList("128Mi", "200m"),
		Default:              testResourceList("512Mi", "500m"), // note: auto-filled from max if we set that;
		DefaultRequest:       testResourceList("128Mi", "200m"), // filled from max if set, or min if that is set
		MaxLimitRequestRatio: kapi.ResourceList{},
	}
	limit := &kapi.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "limit"},
		Spec:       kapi.LimitRangeSpec{Limits: []kapi.LimitRangeItem{limitItem}},
	}
	_, err := limitHandler.Create(limit)
	if err != nil {
		t.Fatal(err)
	}
	podCreated, err := podHandler.Create(testClusterResourceOverridePod("limit-with-default", "", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Limits.Memory(); memory.Cmp(resource.MustParse("512Mi")) != 0 {
		t.Errorf("limit-with-default: Memory limit did not match default 512Mi: %v", memory)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Requests.Memory(); memory.Cmp(resource.MustParse("256Mi")) != 0 {
		t.Errorf("limit-with-default: Memory req did not match expected 256Mi: %v", memory)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Limits.Cpu(); cpu.Cmp(resource.MustParse("500m")) != 0 {
		t.Errorf("limit-with-default: CPU limit did not match expected 500 mcore: %v", cpu)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Requests.Cpu(); cpu.Cmp(resource.MustParse("250m")) != 0 {
		t.Errorf("limit-with-default: CPU req did not match expected 250 mcore: %v", cpu)
	}

	// CRO admission plugin now ensures that a limit does not fall
	// below the minimum across all containers in the namespace
	// and validation should never fail. Create a POD that would
	// ordinarily fail validation checks but notice that it a)
	// doesn't fail and b) the CPU limits and requests are clamped
	// to the floor of CPU and Memory respectively.
	podCreated, err = podHandler.Create(testClusterResourceOverridePod("limit-with-min-floor-cpu", "128Mi", "1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Limits.Memory(); memory.Cmp(resource.MustParse("128Mi")) != 0 {
		t.Errorf("limit-with-min-floor-cpu: Memory limit did not match default 128Mi: %v", memory)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Requests.Memory(); memory.Cmp(resource.MustParse("128Mi")) != 0 {
		t.Errorf("limit-with-min-floor-cpu: Memory req did not match expected 128Mi: %v", memory)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Limits.Cpu(); cpu.Cmp(resource.MustParse("200m")) != 0 {
		t.Errorf("limit-with-min-floor-cpu: CPU limit did not match expected 200 mcore: %v", cpu)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Requests.Cpu(); cpu.Cmp(resource.MustParse("200m")) != 0 {
		t.Errorf("limit-with-min-floor-cpu: CPU req did not match expected 200 mcore: %v", cpu)
	}
}

func setupClusterResourceOverrideTest(t *testing.T, pluginConfig *overrideapi.ClusterResourceOverrideConfig) (kclientset.Interface, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	// fill in possibly-empty config values
	if masterConfig.KubernetesMasterConfig == nil {
		masterConfig.KubernetesMasterConfig = &config.KubernetesMasterConfig{}
	}
	if masterConfig.AdmissionConfig.PluginConfig == nil {
		masterConfig.AdmissionConfig.PluginConfig = map[string]*config.AdmissionPluginConfig{}
	}
	// set our config as desired
	masterConfig.AdmissionConfig.PluginConfig[overrideapi.PluginName] =
		&config.AdmissionPluginConfig{Configuration: pluginConfig}

	// start up a server and return useful clients to that server
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	// need to create a project and return client for project admin
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, testutil.Namespace(), "peon")
	if err != nil {
		t.Fatal(err)
	}
	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, testutil.Namespace(), []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Fatal(err)
	}
	return clusterAdminKubeClientset, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func testClusterResourceOverridePod(name string, memory string, cpu string) *kapi.Pod {
	resources := kapi.ResourceRequirements{
		Limits:   testResourceList(memory, cpu),
		Requests: kapi.ResourceList{},
	}
	container := kapi.Container{Name: name, Image: "scratch", Resources: resources}
	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       kapi.PodSpec{Containers: []kapi.Container{container}},
	}
	return pod
}

func testResourceList(memory string, cpu string) kapi.ResourceList {
	list := kapi.ResourceList{}
	if memory != "" {
		list[kapi.ResourceMemory] = resource.MustParse(memory)
	}
	if cpu != "" {
		list[kapi.ResourceCPU] = resource.MustParse(cpu)
	}
	return list
}
