package integration

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/client"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func setupAuditTest(t *testing.T) (kclientset.Interface, *client.Client, func()) {
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
	return kubeClient, openshiftClient, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func TestBasicFunctionalityWithAudit(t *testing.T) {
	kubeClient, _, fn := setupAuditTest(t)
	defer fn()

	if _, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Watch(metav1.ListOptions{}); err != nil {
		t.Errorf("Unexpected error watching pods: %v", err)
	}

	// TODO: test oc debug, exec, rsh, port-forward
}
