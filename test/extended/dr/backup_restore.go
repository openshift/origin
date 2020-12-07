package dr

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Disruptive]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("backup-restore")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("backup-restore")

	// Validate the documented backup and restore procedure as closely as possible:
	//
	// backup: https://docs.openshift.com/container-platform/4.6/backup_and_restore/backing-up-etcd.html
	// restore: https://docs.openshift.com/container-platform/4.6/backup_and_restore/disaster_recovery/scenario-2-restoring-cluster-state.html
	//
	// Comments like 'Backup 2' and 'Restore '1a' indicate where a test step
	// corresponds to a step in the documentation.
	//
	// Backing up and recovering on the same node is tested by quorum_restore.go.
	g.It("[Feature:EtcdRecovery] Cluster should recover from a backup taken on one node and recovered on another", func() {
		masters := masterNodes(oc)
		// Need one node to backup from and another to restore to
		o.Expect(len(masters)).To(o.BeNumerically(">=", 2))

		// Pick one node to backup on
		backupNode := masters[0]
		framework.Logf("Selecting node %q as the backup host", backupNode.Name)

		// Recovery 1
		// Pick a different node to recover on
		recoveryNode := masters[1]
		framework.Logf("Selecting node %q as the recovery host", recoveryNode.Name)

		// Recovery 2
		g.By("Verifying that all masters are reachable via ssh")
		for _, master := range masters {
			checkSSH(master)
		}

		disruptionFunc := func() {
			// Backup 4
			//
			// The backup has to be taken after the upgrade tests have done
			// their pre-disruption setup to ensure that the api state that is
			// restored includes those changes.
			g.By(fmt.Sprintf("Running the backup script on node %q", backupNode.Name))
			sudoExecOnNodeOrFail(backupNode, "rm -rf /home/core/backup && /usr/local/bin/cluster-backup.sh /home/core/backup && chown -R core /home/core/backup")

			// Recovery 3
			// Copy the backup data from the backup node to the test host and
			// from there to the recovery node.
			//
			// Another solution could be enabling the recovery node to connect
			// directly to the backup node. It seemed simpler to use the test host
			// as an intermediary rather than enabling agent forwarding or copying
			// the private ssh key to the recovery node.

			g.By("Creating a local temporary directory")
			tempDir, err := ioutil.TempDir("", "e2e-backup-restore")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Define the ssh configuration necessary to invoke scp, which does
			// not appear to be supported by the golang ssh client.
			commonOpts := "-o StrictHostKeyChecking=no -o LogLevel=error -o ServerAliveInterval=30 -o ConnectionAttempts=100 -o ConnectTimeout=30"
			authOpt := fmt.Sprintf("-i %s", os.Getenv("KUBE_SSH_KEY_PATH"))
			bastionHost := os.Getenv("KUBE_SSH_BASTION")
			proxyOpt := ""
			if len(bastionHost) > 0 {
				framework.Logf("Bastion host %s will be used to proxy scp to cluster nodes", bastionHost)
				// The bastion host is expected to be of the form address:port
				hostParts := strings.Split(bastionHost, ":")
				o.Expect(len(hostParts)).To(o.Equal(2))
				address := hostParts[0]
				port := hostParts[1]
				// A proxy command is required for a bastion host
				proxyOpt = fmt.Sprintf("-o ProxyCommand='ssh -A -W %%h:%%p %s %s -p %s core@%s'", commonOpts, authOpt, port, address)
			}

			g.By(fmt.Sprintf("Copying the backup directory from backup node %q to the test host", backupNode.Name))
			backupNodeAddress := addressForNode(backupNode)
			o.Expect(backupNodeAddress).NotTo(o.BeEmpty())
			copyFromBackupNodeCmd := fmt.Sprintf(`scp -v %s %s %s -r core@%s:backup %s`, commonOpts, authOpt, proxyOpt, backupNodeAddress, tempDir)
			runCommandAndRetry(copyFromBackupNodeCmd)

			g.By(fmt.Sprintf("Cleaning the backup path on recovery node %q", recoveryNode.Name))
			sudoExecOnNodeOrFail(recoveryNode, "rm -rf /home/core/backup")

			g.By(fmt.Sprintf("Copying the backup directory from the test host to recovery node %q", recoveryNode.Name))
			recoveryNodeAddress := addressForNode(recoveryNode)
			o.Expect(recoveryNodeAddress).NotTo(o.BeEmpty())
			copyToRecoveryNodeCmd := fmt.Sprintf(`scp %s %s %s -r %s/backup core@%s:`, commonOpts, authOpt, proxyOpt, tempDir, recoveryNodeAddress)
			runCommandAndRetry(copyToRecoveryNodeCmd)

			// Stop etcd static pods on non-recovery masters.
			for _, master := range masters {
				// The restore script will stop static pods on the recovery node
				if master.Name == recoveryNode.Name {
					continue
				}
				// Recovery 4b
				g.By(fmt.Sprintf("Stopping etcd static pod on node %q", master.Name))
				manifest := "/etc/kubernetes/manifests/etcd-pod.yaml"
				// Move only if present to ensure idempotent behavior during debugging.
				sudoExecOnNodeOrFail(master, fmt.Sprintf("test -f %s && mv -f %s /tmp || true", manifest, manifest))

				// Recovery 4c
				g.By(fmt.Sprintf("Waiting for etcd to exit on node %q", master.Name))
				// Look for 'etcd ' (with trailing space) to be missing to
				// differentiate from pods like etcd-operator.
				sudoExecOnNodeOrFail(master, "crictl ps | grep 'etcd ' | wc -l | grep -q 0")

				// Recovery 4f
				g.By(fmt.Sprintf("Moving etcd data directory on node %q", master.Name))
				// Move only if present to ensure idempotent behavior during debugging.
				sudoExecOnNodeOrFail(master, "test -d /var/lib/etcd && (rm -rf /tmp/etcd && mv /var/lib/etcd/ /tmp) || true")
			}

			// Recovery 4d
			// Trigger stop of kube-apiserver static pods on non-recovery
			// masters, without waiting, to minimize the test time required for
			// graceful termination to complete.
			for _, master := range masters {
				// The restore script will stop static pods on the recovery node
				if master.Name == recoveryNode.Name {
					continue
				}
				g.By(fmt.Sprintf("Stopping kube-apiserver static pod on node %q", master.Name))
				manifest := "/etc/kubernetes/manifests/kube-apiserver-pod.yaml"
				// Move only if present to ensure idempotent behavior during debugging.
				sudoExecOnNodeOrFail(master, fmt.Sprintf("test -f %s && mv -f %s /tmp || true", manifest, manifest))
			}

			// Recovery 4e
			// Wait for kube-apiserver pods to exit
			for _, master := range masters {
				// The restore script will stop static pods on the recovery node
				if master.Name == recoveryNode.Name {
					continue
				}
				g.By(fmt.Sprintf("Waiting for kube-apiserver to exit on node %q", master.Name))
				// Look for 'kube-apiserver ' (with trailing space) to be missing
				// to differentiate from pods like kube-apiserver-operator.
				sudoExecOnNodeOrFail(master, "crictl ps | grep -q 'kube-apiserver ' | wc -l | grep -q 0")
			}

			// Recovery 7
			restoreFromBackup(recoveryNode)

			// Recovery 8
			for _, master := range masters {
				restartKubelet(master)
			}

			// Recovery 9a, 9b
			waitForAPIServer(oc.AdminKubeClient(), recoveryNode)

			// Recovery 10,11,12
			forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())

			// Recovery 13
			waitForReadyEtcdPods(oc.AdminKubeClient(), len(masters))

			waitForOperatorsToSettle()
		}

		disruption.Run(f, "Backup from one node and recover on another", "restore_different_node",
			disruption.TestData{},
			disruptionTests,
			disruptionFunc,
		)
	})
})

// addressForNode looks for an ssh-accessible ip address for a node in case the
// node name doesn't resolve in the test environment. An empty string will be
// returned if an address could not be determined.
func addressForNode(node *corev1.Node) string {
	for _, a := range node.Status.Addresses {
		if a.Type == corev1.NodeExternalIP && a.Address != "" {
			return a.Address
		}
	}
	// No external IPs were found, let's try to use internal as plan B
	for _, a := range node.Status.Addresses {
		if a.Type == corev1.NodeInternalIP && a.Address != "" {
			return a.Address
		}
	}
	return ""
}

// What follows are helper functions corresponding to steps in the recovery
// procedure. They are defined in a granular fashion to allow reuse by the
// quorum restore test. The quorum restore test needs to interleave the
// standard commands with commands related to master recreation.

// Recovery 7
func restoreFromBackup(node *corev1.Node) {
	g.By(fmt.Sprintf("Running restore script on recovery node %q", node.Name))
	sudoExecOnNodeOrFail(node, "/usr/local/bin/cluster-restore.sh /home/core/backup")
}

// Recovery 8
func restartKubelet(node *corev1.Node) {
	g.By(fmt.Sprintf("Restarting the kubelet service on node %q", node.Name))
	sudoExecOnNodeOrFail(node, "systemctl restart kubelet.service")
}

// Recovery 9a
func waitForEtcdContainer(node *corev1.Node) {
	g.By(fmt.Sprintf("Verifying that the etcd container is running on recovery node %q", node.Name))
	// Look for 'etcd ' (with trailing space) to differentiate from pods
	// like etcd-operator.
	sudoExecOnNodeOrFail(node, "crictl ps | grep -q 'etcd '")
}

// Recovery 9b
func waitForEtcdPod(node *corev1.Node) {
	// The success of this check also ensures that the kube apiserver on
	// the recovery node is accepting connections.
	g.By(fmt.Sprintf("Verifying that the etcd pod is running on recovery node %q", node.Name))
	// Look for a single running etcd pod
	runningEtcdPodCmd := "oc get pods -n openshift-etcd -l k8s-app=etcd --no-headers=true | grep Running | wc -l | grep -q 1"
	// The kubeconfig on the node is only readable by root and usage requires sudo.
	nodeKubeConfig := "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig"
	sudoExecOnNodeOrFail(node, fmt.Sprintf("KUBECONFIG=%s %s", nodeKubeConfig, runningEtcdPodCmd))
}

func waitForAPIServerAvailability(client kubernetes.Interface) {
	g.By("Waiting for API server to become available")
	err := wait.PollImmediate(10*time.Second, 30*time.Minute, func() (done bool, err error) {
		_, err = client.CoreV1().Namespaces().Get(context.Background(), "default", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Observed an error waiting for apiserver availability outside the cluster: %v", err)
		}
		return err == nil, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// waitForAPIServer waits for the etcd container and pod running on the
// recovery node and then waits for the apiserver to be accessible outside
// the cluster.
func waitForAPIServer(client kubernetes.Interface, node *corev1.Node) {
	// Recovery 9a
	waitForEtcdContainer(node)

	// Recovery 9b
	waitForEtcdPod(node)

	// Even with the apiserver available on the recovery node, it may
	// take additional time for the api to become available externally
	// to the cluster.
	waitForAPIServerAvailability(client)
}

// Recovery 13
func waitForReadyEtcdPods(client kubernetes.Interface, masterCount int) {
	g.By("Waiting for all etcd pods to become ready")
	waitForPodsTolerateClientTimeout(
		client.CoreV1().Pods("openshift-etcd"),
		exutil.ParseLabelsOrDie("k8s-app=etcd"),
		exutil.CheckPodIsReady,
		masterCount,
		40*time.Minute,
	)
}
