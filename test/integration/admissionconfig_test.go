package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestAlwaysPullImagesOn(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.KubernetesMasterConfig.APIServerArguments["enable-admission-plugins"] = append(
		masterConfig.KubernetesMasterConfig.APIServerArguments["enable-admission-plugins"],
		"AlwaysPullImages")

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
	_, err = kubeClientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	testPod := &corev1.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []corev1.Container{
		{
			Name:            "container",
			Image:           "openshift/origin-pod:notlatest",
			ImagePullPolicy: corev1.PullNever,
		},
	}

	actualPod, err := kubeClientset.CoreV1().Pods(testutil.Namespace()).Create(testPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualPod.Spec.Containers[0].ImagePullPolicy != corev1.PullAlways {
		t.Errorf("expected %v, got %v", kapi.PullAlways, actualPod.Spec.Containers[0].ImagePullPolicy)
	}
}

func TestAlwaysPullImagesOff(t *testing.T) {
	masterConfig, kubeConfigFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}

	ns := &corev1.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	testPod := &corev1.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []corev1.Container{
		{
			Name:            "container",
			Image:           "openshift/origin-pod:notlatest",
			ImagePullPolicy: corev1.PullNever,
		},
	}

	actualPod, err := kubeClientset.CoreV1().Pods(testutil.Namespace()).Create(testPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualPod.Spec.Containers[0].ImagePullPolicy != corev1.PullNever {
		t.Errorf("expected %v, got %v", corev1.PullNever, actualPod.Spec.Containers[0].ImagePullPolicy)
	}
}
