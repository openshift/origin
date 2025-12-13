package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nadtypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

const nodeLabelSelectorWorker = "node-role.kubernetes.io/worker,!node-role.kubernetes.io/edge,!node-role.kubernetes.io/infra"

var _ = g.Describe("[sig-network][Feature:tap]", func() {
	oc := exutil.NewCLI("tap")
	f := oc.KubeFramework()
	var worker *corev1.Node
	var isCUDDisabled bool

	g.BeforeEach(func() {
		// Fetch worker nodes.
		workerNodes, err := f.ClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		if len(workerNodes.Items) == 0 {
			e2e.Failf("cluster should have nodes")
		}
		// Preventing to select worker nodes in variants which applies NoSchedule taints
		// to some worker nodes, preventing this test to fail to schedule.
		for idx, wk := range workerNodes.Items {
			if len(wk.Spec.Taints) == 0 {
				worker = &workerNodes.Items[idx]
				break
			}
		}
		o.Expect(worker).NotTo(o.BeNil())

		// Load tun module.
		_, err = exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, worker, []string{
			"sh", "-c", "nsenter --mount=/proc/1/ns/mnt -- sh -c 'modprobe tun'",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Get container_use_devices selinux boolean.
		cud, err := exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, worker, []string{
			"sh", "-c", "nsenter --mount=/proc/1/ns/mnt -- sh -c 'getsebool container_use_devices'",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		isCUDDisabled = strings.Contains(cud, "off")

		if isCUDDisabled {
			// Enable container_use_devices selinux boolean.
			_, err = exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, worker, []string{
				"sh", "-c", "nsenter --mount=/proc/1/ns/mnt -- sh -c 'setsebool container_use_devices 1'",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.AfterEach(func() {
		if isCUDDisabled {
			// Disable container_use_devices selinux boolean.
			_, err := exutil.ExecCommandOnMachineConfigDaemon(f.ClientSet, oc, worker, []string{
				"sh", "-c", "nsenter --mount=/proc/1/ns/mnt -- sh -c 'setsebool container_use_devices 0'",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It(fmt.Sprintf("should create a pod with a tap interface [apigroup:k8s.cni.cncf.io]"), g.Label("Size:M"), func() {
		ns := f.Namespace.Name
		podName := "pod1"
		nadName := "nad-tap"
		ifName := "tap1"
		nadConfig := `{
			"cniVersion":"0.4.0",
			"name":"%s",
			"type": "tap",
			"selinuxcontext": "system_u:system_r:container_t:s0"
		}`

		g.By("creating a network attachment definition")
		err := createNetworkAttachmentDefinition(
			oc.AdminConfig(),
			ns,
			nadName,
			fmt.Sprintf(nadConfig, nadName),
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create tap network-attachment-definition")

		g.By("creating a pod on worker with container_use_devices on")
		exutil.CreateExecPodOrFail(f.ClientSet, ns, podName, func(pod *kapiv1.Pod) {
			tapAnnotation := fmt.Sprintf("%s/%s@%s", ns, nadName, ifName)
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s", tapAnnotation)}
			pod.Spec.NodeSelector = worker.Labels
		})
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("checking annotations")
		networkStatusString, ok := pod.Annotations["k8s.v1.cni.cncf.io/network-status"]
		o.Expect(ok).To(o.BeTrue())
		o.Expect(networkStatusString).ToNot(o.BeNil())

		var networkStatuses []nadtypes.NetworkStatus
		o.Expect(json.Unmarshal([]byte(networkStatusString), &networkStatuses)).ToNot(o.HaveOccurred())
		o.Expect(networkStatuses).To(o.HaveLen(2))
		o.Expect(networkStatuses[1].Interface).To(o.Equal(ifName))
		o.Expect(networkStatuses[1].Name).To(o.Equal(fmt.Sprintf("%s/%s", ns, nadName)))
	})
})
