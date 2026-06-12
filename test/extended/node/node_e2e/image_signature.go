package node

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive][Serial] Image signature verification", func() {
	var (
		oc               = exutil.NewCLIWithoutNamespace("image-sig")
		nodeE2EBaseDir   = exutil.FixturePath("testdata", "node", "node_e2e")
		imgSignatureYAML = filepath.Join(nodeE2EBaseDir, "machineconfig-image-signature.yaml")
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Enable image signature verification for Red Hat Container Registries [OCP-59552]", ote.Informing(), func() {
		ctx := context.Background()

		g.By("Check if mcp worker exists in current cluster")
		machineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-o=jsonpath={.status.machineCount}").Output()
		if err != nil || machineCount == "0" {
			g.Skip("Skipping test: mcp worker does not exist in this cluster")
		}
		e2e.Logf("Worker MCP machine count: %s", machineCount)

		g.By("Apply a machine config to set image signature policy for worker nodes")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", imgSignatureYAML).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create MachineConfig")

		g.DeferCleanup(func(ctx context.Context) {
			g.By("Delete the MachineConfig")
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", imgSignatureYAML, "--ignore-not-found").Execute()

			g.By("Wait for MCP to finish rolling back")
			err := waitForMCPUpdate(ctx, oc, "worker", 30*time.Minute)
			if err != nil {
				e2e.Logf("Warning: MCP did not finish rolling back: %v", err)
			}
		}, ctx)

		g.By("Wait for MCP to finish updating")
		err = waitForMCPUpdate(ctx, oc, "worker", 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "MCP worker did not finish updating")

		g.By("Verify the signature configuration in /etc/containers/policy.json")
		err = checkImageSignature(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "image signature configuration verification failed")
	})
})

// waitForMCPUpdate waits for the MachineConfigPool to finish updating.
// It checks the Updated condition to become True (which means update is complete).
// Returns nil when the MCP is updated, or an error if it times out.
// This is a helper function and does not contain assertions.
func waitForMCPUpdate(ctx context.Context, oc *exutil.CLI, mcpName string, timeout time.Duration) error {
	g.GinkgoHelper()
	return wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		// Check the Updated condition instead of Updating
		// Updated=True means the MCP has finished updating
		updatedStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
		if err != nil {
			e2e.Logf("Error getting MCP Updated status: %v", err)
			return false, nil
		}

		// Check that machine counts match (all machines have the desired config)
		machineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={.status.machineCount}").Output()
		if err != nil {
			e2e.Logf("Error getting machine count: %v", err)
			return false, nil
		}
		updatedMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={.status.updatedMachineCount}").Output()
		if err != nil {
			e2e.Logf("Error getting updated machine count: %v", err)
			return false, nil
		}

		e2e.Logf("MCP %s: Updated=%s, machines=%s, updatedMachines=%s", mcpName, updatedStatus, machineCount, updatedMachineCount)

		if strings.Contains(updatedStatus, "True") && machineCount == updatedMachineCount {
			e2e.Logf("MCP %s updated successfully", mcpName)
			return true, nil
		}
		e2e.Logf("MCP %s is still updating", mcpName)
		return false, nil
	})
}

// checkImageSignature verifies that the image signature policy is correctly configured on worker nodes.
// It checks for required entries in /etc/containers/policy.json for Red Hat registries.
// This is a helper function and does not contain assertions.
func checkImageSignature(oc *exutil.CLI) error {
	g.GinkgoHelper()
	return wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		workerNode := nodeutils.GetFirstReadyWorkerNode(oc)
		policyJSON, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/policy.json")
		if err != nil {
			e2e.Logf("Error reading policy.json: %v", err)
			return false, nil
		}

		e2e.Logf("Checking policy.json content from node %s", workerNode)

		// Check for required entries in the policy.json
		requiredEntries := []string{
			"registry.access.redhat.com",
			"signedBy",
			"GPGKeys",
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
			"registry.redhat.io",
		}

		for _, entry := range requiredEntries {
			if !strings.Contains(policyJSON, entry) {
				e2e.Logf("Missing required entry in policy.json: %s", entry)
				return false, nil
			}
		}

		e2e.Logf("Image signature policy verified successfully")
		return true, nil
	})
}
