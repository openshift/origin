package dr

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
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

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Disruptive]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("[Feature:EtcdRecovery] Cluster should restore itself after quorum loss", func() {
		config, err := framework.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)
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

				framework.Logf("Verify SSH is available before restart")
				masters := masterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 1))

				err = scaleEtcdQuorum(oc.AdminKubeClient(), 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedNumberOfMasters := len(masters)
				o.Expect(err).NotTo(o.HaveOccurred())

				firstMaster := masters[0]
				framework.Logf("Make etcd backup on first master %v", firstMaster)
				expectSSH("sudo -i /bin/bash -cx 'rm -rf /home/core/backup; /usr/local/bin/cluster-backup.sh ~core/backup'", firstMaster)

				for i, node := range masters {
					if i == 0 {
						continue
					}
					framework.Logf("Stopping etcd and kube-apiserver pods and removing data-dir from %s", node.Name)

					expectSSH("sudo -i /bin/bash -cx 'mv /etc/kubernetes/manifests/etcd-pod.yaml /tmp'", node)
					time.Sleep(180 * time.Second)

					expectSSH("sudo -i /bin/bash -cx 'mv /etc/kubernetes/manifests/kube-apiserver-pod.yaml /tmp; rm -rf /var/lib/etcd'", node)
					time.Sleep(180 * time.Second)
				}

				framework.Logf("Restore etcd and control-plane on  %s", firstMaster)
				expectSSH("sudo -i /bin/bash -cx '/usr/local/bin/cluster-restore.sh /home/core/backup'", firstMaster)

				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

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

				framework.Logf("Wait for masters to join as nodes and go ready")
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					defer func() {
						if r := recover(); r != nil {
							fmt.Println("Recovered from panic", r)
						}
					}()
					nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
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

				framework.Logf("Force new revision of etcd-pod")
				_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(context.Background(), "cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-etcd"}}`), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Force new revision of kube-apiserver")
				_, err = oc.AdminOperatorClient().OperatorV1().KubeAPIServers().Patch(context.Background(), "cluster", types.MergePatchType, []byte(`{"spec": {"forceRedeploymentReason": "recover-kube-apiserver"}}`), metav1.PatchOptions{})
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
	etcdQGScale, err := client.AppsV1().Deployments("openshift-machine-config-operator").GetScale(context.Background(), "etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if etcdQGScale.Spec.Replicas == int32(replicas) {
		return nil
	}
	framework.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale.Spec.Replicas = int32(replicas)
	_, err = client.AppsV1().Deployments("openshift-machine-config-operator").UpdateScale(context.Background(), "etcd-quorum-guard", etcdQGScale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	etcdQGScale, err = client.AppsV1().Deployments("openshift-machine-config-operator").GetScale(context.Background(), "etcd-quorum-guard", metav1.GetOptions{})
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
