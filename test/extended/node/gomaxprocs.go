package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	gomaxprocsPoolName        = "gomaxprocs-test"
	gomaxprocsKubeletConfig   = "gomaxprocs-system"
	gomaxprocsContainerConfig = "gomaxprocs-container"

	// Neither expected value depends on the node size: system services get
	// max(ceil(systemReserved cpu), 1) so 1500m renders 2, and containers get
	// max(ceil(2 * cpu request), 1) so 750m renders 2.
	gomaxprocsExpected = "2"
)

var _ = g.Describe("[Suite:openshift/nodes/isolated][sig-node][OCPFeatureGate:GomaxprocsInjection] GOMAXPROCS injection",
	g.Label("NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=gomaxprocs"),
	g.Ordered, func() {
		defer g.GinkgoRecover()

		oc := exutil.NewCLI("node-gomaxprocs")
		var (
			mcClient *machineconfigclient.Clientset
			nodeName string
		)

		poolSelector := func() *metav1.LabelSelector {
			return &metav1.LabelSelector{MatchLabels: map[string]string{
				fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", gomaxprocsPoolName): "",
			}}
		}

		g.BeforeAll(func(ctx context.Context) {
			SkipOnMicroShift(oc)
			// custom MCPs are not supported on clusters whose only pool is the master pool
			skipOnSingleNodeTopology(oc)
			skipOnTwoNodeTopology(oc)
			// the [OCPFeatureGate:...] label already filters this test, this check is for direct run-test runs
			enabled, err := exutil.IsFeatureGateEnabled(ctx, oc.AdminConfigClient(), "GomaxprocsInjection")
			o.Expect(err).NotTo(o.HaveOccurred())
			if !enabled {
				g.Skip("GomaxprocsInjection feature gate is not enabled")
			}

			EnsureNodeResourceNodesReady(ctx, oc, "gomaxprocs")
			mcClient, err = machineconfigclient.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeName, err = GetNodeResource(ctx, oc, "gomaxprocs")
			o.Expect(err).NotTo(o.HaveOccurred())

			pool, err := CreateCustomMCPForNode(ctx, oc, mcClient, gomaxprocsPoolName, nodeName)
			// a non-nil pool means the MCP exists, even when its readiness wait failed, so register the
			// cleanup before asserting: CleanupCustomMCP always deletes the pool
			if pool != nil {
				g.DeferCleanup(func() {
					o.Expect(CleanupCustomMCP(context.Background(), pool)).To(o.Succeed())
				})
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// openshift-tests runs each It in its own run-test process, so BeforeAll and its cleanup run
		// once per It: keep this a single It so the pool is built and torn down only once.
		g.It("sets GOMAXPROCS for kubelet, CRI-O and containers", func(ctx context.Context) {
			g.DeferCleanup(func() {
				o.Expect(CleanupKubeletConfig(context.Background(), mcClient, gomaxprocsKubeletConfig, gomaxprocsPoolName)).To(o.Succeed())
			})

			kc := &mcfgv1.KubeletConfig{
				ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsKubeletConfig},
				Spec: mcfgv1.KubeletConfigSpec{
					MachineConfigPoolSelector: poolSelector(),
					AutoSizingReserved:        new(false),
					SystemGomaxprocsBehavior:  mcfgv1.GomaxprocsBehaviorAutosize,
					KubeletConfig:             &runtime.RawExtension{Raw: []byte(`{"systemReserved":{"cpu":"1500m"}}`)},
				},
			}
			o.Expect(ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kc, gomaxprocsPoolName, 15*time.Minute)).To(o.Succeed())

			for _, service := range []string{"kubelet.service", "crio.service"} {
				output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", fmt.Sprintf(`tr '\000' '\n' < /proc/$(systemctl show -p MainPID --value %s)/environ | grep '^GOMAXPROCS='`, service))
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.TrimSpace(output)).To(o.Equal("GOMAXPROCS=" + gomaxprocsExpected))
			}

			g.DeferCleanup(func() {
				o.Expect(CleanupContainerRuntimeConfig(context.Background(), mcClient, gomaxprocsContainerConfig, gomaxprocsPoolName)).To(o.Succeed())
			})

			crc := &mcfgv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsContainerConfig},
				Spec: mcfgv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: poolSelector(),
					ContainerRuntimeConfig: &mcfgv1.ContainerRuntimeConfiguration{
						ContainerGomaxprocsBehavior: mcfgv1.GomaxprocsBehaviorAutosize,
					},
				},
			}
			o.Expect(ApplyContainerRuntimeConfigAndWaitForMCP(ctx, mcClient, crc, gomaxprocsPoolName, 15*time.Minute)).To(o.Succeed())

			for _, tc := range []struct {
				name     string
				env      []corev1.EnvVar
				expected string
			}{
				{name: "gomaxprocs-container", expected: gomaxprocsExpected},
				// a GOMAXPROCS set in the pod spec wins over the injected one
				{name: "gomaxprocs-container-override", env: []corev1.EnvVar{{Name: "GOMAXPROCS", Value: "3"}}, expected: "3"},
			} {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: oc.Namespace()},
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
							Image:   image.LocationFor("registry.k8s.io/e2e-test-images/busybox:1.37.0-2"),
							Command: []string{"sh", "-c", "printf '%s\\n' \"$GOMAXPROCS\""},
							Env:     tc.env,
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
				_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, oc.KubeClient(), pod.Name, oc.Namespace(), 2*time.Minute)).To(o.Succeed())

				output, err := oc.Run("logs").Args(pod.Name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.TrimSpace(output)).To(o.Equal(tc.expected), tc.name)
			}
		})
	})
