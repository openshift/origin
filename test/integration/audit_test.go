package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/client"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func setupAuditTest(t *testing.T) (*kclient.Client, *client.Client) {
	testutil.RequireEtcd(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.AuditConfig.Enabled = true
	kubeConfigFile, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	openshiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %v", err)
	}
	return kubeClient, openshiftClient

}

func TestBasicFunctionalityWithAudit(t *testing.T) {
	kubeClient, _ := setupAuditTest(t)
	defer testutil.DumpEtcdOnFailure(t)

	if _, err := kubeClient.Pods(kapi.NamespaceDefault).Watch(kapi.ListOptions{}); err != nil {
		t.Errorf("Unexpected error watching pods: %v", err)
	}

	// TOOD: test oc debug, exec, rsh, port-forward
}
