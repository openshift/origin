package dr

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/upgrades"
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

const (
	machineAnnotationName = "machine.openshift.io/machine"
)

var disruptionTests []upgrades.Test = []upgrades.Test{
	&upgrades.ServiceUpgradeTest{},
	&upgrades.SecretUpgradeTest{},
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

	g.It("[Feature:EtcdRecovery] Cluster should restore itself after quorum loss", func() {
		e2eskipper.Skipf("Test is disabled pending a fix https://github.com/openshift/origin/pull/25774")

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

				framework.Logf("Perform etcd backup on remaining machine %s (machine %s)", survivingNodeName, survivingMachineName)
				// Need to supply --force to the backup script to avoid failing on the api check for progressing operators.
				execOnNodeOrFail(survivingNode, "sudo -i /bin/bash -cx 'rm -rf /home/core/backup; /usr/local/bin/cluster-backup.sh --force ~core/backup'")

				framework.Logf("Restore etcd and control-plane on remaining node %s (machine %s)", survivingNodeName, survivingMachineName)
				execOnNodeOrFail(survivingNode, "sudo -i /bin/bash -cx '/usr/local/bin/cluster-restore.sh /home/core/backup'")

				framework.Logf("Wait for API server to come up")
				time.Sleep(30 * time.Second)
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					nodes, err := pollClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{Limit: 2})
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

				framework.Logf("Force new revision of etcd-pod")
				_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-etcd"}}`), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Force new revision of kube-apiserver")
				_, err = oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Patch(context.Background(), "cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-kube-apiserver"}}`), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Recovery 13
				waitForReadyEtcdPods(oc.AdminKubeClient(), expectedNumberOfMasters)

				scaleEtcdQuorum(pollClient, expectedNumberOfMasters)

				// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=1707006#
				// SDN won't switch to Degraded mode when service is down after disaster recovery
				// restartSDNPods(oc)
				waitForMastersToUpdate(oc, mcps)
				waitForOperatorsToSettle()
			})
	},
	)
})

func waitForPodsTolerateClientTimeout(c corev1client.PodInterface, label labels.Selector, predicate func(corev1.Pod) bool, count int, timeout time.Duration) {
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {
		p, e := exutil.GetPodNamesByFilter(c, label, predicate)
		if e != nil {
			framework.Logf("Saw an error waiting for etcd pods to become available: %v", e)
			// TODO tolerate transient etcd timeout only and fail other errors
			return false, nil
		}
		if len(p) != count {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func scaleEtcdQuorum(client kubernetes.Interface, replicas int) error {
	etcdQGScale, err := client.AppsV1().Deployments("openshift-etcd").GetScale(context.Background(), "etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if etcdQGScale.Spec.Replicas == int32(replicas) {
		return nil
	}
	framework.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale.Spec.Replicas = int32(replicas)
	_, err = client.AppsV1().Deployments("openshift-etcd").UpdateScale(context.Background(), "etcd-quorum-guard", etcdQGScale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	etcdQGScale, err = client.AppsV1().Deployments("openshift-etcd").GetScale(context.Background(), "etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	o.Expect(etcdQGScale.Spec.Replicas).To(o.Equal(int32(replicas)))
	return nil
}

func getPullSecret(oc *exutil.CLI) string {
	framework.Logf("Saving image pull secret")
	//TODO: copy of test/extended/operators/images.go, move this to a common func
	imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
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
	masterNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	annotations := masterNode.GetAnnotations()
	o.Expect(annotations).To(o.HaveKey(machineAnnotationName))
	return strings.Split(annotations[machineAnnotationName], "/")[1]
}
