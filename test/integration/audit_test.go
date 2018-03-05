package integration

import (
	"io/ioutil"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/apis/audit"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func setupAudit(t *testing.T, auditConfig configapi.AuditConfig) (kclientset.Interface, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.AuditConfig = auditConfig
	kubeConfigFile, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	return kubeClient, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func TestBasicFunctionalityWithAudit(t *testing.T) {
	kubeClient, fn := setupAudit(t, configapi.AuditConfig{Enabled: true})
	defer fn()

	if _, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Watch(metav1.ListOptions{}); err != nil {
		t.Errorf("Unexpected error watching pods: %v", err)
	}

	// TODO: test oc debug, exec, rsh, port-forward
}

func TestAuditConfigEmbeded(t *testing.T) {
	auditConfig := configapi.AuditConfig{
		Enabled: true,
		PolicyConfiguration: &audit.Policy{
			Rules: []audit.PolicyRule{
				{Level: audit.LevelMetadata},
			},
		},
	}
	kubeClient, fn := setupAudit(t, auditConfig)
	defer fn()

	if _, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Watch(metav1.ListOptions{}); err != nil {
		t.Errorf("Unexpected error watching pods: %v", err)
	}
}

func TestAuditConfigV1Alpha1File(t *testing.T) {
	testAuditConfigFile(t, []byte(`
apiVersion: audit.k8s.io/v1alpha1
kind: Policy
rules:
- level: Metadata
`))
}

func TestAuditConfigV1Beta1File(t *testing.T) {
	testAuditConfigFile(t, []byte(`
apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
- level: Metadata
`))
}

func testAuditConfigFile(t *testing.T, policy []byte) {
	tmp, err := ioutil.TempFile("", "audit-policy")
	if err != nil {
		t.Fatalf("Cannot create a temporary file: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(policy); err != nil {
		t.Fatalf("Cannot write to a temporary file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Cannot close a temporary file: %v", err)
	}
	auditConfig := configapi.AuditConfig{
		Enabled:    true,
		PolicyFile: tmp.Name(),
	}
	kubeClient, fn := setupAudit(t, auditConfig)
	defer fn()

	if _, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Watch(metav1.ListOptions{}); err != nil {
		t.Errorf("Unexpected error watching pods: %v", err)
	}
}
