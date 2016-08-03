package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	pluginapi "github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func testRunOnceDurationPod(activeDeadlineSeconds int64) *kapi.Pod {
	pod := &kapi.Pod{}
	pod.GenerateName = "testpod"
	pod.Spec.RestartPolicy = kapi.RestartPolicyNever
	pod.Spec.Containers = []kapi.Container{
		{
			Name:  "container",
			Image: "test/image",
		},
	}
	if activeDeadlineSeconds > 0 {
		pod.Spec.ActiveDeadlineSeconds = &activeDeadlineSeconds
	}
	return pod
}

func testPodDuration(t *testing.T, name string, kclient kclient.Interface, pod *kapi.Pod, expected int64) {
	// Pod with no duration set
	pod, err := kclient.Pods(testutil.Namespace()).Create(pod)
	if err != nil {
		t.Fatalf("%s: unexpected: %v", name, err)
	}
	if pod.Spec.ActiveDeadlineSeconds == nil {
		t.Errorf("%s: unexpected nil value for pod.Spec.ActiveDeadlineSeconds", name)
		return
	}
	if *pod.Spec.ActiveDeadlineSeconds != expected {
		t.Errorf("%s: unexpected value for pod.Spec.ActiveDeadlineSeconds: %d. Expected: %d", name, *pod.Spec.ActiveDeadlineSeconds, expected)
	}
}

func TestRunOnceDurationAdmissionPlugin(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &secs,
	}
	kclient := setupRunOnceDurationTest(t, config, nil)

	testPodDuration(t, "global, no duration", kclient, testRunOnceDurationPod(0), 3600)
	testPodDuration(t, "global, larger duration", kclient, testRunOnceDurationPod(7200), 3600)
	testPodDuration(t, "global, smaller duration", kclient, testRunOnceDurationPod(100), 100)
}

func TestRunOnceDurationAdmissionPluginProjectLimit(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &secs,
	}
	nsAnnotations := map[string]string{
		pluginapi.ActiveDeadlineSecondsLimitAnnotation: "100",
	}
	kclient := setupRunOnceDurationTest(t, config, nsAnnotations)
	testPodDuration(t, "project, no duration", kclient, testRunOnceDurationPod(0), 100)
	testPodDuration(t, "project, larger duration", kclient, testRunOnceDurationPod(7200), 100)
	testPodDuration(t, "project, smaller duration", kclient, testRunOnceDurationPod(50), 50)
}

func setupRunOnceDurationTest(t *testing.T, pluginConfig *pluginapi.RunOnceDurationConfig, nsAnnotations map[string]string) kclient.Interface {
	testutil.RequireEtcd(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.KubernetesMasterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
		"RunOnceDuration": {
			Configuration: pluginConfig,
		},
	}
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	ns.Annotations = nsAnnotations
	_, err = kubeClient.Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClient, testutil.Namespace()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return kubeClient
}
