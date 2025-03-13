package machine_config

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	mcClient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"
)

// This test is [Serial] because it modifies the cluster/machineconfigurations.operator.openshift.io object in each test.
var _ = g.Describe("[sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNode][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigurationBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigurations")
		pinnedImageSetFixture          = filepath.Join(MCOMachineConfigurationBaseDir, "pis.yaml")
		customMCP                      = filepath.Join(MCOMachineConfigurationBaseDir, "customMCP.yaml")
		customMCPpinnedImageSetFixture = filepath.Join(MCOMachineConfigurationBaseDir, "customMCPpis.yaml")
		oc                             = exutil.NewCLIWithoutNamespace("machine-config")
	)
	// Ensure each test pins a separate image, since we are not deleting them after each
	g.It("All Nodes in the Custom Pool should have the PinnedImages specified in PIS [apigroup:machineconfiguration.openshift.io]", func() {
		err := oc.Run("apply").Args("-f", customMCP).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		nodes, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Get all nodes")
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[0].Name, "node-role.kubernetes.io/custom="+"").Execute()

		SimplePISTest(oc, customMCPpinnedImageSetFixture)
	})

	g.It("All Nodes in the Pool should have the PinnedImages specified in PIS [apigroup:machineconfiguration.openshift.io]", func() {
		SimplePISTest(oc, pinnedImageSetFixture)
	})
})

func SimplePISTest(oc *exutil.CLI, fixture string) {
	clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	pis, err := getPISFromFixture(fixture)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	waitForPISStatusSuccess(ctx, oc, clientSet, pis.Name)

}

func waitForPISStatusSuccess(ctx context.Context, oc *exutil.CLI, clientSet *mcClient.Clientset, pisName string) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		// Wait for PIS object to get created
		appliedPIS, err := clientSet.MachineconfigurationV1alpha1().PinnedImageSets().Get(context.TODO(), pisName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("PIS Object not created yet: %w", err)
		}

		pool, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(ctx, appliedPIS.Labels["machineconfiguration.openshift.io/role"], metav1.GetOptions{})
		if err != nil {
			return true, fmt.Errorf("failed to get MCP mentioned in PIS: %w", err)
		}

		nodes, err := getNodesForPool(ctx, oc, pool)
		doneNodes := 0
		for _, node := range nodes.Items {
			mcn, err := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				return true, fmt.Errorf("failed to get mcn: %w", err)
			}
			for _, cond := range mcn.Status.Conditions {
				if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
					return true, fmt.Errorf("PIS degraded for MCN %s with reason: %s and message: %s", mcn.Name, cond.Reason, cond.Message)
				}

				if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
					return false, nil
				}
			}
			for _, img := range appliedPIS.Spec.PinnedImages {
				crictlStatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-machine-config-operator", "crictl", "inspecti", img.Name)
				if err != nil {
					return false, fmt.Errorf("failed to execute `crictl inspecti %s` on node %s: %w", img.Name, node.Name, err)
				}
				if !strings.Contains(crictlStatus, "imageSpec") {
					return false, fmt.Errorf("Image %s not present on node %s: %w", img.Name, node.Name, err)
				}
			}
			doneNodes += 1
		}
		if doneNodes == len(nodes.Items) {
			return true, nil
		}

		return false, nil
	})
}

func getPISFromFixture(path string) (*mcfgv1alpha1.PinnedImageSet, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ob := new(mcfgv1alpha1.PinnedImageSet)
	err = yaml.Unmarshal(data, ob)
	if err != nil {
		return nil, err
	}

	return ob, err
}
