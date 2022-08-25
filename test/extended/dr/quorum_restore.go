package dr

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/upgrades"
	"k8s.io/kubernetes/test/e2e/upgrades/apps"
	"k8s.io/kubernetes/test/e2e/upgrades/network"
	"k8s.io/kubernetes/test/e2e/upgrades/node"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

const (
	machineAnnotationName = "machine.openshift.io/machine"
)

var disruptionTests []upgrades.Test = []upgrades.Test{
	&network.ServiceUpgradeTest{},
	&node.SecretUpgradeTest{},
	&apps.ReplicaSetUpgradeTest{},
	&apps.StatefulSetUpgradeTest{},
	&apps.DeploymentUpgradeTest{},
	&apps.DaemonSetUpgradeTest{},
}

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Disruptive]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	// Validate backing up and restoring to the same node on a cluster
	// that has lost quorum after the backup was taken.
	g.It("[Feature:EtcdRecovery] Cluster should restore itself after quorum loss", func() {
		config, err := framework.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)
		ms := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machine.openshift.io",
			Version:  "v1beta1",
			Resource: "machines",
		}).Namespace("openshift-machine-api")
		mcps := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})

		// test for machines as a proxy for "can we recover a master"
		machines, err := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machine.openshift.io",
			Version:  "v1beta1",
			Resource: "machines",
		}).List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(machines.Items) == 0 {
			e2eskipper.Skipf("machine API is not enabled and automatic recovery test is not possible")
		}

		disruption.Run(f, "Quorum Loss and Restore", "quorum_restore",
			disruption.TestData{},
			disruptionTests,
			func() {

				framework.Logf("Verify SSH is available before restart")
				masters := masterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 1))
				survivingNode := masters[rand.Intn(len(masters))]
				survivingNodeName := survivingNode.Name
				checkSSH(survivingNode)

				err = scaleEtcdQuorum(oc.AdminKubeClient(), 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedNumberOfMasters := len(masters)
				survivingMachineName := getMachineNameByNodeName(oc, survivingNodeName)
				survivingMachine, err := ms.Get(context.Background(), survivingMachineName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// The backup script only supports taking a backup of a cluster
				// that still has quorum, so the backup must be performed before
				// quorum-destroying machine deletion.
				framework.Logf("Perform etcd backup on node %s (machine %s) while quorum still exists", survivingNodeName, survivingMachineName)
				execOnNodeOrFail(survivingNode, "sudo -i /bin/bash -cx 'rm -rf /home/core/backup && /usr/local/bin/cluster-backup.sh /home/core/backup'")

				framework.Logf("Destroy %d masters", len(masters)-1)
				var masterMachines []string
				for _, node := range masters {
					masterMachine := getMachineNameByNodeName(oc, node.Name)
					masterMachines = append(masterMachines, masterMachine)

					if node.Name == survivingNodeName {
						continue
					}

					framework.Logf("Destroying %s", masterMachine)
					err = ms.Delete(context.Background(), masterMachine, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// All API calls for the remainder of the test should be performed in
				// a polling loop to insure against transient failures. API calls can
				// only be assumed to succeed without polling against a healthy
				// cluster, and only at the successful exit of this function will
				// that once again be the case.

				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(masters) != 1 {
					framework.Logf("Wait for control plane to become unresponsive (may take several minutes)")
					failures := 0
					err = wait.Poll(5*time.Second, 30*time.Minute, func() (done bool, err error) {
						_, err = pollClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
						if err != nil {
							framework.Logf("Error seen checking for unresponsive control plane: %v", err)
							failures++
						} else {
							failures = 0
						}

						// there is a small chance the cluster restores the default replica size during
						// this loop process, so keep forcing quorum guard to be zero, without failing on
						// errors
						if err := scaleEtcdQuorum(pollClient, 0); err != nil {
							framework.Logf("Scaling etcd quorum failed: %v", err)
						}

						// wait to see the control plane go down for good to avoid a transient failure
						return failures > 4, nil
					})
				}

				// Recovery 7
				restoreFromBackup(survivingNode)

				// Recovery 8
				restartKubelet(survivingNode)

				// Recovery 9a, 9b
				waitForAPIServer(oc.AdminKubeClient(), survivingNode)

				// Restoring brings back machines and nodes deleted since the
				// backup was taken. Those machines and nodes need to be removed
				// before they can be created again.
				//
				// TODO(marun) Ensure the mechanics of node replacement around
				// disaster recovery are documented.
				for _, master := range masterMachines {
					if master == survivingMachineName {
						continue
					}
					err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
						framework.Logf("Initiating deletion of machine removed after the backup was taken: %s", master)
						err := ms.Delete(context.Background(), master, metav1.DeleteOptions{})
						if err != nil && !errors.IsNotFound(err) {
							framework.Logf("Error seen when attempting to remove restored machine %s: %v", master, err)
							return false, nil
						}
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				for _, node := range masters {
					if node.Name == survivingNodeName {
						continue
					}
					err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
						framework.Logf("Initiating deletion of node removed after the backup was taken: %s", node.Name)
						err := oc.AdminKubeClient().CoreV1().Nodes().Delete(context.Background(), node.Name, metav1.DeleteOptions{})
						if err != nil && !errors.IsNotFound(err) {
							framework.Logf("Error seen when attempting to remove restored node %s: %v", node.Name, err)
							return false, nil
						}
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				if expectedNumberOfMasters == 1 {
					framework.Logf("Cannot create new masters, you must manually create masters and update their DNS entries according to the docs")
				} else {
					framework.Logf("Create new masters")
					for _, master := range masterMachines {
						if master == survivingMachineName {
							continue
						}
						framework.Logf("Creating master %s", master)
						newMaster := survivingMachine.DeepCopy()
						// The providerID is relied upon by the machine controller to determine a machine
						// has been provisioned
						// https://github.com/openshift/cluster-api/blob/c4a461a19efb8a25b58c630bed0829512d244ba7/pkg/controller/machine/controller.go#L306-L308
						unstructured.SetNestedField(newMaster.Object, "", "spec", "providerID")
						newMaster.SetName(master)
						newMaster.SetResourceVersion("")
						newMaster.SetSelfLink("")
						newMaster.SetUID("")
						newMaster.SetCreationTimestamp(metav1.NewTime(time.Time{}))
						// retry until the machine gets created
						err := wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
							_, err := ms.Create(context.Background(), newMaster, metav1.CreateOptions{})
							if errors.IsAlreadyExists(err) {
								framework.Logf("Waiting for old machine object %s to be deleted so we can create a new one", master)
								return false, nil
							}
							if err != nil {
								framework.Logf("Error seen when re-creating machines: %v", err)
								return false, nil
							}
							return true, nil
						})
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					framework.Logf("Waiting for machines to be created")
					err = wait.Poll(30*time.Second, 20*time.Minute, func() (done bool, err error) {
						mastersList, err := ms.List(context.Background(), metav1.ListOptions{
							LabelSelector: "machine.openshift.io/cluster-api-machine-role=master",
						})
						if err != nil {
							framework.Logf("Failed to check that machines are created: %v", err)
							return false, nil
						}
						if mastersList.Items == nil {
							return false, nil
						}
						return len(mastersList.Items) == expectedNumberOfMasters, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					framework.Logf("Wait for masters to join as nodes and go ready")
					err = wait.Poll(30*time.Second, 50*time.Minute, func() (done bool, err error) {
						defer func() {
							if r := recover(); r != nil {
								fmt.Println("Recovered from panic", r)
							}
						}()
						nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
						if err != nil {
							// scale up to 2nd etcd will make this error inevitable
							framework.Logf("Error seen attempting to list master nodes: %v", err)
							return false, nil
						}
						ready := countReady(nodes.Items)
						if ready != expectedNumberOfMasters {
							framework.Logf("%d nodes still unready", expectedNumberOfMasters-ready)
							return false, nil
						}
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// Recovery 10,11,12
				forceOperandRedeployment(oc.AdminOperatorClient().OperatorV1())

				// Recovery 13
				waitForReadyEtcdPods(oc.AdminKubeClient(), expectedNumberOfMasters)

				// Scale quorum guard in a polling loop to ensure tolerance for disruption
				err = wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
					err := scaleEtcdQuorum(pollClient, int32(expectedNumberOfMasters))
					if err != nil {
						framework.Logf("Saw an error attempting to scale etcd quorum guard: %v", err)
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				waitForMastersToUpdate(oc, mcps)
				waitForOperatorsToSettle()
			})
	},
	)
})

func scaleEtcdQuorum(client kubernetes.Interface, replicas int32) error {
	etcdQGScale, err := client.AppsV1().Deployments("openshift-etcd").GetScale(context.Background(), "etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if etcdQGScale.Spec.Replicas == replicas {
		return nil
	}
	framework.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale.Spec.Replicas = replicas
	_, err = client.AppsV1().Deployments("openshift-etcd").UpdateScale(context.Background(), "etcd-quorum-guard", etcdQGScale, metav1.UpdateOptions{})
	return err
}

func getMachineNameByNodeName(oc *exutil.CLI, name string) string {
	masterNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	annotations := masterNode.GetAnnotations()
	o.Expect(annotations).To(o.HaveKey(machineAnnotationName))
	return strings.Split(annotations[machineAnnotationName], "/")[1]
}
