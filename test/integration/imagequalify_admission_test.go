package integration

import (
	"io/ioutil"
	"os"
	"testing"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestImageQualifyAdmission(t *testing.T) {
	pluginFile, err := ioutil.TempFile("", "admission.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pluginFile.Name())

	err = ioutil.WriteFile(pluginFile.Name(), []byte(`
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
rules:
- pattern: busybox
  domain: domain1.com
- pattern: mylib/*
  domain: mydomain.com
`), os.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
		"openshift.io/ImageQualify": {
			Location: pluginFile.Name(),
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

	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.Core().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	tests := map[string]string{
		"busybox":     "domain1.com/busybox",
		"mylib/foo":   "mydomain.com/mylib/foo",
		"yourlib/foo": "yourlib/foo",
	}

	for image, result := range tests {
		testPod := &kapi.Pod{}
		testPod.GenerateName = "test"
		testPod.Spec.Containers = []kapi.Container{{Name: "container", Image: image}}
		actualPod, err := kubeClientset.Core().Pods(testutil.Namespace()).Create(testPod)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if actualPod.Spec.Containers[0].Image != result {
			t.Errorf("expected %v, got %v", result, actualPod.Spec.Containers[0].Image)
			continue
		}
	}
}
