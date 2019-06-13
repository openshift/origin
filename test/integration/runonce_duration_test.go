package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	pluginapi "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

func testRunOnceDurationPod(activeDeadlineSeconds int64) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.GenerateName = "testpod"
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.Containers = []corev1.Container{
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

func testPodDuration(t *testing.T, name string, kclientset kubernetes.Interface, pod *corev1.Pod, expected int64) {
	// Pod with no duration set
	pod, err := kclientset.CoreV1().Pods(testutil.Namespace()).Create(pod)
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
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &secs,
	}
	kclientset, fn := setupRunOnceDurationTest(t, config, nil)
	defer fn()

	testPodDuration(t, "global, no duration", kclientset, testRunOnceDurationPod(0), 3600)
	testPodDuration(t, "global, larger duration", kclientset, testRunOnceDurationPod(7200), 3600)
	testPodDuration(t, "global, smaller duration", kclientset, testRunOnceDurationPod(100), 100)
}

func TestRunOnceDurationAdmissionPluginProjectLimit(t *testing.T) {
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsLimit: &secs,
	}
	nsAnnotations := map[string]string{
		pluginapi.ActiveDeadlineSecondsLimitAnnotation: "100",
	}
	kclientset, fn := setupRunOnceDurationTest(t, config, nsAnnotations)
	defer fn()
	testPodDuration(t, "project, no duration", kclientset, testRunOnceDurationPod(0), 100)
	testPodDuration(t, "project, larger duration", kclientset, testRunOnceDurationPod(7200), 100)
	testPodDuration(t, "project, smaller duration", kclientset, testRunOnceDurationPod(50), 50)
}

func setupRunOnceDurationTest(t *testing.T, pluginConfig *pluginapi.RunOnceDurationConfig, nsAnnotations map[string]string) (kubernetes.Interface, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
		"autoscaling.openshift.io/RunOnceDuration": {
			Configuration: pluginConfig,
		},
	}
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	ns := &corev1.Namespace{}
	ns.Name = testutil.Namespace()
	ns.Annotations = nsAnnotations
	_, err = kubeClientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return kubeClientset, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}
