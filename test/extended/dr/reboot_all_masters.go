package dr

import (
	"fmt"
	"math/rand"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:DisasterRecovery][Disruptive]", func() {
	f := framework.NewDefaultFramework("machine-reboots")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("machine-reboots")

	g.It("[Feature:AllMasterNodesReboot] Cluster should survive reboot of all master machines at the same time", func() {
		framework.SkipUnlessProviderIs("aws")

		disruption.Run("Reboot all Masters at once", "reboot_masters",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				&controlplane.AvailableTest{},
				&frontends.AvailableTest{},
			},
			func() {
				framework.Logf("Verify SSH is available before restart")
				masters, workers := clusterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 3))
				o.Expect(len(workers)).To(o.BeNumerically(">=", 2))
				for _, m := range masters {
					expectSSH("true", m)
				}

				framework.Logf("Verify etcd endpoints are healthy")
				checkHealthyEtcd(oc, masters[rand.Intn(len(masters))].Name)
				// forcing reboot for each master
				for _, m := range masters {
					framework.Logf("Forcing reboot of node %s", m.Name)
					go expectSSH("sudo -i reboot -f", m)
				}

				for _, node := range masters {
					err := wait.PollImmediate(15*time.Second, 5*time.Minute, func() (bool, error) {
						_, err := ssh("true", node)
						if err != nil {
							framework.Logf("error sshing into node %s: %#v", node.Name, err)
							return false, nil
						}
						framework.Logf("ssh successful into node %s: after reboot", node.Name)
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				framework.Logf("Wait for all masters to go ready")
				err := wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
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

				framework.Logf("Wait for all etcd pods to go ready")
				err = wait.PollImmediate(15*time.Second, 5*time.Minute, func() (bool, error) {
					defer func() {
						if r := recover(); r != nil {
							fmt.Println("Recovered from panic", r)
						}
					}()
					pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(metav1.ListOptions{LabelSelector: "app=etcd"})
					if err != nil {
						return false, err
					}
					readyPods := []corev1.Pod{}
					for _, pod := range pods.Items {
						if pod.Status.Phase == corev1.PodRunning {
							for _, container := range pod.Status.ContainerStatuses {
								if container.Name == "etcd" && container.Ready {
									readyPods = append(readyPods, pod)
								}
							}
						}
					}
					if len(readyPods) != len(masters) {
						framework.Logf("%d etcd pods still unready", len(masters)-len(readyPods))
						return false, nil
					}
					framework.Logf("all etcd have started after reboot and are healthy")
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				checkHealthyEtcd(oc, masters[rand.Intn(len(masters))].Name)
			})
	})
})

func checkHealthyEtcd(oc *exutil.CLI, healthyNode string) {
	output, err := oc.AsAdmin().Run("exec").Args("--namespace", "openshift-etcd", "etcd-"+healthyNode, "-c", "etcdctl", "--", "etcdctl", "endpoint", "health", "--cluster").Output()
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(output).ToNot(o.ContainSubstring("unhealthy"))
}
