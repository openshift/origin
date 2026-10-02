package node

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	gomaxprocsPoolName        = "gomaxprocs-test"
	gomaxprocsNodeLabel       = "node-role.kubernetes.io/gomaxprocs-test"
	gomaxprocsKubeletConfig   = "gomaxprocs-system"
	gomaxprocsContainerConfig = "gomaxprocs-container"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive][OCPFeatureGate:GomaxprocsInjection] GOMAXPROCS injection", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("node-gomaxprocs")
	var (
		mcClient *machineconfigclient.Clientset
		nodeName string
	)

	g.BeforeAll(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("skipping on MicroShift")
		}
		enabled, err := exutil.IsFeatureGateEnabled(ctx, oc.AdminConfigClient(), "GomaxprocsInjection")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !enabled {
			g.Skip("GomaxprocsInjection feature gate is not enabled")
		}

		mcClient, err = machineconfigclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		workers, err := exutil.GetReadySchedulableWorkerNodes(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(workers) == 0 {
			g.Skip("no worker node available for a custom machine config pool")
		}
		nodeName = workers[rand.Intn(len(workers))].Name

		_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType,
			[]byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, gomaxprocsNodeLabel)), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(cleanupCtx, gomaxprocsContainerConfig, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				framework.Logf("cleanup failed for %s: %v", gomaxprocsContainerConfig, err)
			}
			err = mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, gomaxprocsKubeletConfig, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				framework.Logf("cleanup failed for %s: %v", gomaxprocsKubeletConfig, err)
			}
			_ = WaitForMCP(cleanupCtx, mcClient, gomaxprocsPoolName, 15*time.Minute)
			_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(cleanupCtx, nodeName, types.MergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, gomaxprocsNodeLabel)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func() bool {
				node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, nodeName, metav1.GetOptions{})
				return err == nil && node.Annotations["machineconfiguration.openshift.io/currentConfig"] == node.Annotations["machineconfiguration.openshift.io/desiredConfig"] && !strings.Contains(node.Annotations["machineconfiguration.openshift.io/currentConfig"], gomaxprocsPoolName)
			}, 15*time.Minute, 10*time.Second).Should(o.BeTrue())
			err = mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, gomaxprocsPoolName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				framework.Logf("cleanup failed for %s: %v", gomaxprocsPoolName, err)
			}
		})

		_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, &mcfgv1.MachineConfigPool{
			ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsPoolName, Labels: map[string]string{
				"machineconfiguration.openshift.io/pool":                                 gomaxprocsPoolName,
				"pools.operator.machineconfiguration.openshift.io/" + gomaxprocsPoolName: "",
			}},
			Spec: mcfgv1.MachineConfigPoolSpec{
				MachineConfigSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "machineconfiguration.openshift.io/role", Operator: metav1.LabelSelectorOpIn, Values: []string{"worker", gomaxprocsPoolName}}}},
				NodeSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{gomaxprocsNodeLabel: ""}},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(WaitForMCP(ctx, mcClient, gomaxprocsPoolName, 15*time.Minute)).To(o.Succeed())
	})

	g.It("sets GOMAXPROCS for kubelet and CRI-O", func(ctx context.Context) {
		_, err := mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, &mcfgv1.KubeletConfig{
			ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsKubeletConfig},
			Spec: mcfgv1.KubeletConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{MatchLabels: map[string]string{fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", gomaxprocsPoolName): ""}},
				AutoSizingReserved:        new(false),
				SystemGomaxprocsBehavior:  mcfgv1.GomaxprocsBehaviorAutosize,
				KubeletConfig:             &runtime.RawExtension{Raw: []byte(`{"systemReserved":{"cpu":"1500m"}}`)},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(WaitForMCPUpdating(ctx, mcClient, gomaxprocsPoolName, 5*time.Minute)).To(o.Succeed())
		o.Expect(WaitForMCP(ctx, mcClient, gomaxprocsPoolName, 15*time.Minute)).To(o.Succeed())

		for _, service := range []string{"kubelet.service", "crio.service"} {
			output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", fmt.Sprintf(`tr '\000' '\n' < /proc/$(systemctl show -p MainPID --value %s)/environ | grep '^GOMAXPROCS='`, service))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("GOMAXPROCS=2"))
		}
	})

	g.It("injects GOMAXPROCS into containers from their CPU request", func(ctx context.Context) {
		_, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, &mcfgv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsContainerConfig},
			Spec: mcfgv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{MatchLabels: map[string]string{fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", gomaxprocsPoolName): ""}},
				ContainerRuntimeConfig:    &mcfgv1.ContainerRuntimeConfiguration{ContainerGomaxprocsBehavior: mcfgv1.GomaxprocsBehaviorAutosize},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(WaitForMCPUpdating(ctx, mcClient, gomaxprocsPoolName, 5*time.Minute)).To(o.Succeed())
		o.Expect(WaitForMCP(ctx, mcClient, gomaxprocsPoolName, 15*time.Minute)).To(o.Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "gomaxprocs-container", Namespace: oc.Namespace()},
			Spec: corev1.PodSpec{
				NodeName:      nodeName,
				RestartPolicy: corev1.RestartPolicyNever,
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Containers: []corev1.Container{{
					Name:    "check",
					Image:   image.LocationFor("registry.k8s.io/e2e-test-images/busybox:1.37.0-1"),
					Command: []string{"sh", "-c", "printf '%s\\n' \"$GOMAXPROCS\""},
					Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("750m"),
					}},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					},
				}},
			},
		}
		_, err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, oc.KubeClient(), pod.Name, oc.Namespace(), 2*time.Minute)).To(o.Succeed())

		output, err := oc.Run("logs").Args(pod.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(output)).To(o.Equal("2"))
	})
})
