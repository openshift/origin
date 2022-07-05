package coreos

import (
	"math/rand"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-coreos] [Feature:machine-config-operator] [Conformance] [Slow] [Disruptive] kdump", func() {
	defer g.GinkgoRecover()
	var (
		infraMCPYaml         = exutil.FixturePath("testdata", "coreos", "infra.mcp.yaml")
		infraKdumpConfigYaml = exutil.FixturePath("testdata", "coreos", "99-infra-kdump-configuration.yaml")
	)

	oc := exutil.NewCLIWithPodSecurityLevel("kdump-enablement", admissionapi.LevelPrivileged)
	g.It("TestKdump", func() {
		_, workers := clusterNodes(oc)
		workerNode := workers[rand.Intn(len(workers))]

		config, err := framework.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)

		mcps := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})

		err = kdumpSetup(oc, workerNode, mcps, infraMCPYaml, infraKdumpConfigYaml)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = kernelCrash(oc, workerNode, mcps)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cleanup(oc, workerNode, mcps, infraMCPYaml, infraKdumpConfigYaml)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func kdumpSetup(oc *exutil.CLI, node *corev1.Node, mcps dynamic.NamespaceableResourceInterface, infraMCPYaml string, infraKdumpConfigYaml string) error {
	// Label a worker node to infra and create infra pool to roll out MC changes
	err := oc.AsAdmin().Run("label").Args("node/"+node.Name, "node-role.kubernetes.io/infra=").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().Run("create").Args("-f", infraMCPYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Apply MC to enable kdump.service and reserve memory for kdump
	err = oc.AsAdmin().Run("create").Args("-f", infraKdumpConfigYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForInfraToUpdate(oc, mcps)
	framework.Logf("infra.mcp and 99-infra-kdump-configuration config rendered")

	kargs, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c", "cat /proc/cmdline").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(kargs).Should(o.ContainSubstring("crashkernel=256M"))
	framework.Logf("The infra node has crashkernel kargs")

	activeKdump, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c", "systemctl is-active kdump").Output()
	o.Expect(activeKdump).ShouldNot(o.ContainSubstring("inactive"))
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("kdump.service is active")

	return nil
}

func kernelCrash(oc *exutil.CLI, node *corev1.Node, mcps dynamic.NamespaceableResourceInterface) error {
	// Triggering crash
	framework.Logf("Triggering kernel crash")
	err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c", string("echo 1 > /proc/sys/kernel/sysrq")).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c", string("(sleep 2; echo c > /proc/sysrq-trigger)")).Execute()
	waitForInfraToUpdate(oc, mcps)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Test the existence of vmcore file in /var/crash/
	kcore, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "chroot", "/host", "/bin/bash", "-c", "find /var/crash -type f -name vmcore").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(kcore).Should(o.ContainSubstring("/vmcore"))
	framework.Logf("kcore found in /var/crash")

	return nil
}

func cleanup(oc *exutil.CLI, node *corev1.Node, mcps dynamic.NamespaceableResourceInterface, infraMCPYaml string, infraKdumpConfigYaml string) error {
	// Remove the infra label from the node
	err := oc.AsAdmin().Run("label").Args("node/"+node.Name, "node-role.kubernetes.io/infra-").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForWorkerToUpdate(oc, mcps)

	// Delete the respective mc and mcp
	err = oc.AsAdmin().Run("delete").Args("mc", "99-infra-kdump-configuration").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().Run("delete").Args("mcp", "infra").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	return nil
}
