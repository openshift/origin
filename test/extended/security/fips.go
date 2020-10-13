package security

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	installConfigName = "cluster-config-v1"
	fipsFile          = "/proc/sys/crypto/fips_enabled"
)

// installConfig The subset of openshift-install's InstallConfig we parse for this test
type installConfig struct {
	FIPS bool `json:"fips,omitempty"`
}

func installConfigFromCluster(client clientcorev1.ConfigMapsGetter) (*installConfig, error) {
	cm, err := client.ConfigMaps("kube-system").Get(context.Background(), installConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data, ok := cm.Data["install-config"]
	if !ok {
		return nil, fmt.Errorf("No install-config found in kube-system/%s", installConfigName)
	}
	config := &installConfig{}
	if err := yaml.Unmarshal([]byte(data), config); err != nil {
		return nil, err
	}
	return config, nil
}

func validateFIPSOnNode(oc *exutil.CLI, fipsExpected bool, node *corev1.Node) error {
	command := []string{"cat", fipsFile}
	out, err := exutil.ExecCommandOnMachineConfigDaemon(oc.AdminKubeClient(), oc, node, command)
	if err != nil {
		return err
	}
	nodeFips, err := strconv.ParseBool(strings.TrimSuffix(string(out), "\n"))
	if err != nil {
		return fmt.Errorf("Error parsing %s on node %s: %v", fipsFile, node.Name, err)
	}
	if nodeFips != fipsExpected {
		return fmt.Errorf("Expected FIPS state %v, found %v", fipsExpected, nodeFips)
	}
	return nil
}

var _ = g.Describe("[sig-arch] [Conformance] FIPS", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("fips")

	g.It("TestFIPS", func() {
		clusterAdminKubeClientset := oc.AdminKubeClient()
		installConfig, err := installConfigFromCluster(clusterAdminKubeClientset.CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		// fetch one control plane and one worker, and validate FIPS state on it
		masterNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		masterNode := &masterNodes.Items[0]
		err = validateFIPSOnNode(oc, installConfig.FIPS, masterNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		workerNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(workerNodes.Items) > 0 {
			workerNode := &workerNodes.Items[0]
			err = validateFIPSOnNode(oc, installConfig.FIPS, workerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
