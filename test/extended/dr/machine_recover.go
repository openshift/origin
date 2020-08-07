package dr

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/transport"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

var _ = g.Describe("[sig-cluster-lifecycle][Feature:DisasterRecovery][Disruptive]", func() {
	f := framework.NewDefaultFramework("machine-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("machine-recovery")

	g.It("[Feature:NodeRecovery] Cluster should survive master and worker failure and recover with machine health checks", func() {

		framework.Logf("Verify SSH is available before restart")
		masters, workers := clusterNodes(oc)
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3))
		o.Expect(len(workers)).To(o.BeNumerically(">=", 2))

		replacedMaster := masters[rand.Intn(len(masters))]
		expectSSH("true", replacedMaster)

		replacedWorker := workers[rand.Intn(len(workers))]
		expectSSH("true", replacedWorker)

		disruption.Run("Machine Shutdown and Restore", "machine_failure",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				controlplane.NewKubeAvailableTest(),
				controlplane.NewOpenShiftAvailableTest(),
				controlplane.NewOAuthAvailableTest(),
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

				// framework.Logf("Verify etcd endpoints are healthy")
				// certsDir := "/tmp/etcd-client-certs/"
				// dumpEtcdCertsOnDisk(oc, certsDir)
				// defer os.RemoveAll(certsDir)
				// unhealthyEtcds := getUnhealthyEtcds(certsDir, masters)
				// o.Expect(len(unhealthyEtcds)).To(o.BeNumerically("==", 0))

				// createMachineHealthCheckForRole("master")
				// defer deleteMachineCheckForRole("master")
				createMachineHealthCheckForRole("worker")
				defer deleteMachineCheckForRole("worker")

				replacedWorkerMachineName := getMachineNameByNodeName(oc, replacedWorker.Name)
				// replacedMasterMachineName := getMachineNameByNodeName(oc, replacedMaster.Name)
				// replacedMasterMachine, err := ms.Get(context.Background(), replacedMasterMachineName, metav1.GetOptions{})
				// o.Expect(err).NotTo(o.HaveOccurred())

				// add controller reference
				// replacedMasterMachineCopy := replacedMasterMachine.DeepCopy()
				// ownerReferences := getOwnerReferenceForMasterMachine(replacedMasterMachineCopy)
				// replacedMasterMachineCopy.SetOwnerReferences(ownerReferences)
				// _, err = ms.Update(context.Background(), replacedMasterMachineCopy, metav1.UpdateOptions{})
				// o.Expect(err).NotTo(o.HaveOccurred())

				targets := []*corev1.Node{ /* replacedMaster , */ replacedWorker}
				targetMachineNames := []string{ /* replacedMasterMachineName, */ replacedWorkerMachineName}

				// we use a hard shutdown to simulate a poweroff
				for _, target := range targets {
					framework.Logf("Forcing shutdown of node %s", target.Name)
					_, err = ssh("sudo -i systemctl poweroff --force --force", target)
				}

				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for nodes to go unready")
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					nodes, err := pollClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
					if err != nil || nodes.Items == nil {
						framework.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
						return false, nil
					}
					notReadyNodes := sets.NewString()
					for _, node := range nodes.Items {
						nodeReady := true
						for _, t := range node.Spec.Taints {
							if t.Key == "node.kubernetes.io/unreachable" {
								nodeReady = false
								break
							}
						}
						if !nodeReady {
							notReadyNodes.Insert(node.Name)
						}
					}
					if notReadyNodes.Len() != len(targets) {
						framework.Logf("Nodes waiting to go unready: %v", notReadyNodes.List())
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				// etcdMemberToRemove := getEtcdMemberToRemove(oc, replacedMaster.Name)
				// framework.Logf("remove etcd member with ID %s", etcdMemberToRemove)
				// removeMember(oc, etcdMemberToRemove)

				// framework.Logf("Recreating master %s", replacedMasterMachineName+"-duplicate")
				// newMaster := replacedMasterMachine.DeepCopy()
				// // The providerID is relied upon by the machine controller to determine a machine
				// // has been provisioned
				// // https://github.com/openshift/cluster-api/blob/c4a461a19efb8a25b58c630bed0829512d244ba7/pkg/controller/machine/controller.go#L306-L308
				// unstructured.SetNestedField(newMaster.Object, "", "spec", "providerID")
				// newMaster.SetName(replacedMasterMachineName + "-duplicate")
				// newMaster.SetResourceVersion("")
				// newMaster.SetSelfLink("")
				// newMaster.SetUID("")
				// newMaster.SetCreationTimestamp(metav1.NewTime(time.Time{}))
				// // retry until the machine gets created
				// err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
				// 	_, err := ms.Create(context.Background(), newMaster, metav1.CreateOptions{})
				// 	if errors.IsAlreadyExists(err) {
				// 		framework.Logf("Waiting for old machine object %s to be deleted so we can create a new one", replacedMaster.Name)
				// 		return false, nil
				// 	}
				// 	if err != nil {
				// 		return false, err
				// 	}
				// 	return true, nil
				// })
				// o.Expect(err).NotTo(o.HaveOccurred())

				// framework.Logf("Wait for masters to join as nodes and go ready")
				// err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
				// 	defer func() {
				// 		if r := recover(); r != nil {
				// 			fmt.Println("Recovered from panic", r)
				// 		}
				// 	}()
				// 	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
				// 	if err != nil {
				// 		return false, err
				// 	}
				// 	ready := countReady(nodes.Items)
				// 	if ready != len(masters) {
				// 		framework.Logf("%d master nodes still unready", len(masters)-ready)
				// 		return false, nil
				// 	}
				// 	return true, nil
				// })
				// o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for worker to join as nodes and go ready")
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					defer func() {
						if r := recover(); r != nil {
							fmt.Println("Recovered from panic", r)
						}
					}()
					nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="})
					if err != nil {
						return false, err
					}
					ready := countReady(nodes.Items)
					if ready != len(workers) {
						framework.Logf("%d worker nodes still unready", len(workers)-ready)
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Wait for old machines to be deleted")
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					machines, err := ms.List(context.Background(), metav1.ListOptions{})
					if err != nil || machines.Items == nil {
						framework.Logf("return false - err %v nodes.Items %v", err, machines.Items)
						return false, nil
					}
					vanishedMachines := sets.NewString()
					for _, machine := range targetMachineNames {
						vanishedMachines.Insert(machine)
					}
					for _, machine := range machines.Items {
						vanishedMachines.Delete(machine.GetName())
					}
					if vanishedMachines.Len() != len(targetMachineNames) {
						framework.Logf("Machines waiting to go be deleted: %v", vanishedMachines.List())
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			})
	},
	)
})

func getEtcdMemberToRemove(oc *exutil.CLI, unhealthyNodeName string) string {
	var healthyEtcdPod string
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, node := range nodes.Items {
		nodeReady := true
		for _, t := range node.Spec.Taints {
			if t.Key == "node.kubernetes.io/unreachable" {
				nodeReady = false
				break
			}
		}
		if nodeReady {
			healthyEtcdPod = "etcd-" + node.Name
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	var memberListOutput string
	// give 2 mins for api to be up and retry
	err = wait.Poll(2*time.Second, 2*time.Minute, func() (done bool, err error) {
		memberListOutput, err = oc.AsAdmin().Run("exec").Args("-n", "openshift-etcd", healthyEtcdPod, "-c", "etcdctl", "--", "etcdctl", "memberListOutput", "list").Output()
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	for _, memberLine := range strings.Split(memberListOutput, "\n") {
		if strings.Contains(memberLine, unhealthyNodeName) {
			return strings.Split(memberLine, ", ")[0]
		}
	}
	o.Expect(fmt.Errorf("could not find memberListOutput name %s in memberListOutput output %s", unhealthyNodeName, memberListOutput)).NotTo(o.HaveOccurred())
	return ""
}

func removeMember(oc *exutil.CLI, memberID string) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
	o.Expect(err).NotTo(o.HaveOccurred())

	var healthyEtcdPod string
	for _, node := range nodes.Items {
		nodeReady := true
		for _, t := range node.Spec.Taints {
			if t.Key == "node.kubernetes.io/unreachable" {
				nodeReady = false
				break
			}
		}
		if nodeReady {
			healthyEtcdPod = "etcd-" + node.Name
			break
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	member, err := oc.AsAdmin().Run("exec").Args("-n", "openshift-etcd", healthyEtcdPod, "-c", "etcdctl", "etcdctl", "member", "remove", memberID).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(member).To(o.ContainSubstring("removed from cluster"))
}

func deleteMachineCheckForRole(role string) {
	config, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	dynamicClient := dynamic.NewForConfigOrDie(config)
	mhc := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "machine.openshift.io",
		Version:  "v1beta1",
		Resource: "machinehealthchecks",
	}).Namespace("openshift-machine-api")
	err = mhc.Delete(context.Background(), "e2e-health-check-"+role, metav1.DeleteOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
}

func createMachineHealthCheckForRole(role string) {
	config, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	dynamicClient := dynamic.NewForConfigOrDie(config)
	mhc := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "machine.openshift.io",
		Version:  "v1beta1",
		Resource: "machinehealthchecks",
	}).Namespace("openshift-machine-api")
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "machine.openshift.io",
		Version: "v1beta1",
		Kind:    "MachineHealthCheck",
	})
	u.SetName("e2e-health-check-" + role)
	u.SetNamespace("openshift-machine-api")
	err = unstructured.SetNestedField(u.Object, role, "spec", "selector", "matchLabels", "machine.openshift.io/cluster-api-machine-role")
	o.Expect(err).ToNot(o.HaveOccurred())
	err = unstructured.SetNestedField(u.Object, []interface{}{
		map[string]interface{}{
			"type":    "Ready",
			"timeout": "5m",
			"status":  "False",
		},
		map[string]interface{}{
			"type":    "Ready",
			"timeout": "5m",
			"status":  "Unknown",
		},
	}, "spec", "unhealthyConditions")
	o.Expect(err).ToNot(o.HaveOccurred())
	_, err = mhc.Create(context.Background(), u, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
}

func getOwnerReferenceForMasterMachine(obj metav1.Object) []metav1.OwnerReference {
	o := metav1.NewControllerRef(obj, schema.GroupVersionKind{
		Group:   "machine.openshift.io",
		Version: "v1beta1",
		Kind:    "MachineSet",
	})
	return []metav1.OwnerReference{*o}
}

func getUnhealthyEtcds(certsDir string, masters []*corev1.Node) []*etcdserverpb.Member {
	endpoints := getEtcdEndpoints(masters)
	etcdClient, err := getEtcdClient(certsDir, endpoints)
	o.Expect(err).ToNot(o.HaveOccurred())
	memberListResp, err := etcdClient.MemberList(context.Background())
	o.Expect(err).ToNot(o.HaveOccurred())

	unhealthEtcds := []*etcdserverpb.Member{}

	for _, m := range memberListResp.Members {
		_, err := etcdClient.Status(context.Background(), m.ClientURLs[0])
		if err == nil {
			unhealthEtcds = append(unhealthEtcds, m)
		}
	}
	return unhealthEtcds
}

func getEtcdClient(certsDir string, endpoints []string) (*clientv3.Client, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithBlock(), // block until the underlying connection is up
	}

	tlsInfo := transport.TLSInfo{
		CertFile:      path.Join(certsDir, "tls.crt"),
		KeyFile:       path.Join(certsDir, "tls.key"),
		TrustedCAFile: path.Join(certsDir, "ca-bundle.crt"),
	}
	tlsConfig, err := tlsInfo.ClientConfig()

	cfg := &clientv3.Config{
		DialOptions: dialOptions,
		Endpoints:   endpoints,
		DialTimeout: 15 * time.Second,
		TLS:         tlsConfig,
	}

	cli, err := clientv3.New(*cfg)
	if err != nil {
		return nil, err
	}
	return cli, err
}

func dumpEtcdCertsOnDisk(oc *exutil.CLI, dir string) {
	err := os.MkdirAll(dir, os.ModePerm)
	o.Expect(err).ToNot(o.HaveOccurred())

	etcdCA, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "etcd-ca-bundle", metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	caData, ok := etcdCA.Data["ca-bundle.crt"]
	if !ok {
		o.Expect(fmt.Errorf("etcd CA data  missing in configmap openshift-config/etcd-ca-bundle")).ToNot(o.HaveOccurred())
	}

	etcdClientCerts, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "etcd-client", metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	clientCert, ok := etcdClientCerts.Data["tls.crt"]
	if !ok {
		o.Expect(fmt.Errorf("etcd client Certificate data  missing in secret openshift-config/etcd-client")).ToNot(o.HaveOccurred())
	}
	clientKey, ok := etcdClientCerts.Data["tls.key"]
	if !ok {
		o.Expect(fmt.Errorf("etcd client Private Key data  missing in secret openshift-config/etcd-client")).ToNot(o.HaveOccurred())
	}

	err = ioutil.WriteFile(path.Join(dir, "ca-bundle.crt"), []byte(caData), 0600)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = ioutil.WriteFile(path.Join(dir, "tls.crt"), []byte(clientCert), 0600)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = ioutil.WriteFile(path.Join(dir, "tls.key"), []byte(clientKey), 0600)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getEtcdEndpoints(masters []*corev1.Node) []string {
	endpoints := []string{}
	for _, m := range masters {
		for _, addr := range m.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				endpoints = append(endpoints, "https://"+net.JoinHostPort(addr.Address, "2379"))
				break
			}
		}
	}

	return endpoints
}
