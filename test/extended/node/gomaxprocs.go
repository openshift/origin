package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	gomaxprocsPoolName        = "gomaxprocs-test"
	gomaxprocsKubeletConfig   = "gomaxprocs-system"
	gomaxprocsContainerConfig = "gomaxprocs-container"

	// The system case rolls the node four times. Keep each wait short enough that a stuck
	// rollout fails on its own step instead of the suite's 60 minute test timeout.
	gomaxprocsRolloutTimeout = 10 * time.Minute
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
					if cleanupErr := CleanupCustomMCP(context.Background(), pool); cleanupErr != nil {
						framework.Logf("Warning: cleanup had errors: %v", cleanupErr)
					}
				})
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// openshift-tests runs each It in its own run-test process, and Ginkgo ties cleanups
		// registered in BeforeAll to the end of the ordered container, so the pool is built and
		// torn down once per It. The cases are therefore grouped by the config they need, one
		// for the system services and one for containers, rather than one It per case.
		g.It("sizes system service GOMAXPROCS from the system reservation and drops it when disabled", func(ctx context.Context) {
			g.DeferCleanup(func() {
				if err := CleanupKubeletConfig(context.Background(), mcClient, gomaxprocsKubeletConfig, gomaxprocsPoolName); err != nil {
					framework.Logf("Warning: KubeletConfig cleanup failed: %v", err)
				}
			})

			// no pinned reservation, so node sizing computes it from the node's capacity
			g.By("Autosizing from the reservation node sizing computes")
			applyKubeletConfig(ctx, mcClient, mcfgv1.GomaxprocsBehaviorAutosize, "")
			expectSystemGomaxprocs(ctx, oc, nodeName, o.Equal(gomaxprocsFromAutoSizing(ctx, oc, nodeName)))

			g.By("Autosizing from a pinned 1500m reservation")
			applyKubeletConfig(ctx, mcClient, mcfgv1.GomaxprocsBehaviorAutosize, "1500m")
			expectSystemGomaxprocs(ctx, oc, nodeName, o.Equal("2"))

			g.By("Raising the pinned reservation to 2500m")
			applyKubeletConfig(ctx, mcClient, mcfgv1.GomaxprocsBehaviorAutosize, "2500m")
			expectSystemGomaxprocs(ctx, oc, nodeName, o.Equal("3"))

			g.By("Disabling and checking the injected value is removed")
			applyKubeletConfig(ctx, mcClient, mcfgv1.GomaxprocsBehaviorDisabled, "")
			// the operator writes the requested behavior here, so this proves the config reached the
			// node instead of the "nothing injected" assertion passing on the default state
			behavior, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", "/etc/system-gomaxprocs.env")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(behavior)).To(o.ContainSubstring("SYSTEM_GOMAXPROCS_BEHAVIOR=Disabled"))
			expectSystemGomaxprocs(ctx, oc, nodeName, o.BeEmpty())
		})

		// Containers get max(ceil(2 * cpu request), 1), so 750m renders 2 and 1500m renders 3.
		g.It("injects container GOMAXPROCS from the CPU request and stops when overridden or disabled", func(ctx context.Context) {
			g.DeferCleanup(func() {
				if err := CleanupContainerRuntimeConfig(context.Background(), mcClient, gomaxprocsContainerConfig, gomaxprocsPoolName); err != nil {
					framework.Logf("Warning: ContainerRuntimeConfig cleanup failed: %v", err)
				}
			})

			g.By("Enabling container autosizing")
			o.Expect(ApplyContainerRuntimeConfigAndWaitForMCP(ctx, mcClient,
				newGomaxprocsContainerRuntimeConfig(mcfgv1.GomaxprocsBehaviorAutosize), gomaxprocsPoolName, gomaxprocsRolloutTimeout)).To(o.Succeed())

			// the 750m pod is also the control the override stages below compare against
			g.By("Checking a 750m request is injected 2")
			o.Expect(runGomaxprocsPod(ctx, oc, nodeName, "gomaxprocs-container-750m", "750m", nil, nil)).To(o.Equal("2"))

			g.By("Checking a 1500m request is injected 3")
			o.Expect(runGomaxprocsPod(ctx, oc, nodeName, "gomaxprocs-container-1500m", "1500m", nil, nil)).To(o.Equal("3"))

			g.By("Checking a GOMAXPROCS set in the pod spec wins")
			o.Expect(runGomaxprocsPod(ctx, oc, nodeName, "gomaxprocs-container-env", "750m", nil,
				[]corev1.EnvVar{{Name: "GOMAXPROCS", Value: "3"}})).To(o.Equal("3"))

			g.By("Checking the skip-gomaxprocs annotation suppresses injection")
			o.Expect(runGomaxprocsPod(ctx, oc, nodeName, "gomaxprocs-container-skip", "750m",
				map[string]string{"skip-gomaxprocs.crio.io": "true"}, nil)).To(o.BeEmpty())

			g.By("Disabling injection and checking a 750m request is left alone")
			o.Expect(ApplyContainerRuntimeConfigAndWaitForMCP(ctx, mcClient,
				newGomaxprocsContainerRuntimeConfig(mcfgv1.GomaxprocsBehaviorDisabled), gomaxprocsPoolName, gomaxprocsRolloutTimeout)).To(o.Succeed())
			o.Expect(runGomaxprocsPod(ctx, oc, nodeName, "gomaxprocs-container-disabled", "750m", nil, nil)).To(o.BeEmpty())
		})
	})

func gomaxprocsPoolSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: map[string]string{poolOperatorLabel(gomaxprocsPoolName): ""}}
}

// newGomaxprocsContainerRuntimeConfig builds the pool's ContainerRuntimeConfig.
func newGomaxprocsContainerRuntimeConfig(behavior mcfgv1.GomaxprocsBehaviorType) *mcfgv1.ContainerRuntimeConfig {
	return &mcfgv1.ContainerRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsContainerConfig},
		Spec: mcfgv1.ContainerRuntimeConfigSpec{
			MachineConfigPoolSelector: gomaxprocsPoolSelector(),
			ContainerRuntimeConfig:    &mcfgv1.ContainerRuntimeConfiguration{ContainerGomaxprocsBehavior: behavior},
		},
	}
}

// applyKubeletConfig rolls the pool with a KubeletConfig. An empty systemReservedCPU leaves node
// sizing to compute the reservation, which is what a cluster without a pinned reservation runs.
func applyKubeletConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, behavior mcfgv1.GomaxprocsBehaviorType, systemReservedCPU string) {
	g.GinkgoHelper()

	kc := &mcfgv1.KubeletConfig{
		ObjectMeta: metav1.ObjectMeta{Name: gomaxprocsKubeletConfig},
		Spec: mcfgv1.KubeletConfigSpec{
			MachineConfigPoolSelector: gomaxprocsPoolSelector(),
			SystemGomaxprocsBehavior:  behavior,
		},
	}
	if systemReservedCPU != "" {
		kc.Spec.KubeletConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"systemReserved":{"cpu":%q}}`, systemReservedCPU))}
	}

	o.Expect(ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kc, gomaxprocsPoolName, gomaxprocsRolloutTimeout)).To(o.Succeed())
}

// expectSystemGomaxprocs asserts kubelet and CRI-O match matcher: o.Equal for an injected value,
// o.BeEmpty when neither may be injected and both fall back to the Go default.
func expectSystemGomaxprocs(ctx context.Context, oc *exutil.CLI, nodeName string, matcher gomegatypes.GomegaMatcher) {
	g.GinkgoHelper()
	for _, service := range []string{"kubelet.service", "crio.service"} {
		output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", fmt.Sprintf(`
		pid=$(systemctl show -p MainPID --value %s)
		if [ "$pid" = 0 ]; then echo "%s is not running" >&2; exit 1; fi
		tr '\000' '\n' < /proc/$pid/environ | sed -n 's/^GOMAXPROCS=//p'`, service, service))
		o.Expect(err).NotTo(o.HaveOccurred(), "GOMAXPROCS of "+service)
		o.Expect(strings.TrimSpace(output)).To(matcher, "GOMAXPROCS of "+service)
	}
}

// gomaxprocsFromAutoSizing returns the GOMAXPROCS the reservation node sizing computed for the node
// implies, max(ceil(systemReserved cpu), 1), so the default (unpinned) path is asserted against the
// same number the node computed instead of a literal.
func gomaxprocsFromAutoSizing(ctx context.Context, oc *exutil.CLI, nodeName string) string {
	g.GinkgoHelper()
	const autoSizingConf = "/etc/openshift/kubelet.conf.d/20-auto-sizing.conf"

	output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", autoSizingConf)
	o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read "+autoSizingConf)

	var autoSizing struct {
		SystemReserved map[string]string `json:"systemReserved"`
	}
	o.Expect(yaml.Unmarshal([]byte(output), &autoSizing)).To(o.Succeed())

	cpu, err := resource.ParseQuantity(autoSizing.SystemReserved["cpu"])
	o.Expect(err).NotTo(o.HaveOccurred(), "systemReserved.cpu should be set in "+autoSizingConf)

	expected := max(int((cpu.MilliValue()+999)/1000), 1)
	framework.Logf("auto-sized systemReserved cpu %s implies GOMAXPROCS %d", cpu.String(), expected)
	return strconv.Itoa(expected)
}

// runGomaxprocsPod runs a pod pinned to nodeName and returns the GOMAXPROCS its process saw, ""
// when CRI-O injected nothing. CPU requests only: CRI-O skips pods with a limit (Guaranteed)
// because Go 1.25+ reads the cgroup quota itself.
func runGomaxprocsPod(ctx context.Context, oc *exutil.CLI, nodeName, name, cpuRequest string, annotations map[string]string, env []corev1.EnvVar) string {
	g.GinkgoHelper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: oc.Namespace(), Annotations: annotations},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:    "check",
				Image:   image.LocationFor("registry.k8s.io/e2e-test-images/busybox:1.37.0-2"),
				Command: []string{"sh", "-c", `printf '%s\n' "$GOMAXPROCS"`},
				Env:     env,
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse(cpuRequest),
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
	o.Expect(e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, oc.KubeClient(), name, oc.Namespace(), 2*time.Minute)).To(o.Succeed())

	output, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(name, &corev1.PodLogOptions{}).DoRaw(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.TrimSpace(string(output))
}
