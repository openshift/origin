package storage

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-storage][Feature:StorageBinaries][Jira:Storage]", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("storage-binaries-test", admissionapi.LevelPrivileged)
	)

	g.It("should have required storage binaries on node", func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Storage binaries check is not supported on MicroShift")
		}

		g.By("getting healthy and schedulable cluster nodes")
		allNodes, err := exutil.GetAllClusterNodes(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster nodes")
		o.Expect(len(allNodes)).To(o.BeNumerically(">", 0), "no nodes found in cluster")

		// Filter to get healthy and schedulable nodes only.
		// For efficiency in large clusters, we limit to the first 6 healthy nodes.
		// This provides sufficient randomness while avoiding unnecessary iteration.
		const maxHealthyNodes = 6
		healthyNodes := []string{}
		for _, node := range allNodes {
			// Check if node is schedulable (not cordoned) and in Ready state
			if !node.Spec.Unschedulable && exutil.IsNodeReady(node) {
				healthyNodes = append(healthyNodes, node.Name)
				// Stop after finding enough healthy nodes for random selection
				if len(healthyNodes) >= maxHealthyNodes {
					break
				}
			}
		}
		o.Expect(len(healthyNodes)).To(o.BeNumerically(">", 0), "no healthy and schedulable nodes found in cluster")

		// Randomly select a node from the healthy pool to cover master/worker
		rand.Seed(time.Now().UnixNano())
		randomIndex := rand.Intn(len(healthyNodes))
		nodeName := healthyNodes[randomIndex]
		e2e.Logf("Testing storage binaries on randomly selected healthy node: %s (from %d healthy nodes)", nodeName, len(healthyNodes))

		// List of required storage binaries to check
		requiredBinaries := []string{
			"/sbin/xfs_quota",
			"losetup",
			"stat",
			"find",
			"nice",
			"du",
			"multipath",
			"iscsiadm",
			"lsattr",
			"test",
			"udevadm",
			"resize2fs",
			"xfs_growfs",
			"umount",
			"mkfs.ext3",
			"mkfs.ext4",
			"mkfs.xfs",
			"fsck",
			"blkid",
			"systemd-run",
			"mount.nfs",
		}

		g.By("checking for required storage binaries")

		// Build a single script to check all binaries at once
		var checkScript strings.Builder
		for _, binary := range requiredBinaries {
			var checkCmd string
			if strings.HasPrefix(binary, "/") {
				checkCmd = fmt.Sprintf("if [ -f %s ]; then echo '%s:found'; else echo '%s:missing'; fi", binary, binary, binary)
			} else {
				checkCmd = fmt.Sprintf("if which %s >/dev/null 2>&1; then echo '%s:found'; else echo '%s:missing'; fi", binary, binary, binary)
			}
			checkScript.WriteString(checkCmd + "\n")
		}

		// Execute all checks in a single debug session
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "default", "bash", "-c", checkScript.String())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to check binaries on node %s", nodeName))

		// Parse the output
		var missingBinaries []string
		var foundBinaries []string
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			fields := strings.Split(line, ":")
			if len(fields) != 2 {
				continue
			}
			binary := fields[0]
			status := fields[1]

			if status == "found" {
				foundBinaries = append(foundBinaries, binary)
				e2e.Logf("Found binary: %s", binary)
			} else {
				missingBinaries = append(missingBinaries, binary)
				e2e.Logf("Missing binary: %s", binary)
			}
		}

		g.By("verifying all required binaries are present")
		if len(missingBinaries) > 0 {
			e2e.Failf("Missing %d required storage binaries on node %s: %v",
				len(missingBinaries), nodeName, missingBinaries)
		}

		e2e.Logf("Successfully verified %d storage binaries on node %s", len(foundBinaries), nodeName)
	})
})
