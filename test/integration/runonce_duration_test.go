//  +build integration

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	pluginapi "github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func testRunOnceDurationPod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Name = "testpod"
	pod.Spec.RestartPolicy = kapi.RestartPolicyNever
	pod.Spec.Containers = []kapi.Container{
		{
			Name:  "container",
			Image: "test/image",
		},
	}
	return pod
}

func TestRunOnceDurationAdmissionPlugin(t *testing.T) {
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsOverride: &secs,
	}
	kclient := setupRunOnceDurationTest(t, config, nil)
	pod, err := kclient.Pods(testutil.Namespace()).Create(testRunOnceDurationPod())
	if err != nil {
		t.Fatalf("Unexpected: %v", err)
	}
	if pod.Spec.ActiveDeadlineSeconds == nil || *pod.Spec.ActiveDeadlineSeconds != 3600 {
		t.Errorf("Unexpected value for pod.ActiveDeadlineSeconds %v", pod.Spec.ActiveDeadlineSeconds)
	}
}

func TestRunOnceDurationAdmissionPluginProjectOverride(t *testing.T) {
	var secs int64 = 3600
	config := &pluginapi.RunOnceDurationConfig{
		ActiveDeadlineSecondsOverride: &secs,
	}
	nsAnnotations := map[string]string{
		pluginapi.ActiveDeadlineSecondsOverrideAnnotation: "100",
	}
	kclient := setupRunOnceDurationTest(t, config, nsAnnotations)
	pod, err := kclient.Pods(testutil.Namespace()).Create(testRunOnceDurationPod())
	if err != nil {
		t.Fatalf("Unexpected: %v", err)
	}
	if pod.Spec.ActiveDeadlineSeconds == nil || *pod.Spec.ActiveDeadlineSeconds != 100 {
		t.Errorf("Unexpected value for pod.ActiveDeadlineSeconds %v", pod.Spec.ActiveDeadlineSeconds)
	}
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
	if err := testserver.WaitForServiceAccounts(kubeClient, testutil.Namespace(), []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return kubeClient
}
