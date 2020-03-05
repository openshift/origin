package dr

import (
	"fmt"
	"math/rand"
	"time"

	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
)

var _ = g.Describe("[Feature:DisasterRecovery][Disruptive]", func() {
	f := framework.NewDefaultFramework("machine-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("machine-recovery")

	g.It("[dr-quorum-restore] Cluster should survive master and worker failure and recover with machine health checks", func() {
		framework.SkipUnlessProviderIs("aws")

		disruption.Run("Machine Shutdown and Restore", "machine_failure",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				&controlplane.AvailableTest{},
				&frontends.AvailableTest{},
			},
			func() {

				config, err := framework.LoadConfig()
				o.Expect(err).NotTo(o.HaveOccurred())
				dynamicClient := dynamic.NewForConfigOrDie(config)
				ms := dynamicClient.Resource(schema.GroupVersionResource{
					Group:    "machine.openshift.io",
					Version:  "v1beta1",
					Resource: "machines",
				}).Namespace("openshift-machine-api")

				framework.Logf("Verify SSH is available before restart")
				masters, workers := clusterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 3))
				o.Expect(len(workers)).To(o.BeNumerically(">=", 2))

				// TODO: create a machine health check for the masters with 5m downtime on ready
				// TODO: create a machine health check for the workers with 5m downtime on ready

				replacedMaster := masters[rand.Intn(len(masters))]
				expectSSH("true", replacedMaster)

				replacedWorker := workers[rand.Intn(len(workers))]
				expectSSH("true", replacedWorker)

				replacedMasterMachineName := getMachineNameByNodeName(oc, replacedMaster.Name)
				replacedMasterMachine, err := ms.Get(replacedMasterMachineName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := []*corev1.Node{replacedMaster, replacedWorker}

				// we use a hard shutdown to simulate a poweroff
				framework.Logf("Forcing shutdown of node %s", replacedMaster.Name)
				expectSSH("sudo -i systemctl poweroff --force --force now", replacedMaster)
				framework.Logf("Forcing shutdown of node %s", replacedWorker.Name)
				expectSSH("sudo -i systemctl poweroff --force --force now", replacedWorker)

				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for nodes to go away")
				time.Sleep(30 * time.Second)
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					nodes, err := pollClient.CoreV1().Nodes().List(metav1.ListOptions{})
					if err != nil || nodes.Items == nil {
						framework.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
						return false, nil
					}
					vanishedNodes := sets.NewString()
					for _, node := range targets {
						vanishedNodes.Insert(node.Name)
					}
					for _, node := range nodes.Items {
						vanishedNodes.Delete(node.Name)
					}
					if vanishedNodes.Len() > 0 {
						framework.Logf("Nodes still remain: %v", vanishedNodes.List())
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				// TODO: in the future this would be done by a controller automatically
				framework.Logf("Recreating master %s", replacedMaster.Name)
				newMaster := replacedMasterMachine.DeepCopy()
				// The providerID is relied upon by the machine controller to determine a machine
				// has been provisioned
				// https://github.com/openshift/cluster-api/blob/c4a461a19efb8a25b58c630bed0829512d244ba7/pkg/controller/machine/controller.go#L306-L308
				unstructured.SetNestedField(newMaster.Object, "", "spec", "providerID")
				newMaster.SetName(replacedMaster.Name)
				newMaster.SetResourceVersion("")
				newMaster.SetSelfLink("")
				newMaster.SetUID("")
				newMaster.SetCreationTimestamp(metav1.NewTime(time.Time{}))
				// retry until the machine gets created
				err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
					_, err := ms.Create(newMaster, metav1.CreateOptions{})
					if errors.IsAlreadyExists(err) {
						framework.Logf("Waiting for old machine object %s to be deleted so we can create a new one", replacedMaster.Name)
						return false, nil
					}
					if err != nil {
						return false, err
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for masters to join as nodes and go ready")
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					defer func() {
						if r := recover(); r != nil {
							fmt.Println("Recovered from panic", r)
						}
					}()
					nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
					if err != nil {
						return false, err
					}
					ready := countReady(nodes.Items)
					if ready != len(masters) {
						framework.Logf("%d master nodes still unready", len(masters)-ready)
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			})
	},
	)
})
