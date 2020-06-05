package dr

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

const (
	machineAnnotationName = "machine.openshift.io/machine"
)

var _ = g.Describe("[Feature:DisasterRecovery][Disruptive]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("[dr-quorum-restore] Cluster should restore itself after quorum loss", func() {
		disruption.Run("Quorum Loss and Restore", "quorum_restore",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				&upgrades.SecretUpgradeTest{},
				&apps.ReplicaSetUpgradeTest{},
				&apps.StatefulSetUpgradeTest{},
				&apps.DeploymentUpgradeTest{},
				&apps.DaemonSetUpgradeTest{},
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
				mcps := dynamicClient.Resource(schema.GroupVersionResource{
					Group:    "machineconfiguration.openshift.io",
					Version:  "v1",
					Resource: "machineconfigpools",
				})
				coc := dynamicClient.Resource(schema.GroupVersionResource{
					Group:    "config.openshift.io",
					Version:  "v1",
					Resource: "clusteroperators",
				})

				framework.Logf("Verify SSH is available before restart")
				masters := masterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 1))
				survivingNode := masters[rand.Intn(len(masters))]
				survivingNodeName := survivingNode.Name
				expectSSH("true", survivingNode)

				err = scaleEtcdQuorum(oc.AdminKubeClient(), 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedNumberOfMasters := len(masters)
				survivingMachineName := getMachineNameByNodeName(oc, survivingNodeName)
				survivingMachine, err := ms.Get(survivingMachineName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Destroy %d masters", len(masters)-1)
				var masterMachines []string
				for _, node := range masters {
					masterMachine := getMachineNameByNodeName(oc, node.Name)
					masterMachines = append(masterMachines, masterMachine)

					if node.Name == survivingNodeName {
						continue
					}

					framework.Logf("Destroying %s", masterMachine)
					err = ms.Delete(masterMachine, &metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(masters) != 1 {
					framework.Logf("Wait for control plane to become unresponsive (may take several minutes)")
					failures := 0
					err = wait.Poll(5*time.Second, 30*time.Minute, func() (done bool, err error) {
						_, err = pollClient.CoreV1().Nodes().List(metav1.ListOptions{})
						if err != nil {
							failures++
						} else {
							failures = 0
						}

						// there is a small chance the cluster restores the default replica size during
						// this loop process, so keep forcing quorum guard to be zero, without failing on
						// errors
						scaleEtcdQuorum(pollClient, 0)

						// wait to see the control plane go down for good to avoid a transient failure
						return failures > 4, nil
					})
				}

				framework.Logf("Perform etcd backup on remaining machine %s (machine %s)", survivingNodeName, survivingMachineName)
				expectSSH("sudo -i /bin/bash -cx 'rm -rf /home/core/backup; /usr/local/bin/cluster-backup.sh ~core/backup'", survivingNode)
				framework.Logf("Restore etcd and control-plane on remaining node %s (machine %s)", survivingNodeName, survivingMachineName)
				expectSSH("sudo -i /bin/bash -cx '/usr/local/bin/cluster-restore.sh /home/core/backup'", survivingNode)

				framework.Logf("Wait for API server to come up")
				time.Sleep(30 * time.Second)
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					nodes, err := pollClient.CoreV1().Nodes().List(metav1.ListOptions{Limit: 2})
					if err != nil || nodes.Items == nil {
						framework.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

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
							_, err := ms.Create(newMaster, metav1.CreateOptions{})
							if errors.IsAlreadyExists(err) {
								framework.Logf("Waiting for old machine object %s to be deleted so we can create a new one", master)
								return false, nil
							}
							if err != nil {
								return false, err
							}
							return true, nil
						})
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					framework.Logf("Waiting for machines to be created")
					err = wait.Poll(30*time.Second, 10*time.Minute, func() (done bool, err error) {
						mastersList, err := ms.List(metav1.ListOptions{
							LabelSelector: "machine.openshift.io/cluster-api-machine-role=master",
						})
						if err != nil {
							return false, err
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
						nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
						if err != nil {
							// scale up to 2nd etcd will make this error inevitable
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

				framework.Logf("Force new revision of etcd-pod")
				_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch("cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-etcd"}}`))
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Force new revision of kube-apiserver")
				_, err = oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Patch("cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-kube-apiserver"}}`))
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for etcd pods to become available")
				_, err = waitForPodsTolerateClientTimeout(
					oc.AdminKubeClient().CoreV1().Pods("openshift-etcd"),
					exutil.ParseLabelsOrDie("k8s-app=etcd"),
					exutil.CheckPodIsReady,
					expectedNumberOfMasters,
					40*time.Minute,
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				scaleEtcdQuorum(pollClient, expectedNumberOfMasters)

				// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=1707006#
				// SDN won't switch to Degraded mode when service is down after disaster recovery
				// restartSDNPods(oc)
				waitForMastersToUpdate(oc, mcps)
				waitForOperatorsToSettle(coc)
			})
	},
	)
})

func waitForPodsTolerateClientTimeout(c corev1client.PodInterface, label labels.Selector, predicate func(corev1.Pod) bool, count int, timeout time.Duration) ([]string, error) {
	var podNames []string
	err := wait.Poll(1*time.Second, timeout, func() (bool, error) {
		p, e := exutil.GetPodNamesByFilter(c, label, predicate)
		if e != nil {
			// TODO tolerate transient etcd timeout only and fail other errors
			return false, nil
		}
		if len(p) != count {
			return false, nil
		}
		podNames = p
		return true, nil
	})
	return podNames, err
}

func scaleEtcdQuorum(client kubernetes.Interface, replicas int) error {
	etcdQGScale, err := client.AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if etcdQGScale.Spec.Replicas == int32(replicas) {
		return nil
	}
	framework.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale.Spec.Replicas = int32(replicas)
	_, err = client.AppsV1().Deployments("openshift-machine-config-operator").UpdateScale("etcd-quorum-guard", etcdQGScale)
	if err != nil {
		return err
	}

	etcdQGScale, err = client.AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	o.Expect(etcdQGScale.Spec.Replicas).To(o.Equal(int32(replicas)))
	return nil
}

func getPullSecret(oc *exutil.CLI) string {
	framework.Logf("Saving image pull secret")
	//TODO: copy of test/extended/operators/images.go, move this to a common func
	imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get("pull-secret", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		framework.Failf("unable to get pull secret for cluster: %v", err)
	}

	// cache file to local temp location
	imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
	if err != nil {
		framework.Failf("unable to create a temporary file: %v", err)
	}

	// write the content
	imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
	if _, err := imagePullFile.Write(imagePullSecretBytes); err != nil {
		framework.Failf("unable to write pull secret to temp file: %v", err)
	}
	if err := imagePullFile.Close(); err != nil {
		framework.Failf("unable to close file: %v", err)
	}
	framework.Logf("Image pull secret: %s", imagePullFile.Name())
	return imagePullFile.Name()
}

func getImagePullSpecFromRelease(oc *exutil.CLI, imagePullSecretPath, imageName string) string {
	var image string
	err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		location, err := oc.Run("adm", "release", "info").Args("--image-for", imageName, "--registry-config", imagePullSecretPath).Output()
		if err != nil {
			framework.Logf("Unable to find release info, retrying: %v", err)
			return false, nil
		}
		image = location
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return image
}

func getMachineNameByNodeName(oc *exutil.CLI, name string) string {
	masterNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	annotations := masterNode.GetAnnotations()
	o.Expect(annotations).To(o.HaveKey(machineAnnotationName))
	return strings.Split(annotations[machineAnnotationName], "/")[1]
}
