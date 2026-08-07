package imagepolicy

import (
	"context"
	"time"

	o "github.com/onsi/gomega"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	machineconfighelper "github.com/openshift/origin/test/extended/machine_config"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetMCPCurrentSpecConfigName returns the current Spec.Configuration.Name for the given MachineConfigPool.
func GetMCPCurrentSpecConfigName(oc *exutil.CLI, pool string) string {
	clientSet, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return mcp.Spec.Configuration.Name
}

// WaitForMCPConfigSpecChangeAndUpdated waits until Spec.Configuration.Name changes from the provided initial value
// and the MCP reports Updated=true.
func WaitForMCPConfigSpecChangeAndUpdated(oc *exutil.CLI, pool string, initialSpecName string) {
	e2e.Logf("Waiting for pool %s to complete", pool)
	clientSet, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
		if err != nil {
			return false
		}
		if mcp.Status.Configuration.Name == initialSpecName {
			return false
		}
		return machineconfighelper.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated)
	}, 20*time.Minute, 10*time.Second).Should(o.BeTrue())
}
