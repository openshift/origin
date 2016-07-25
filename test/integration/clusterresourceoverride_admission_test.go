package integration

import (
	"testing"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	_ "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/install"
)

func TestClusterResourceOverridePluginWithNoLimits(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	config := &overrideapi.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     100,
		CPURequestToLimitPercent:    50,
		MemoryRequestToLimitPercent: 50,
	}
	kubeClient := setupClusterResourceOverrideTest(t, config)
	podHandler := kubeClient.Pods(testutil.Namespace())

	// test with no limits object present

	podCreated, err := podHandler.Create(testClusterResourceOverridePod("limitless", "2Gi", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if memory := podCreated.Spec.Containers[0].Resources.Requests.Memory(); memory.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("limitlesspod: Memory did not match expected 1Gi: %#v", memory)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Limits.Cpu(); cpu.Cmp(resource.MustParse("2")) != 0 {
		t.Errorf("limitlesspod: CPU limit did not match expected 2 core: %#v", cpu)
	}
	if cpu := podCreated.Spec.Containers[0].Resources.Requests.Cpu(); cpu.Cmp(resource.MustParse("1")) != 0 {
		t.Errorf("limitlesspod: CPU req did not match expected 1 core: %#v", cpu)
	}
}

func TestClusterResourceOverridePluginWithLimits(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	config := &overrideapi.ClusterResourceOverrideConfig{
		LimitCPUToMemoryPercent:     100,
		CPURequestToLimitPercent:    50,
		MemoryRequestToLimitPercent: 50,
	}
	kubeClient := setupClusterResourceOverrideTest(t, config)
	podHandler := kubeClient.Pods(testutil.Namespace())
	limitHandler := kubeClient.LimitRanges(testutil.Namespace())

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
		ObjectMeta: kapi.ObjectMeta{Name: "limit"},
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

	// set it up so that the overrides create resources that fail validation
	_, err = podHandler.Create(testClusterResourceOverridePod("limit-with-default-fail", "128Mi", "1"))
	if err == nil {
		t.Errorf("limit-with-default-fail: expected to be forbidden")
	} else if !apierrors.IsForbidden(err) {
		t.Errorf("limit-with-default-fail: unexpected error: %v", err)
	}
}

func setupClusterResourceOverrideTest(t *testing.T, pluginConfig *overrideapi.ClusterResourceOverrideConfig) kclient.Interface {
	testutil.RequireEtcd(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	// fill in possibly-empty config values
	if masterConfig.KubernetesMasterConfig == nil {
		masterConfig.KubernetesMasterConfig = &api.KubernetesMasterConfig{}
	}
	kubeMaster := masterConfig.KubernetesMasterConfig
	if kubeMaster.AdmissionConfig.PluginConfig == nil {
		kubeMaster.AdmissionConfig.PluginConfig = map[string]api.AdmissionPluginConfig{}
	}
	// set our config as desired
	kubeMaster.AdmissionConfig.PluginConfig[overrideapi.PluginName] =
		api.AdmissionPluginConfig{Configuration: pluginConfig}

	// start up a server and return useful clients to that server
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	// need to create a project and return client for project admin
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, testutil.Namespace(), "peon")
	if err != nil {
		t.Fatal(err)
	}
	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Fatal(err)
	}
	return clusterAdminKubeClient
}

func testClusterResourceOverridePod(name string, memory string, cpu string) *kapi.Pod {
	resources := kapi.ResourceRequirements{
		Limits:   testResourceList(memory, cpu),
		Requests: kapi.ResourceList{},
	}
	container := kapi.Container{Name: name, Image: "scratch", Resources: resources}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: name},
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
