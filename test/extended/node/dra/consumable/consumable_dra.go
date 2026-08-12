package consumable

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	nodeutils "github.com/openshift/origin/test/extended/node"
	dracommon "github.com/openshift/origin/test/extended/node/dra/common"
	draexample "github.com/openshift/origin/test/extended/node/dra/example"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	netDriverName  = "net.example.com"
	netDeviceClass = "net.example.com"

	// numNICs is the number of NIC devices per node. Kept low so the
	// capacity-exhaustion test can deterministically consume all bandwidth
	// without requiring an excessive number of claims.
	numNICs = 2
)

var (
	prerequisitesOnce      sync.Once
	prerequisitesInstalled bool
	prerequisitesError     error
	cachedInstaller        *draexample.PrerequisitesInstaller
)

var _ = g.Describe("[sig-scheduling][Feature:DRAConsumableCapacity][Suite:openshift/dra-example][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("dra-consumable", admissionapi.LevelPrivileged)

	var (
		prereqInstaller   *draexample.PrerequisitesInstaller
		capacityValidator *dracommon.CapacityValidator
		builder           *dracommon.ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		builder = dracommon.NewResourceBuilder(oc.Namespace(), dracommon.DriverConfig{
			DriverName:       netDriverName,
			DefaultClass:     netDeviceClass,
			RequestName:      "nic",
			ContainerImage:   image.ShellImage(),
			ContainerCommand: []string{"sh", "-c", "echo DRA NIC device allocated && sleep infinity"},
			LongRunCommand:   []string{"sh", "-c", "while true; do echo DRA NIC active; sleep 60; done"},
		})
	})

	g.Context("Consumable Capacity", g.Ordered, func() {
		var targetNodeName string

		g.BeforeAll(func(ctx context.Context) {
			nodeutils.SkipOnMicroShift(oc)

			capacityValidator = dracommon.NewCapacityValidator(oc.KubeFramework().ClientSet, netDriverName)
			if cachedInstaller == nil {
				cachedInstaller = draexample.NewPrerequisitesInstaller(oc.KubeFramework())
			}
			prereqInstaller = cachedInstaller

			prerequisitesOnce.Do(func() {
				framework.Logf("Checking DRA example driver prerequisites for consumable capacity tests")

				if prereqInstaller.IsDriverInstalled(ctx) {
					framework.Logf("DRA example driver already installed")
					prerequisitesInstalled = true
					return
				}

				framework.Logf("Installing DRA example driver...")
				if err := prereqInstaller.InstallAll(ctx); err != nil {
					prerequisitesError = err
					framework.Logf("ERROR: Failed to install DRA example driver: %v", err)
					return
				}

				prerequisitesInstalled = true
				framework.Logf("DRA example driver installation completed successfully")
			})

			if prerequisitesError != nil {
				if stderrors.Is(prerequisitesError, draexample.ErrToolingUnavailable) {
					g.Skip(fmt.Sprintf("Required tooling unavailable: %v", prerequisitesError))
				}
				g.Fail(fmt.Sprintf("DRA example driver prerequisites failed: %v", prerequisitesError))
			}
			if !prerequisitesInstalled {
				g.Fail("DRA example driver prerequisites not installed")
			}

			framework.Logf("Upgrading DRA example driver to net profile with numDevices=%d", numNICs)
			err := prereqInstaller.HelmUpgrade(ctx,
				"deviceProfile=net",
				fmt.Sprintf("kubeletPlugin.numDevices=%d", numNICs),
			)
			framework.ExpectNoError(err, "Failed to helm upgrade driver to net profile")

			framework.Logf("Waiting for driver to stabilize after upgrade...")
			err = prereqInstaller.WaitForDriver(ctx, 5*time.Minute)
			framework.ExpectNoError(err, "Driver failed to stabilize after helm upgrade")

			err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
				return capacityValidator.HasCapacityDevices(ctx)
			})
			if err != nil {
				if wait.Interrupted(err) {
					g.Skip("DRAConsumableCapacity not available — no devices with Capacity published after upgrade")
				}
				framework.Failf("Failed to check for capacity devices: %v", err)
			}

			framework.Logf("Capacity devices detected — DRAConsumableCapacity is active")

			targetNodeName, err = capacityValidator.GetNodeWithDevices(ctx)
			framework.ExpectNoError(err, "Failed to find a node with published devices")
			framework.Logf("Using node %s for consumable capacity tests", targetNodeName)
		})

		g.AfterAll(func(ctx context.Context) {
			framework.Logf("Restoring DRA example driver to default gpu profile")
			err := prereqInstaller.HelmUpgrade(ctx,
				"deviceProfile=gpu",
				"kubeletPlugin.numDevices=8",
			)
			if err != nil {
				framework.Logf("Warning: failed to restore driver to default mode: %v", err)
				return
			}
			if waitErr := prereqInstaller.WaitForDriver(ctx, 5*time.Minute); waitErr != nil {
				framework.Logf("Warning: driver did not stabilize after restore: %v", waitErr)
			}
		})

		g.It("should publish ResourceSlices with device Capacity and AllowMultipleAllocations", func(ctx context.Context) {
			g.By("Validating device capacity entries")
			err := capacityValidator.ValidateDeviceCapacity(ctx, []string{"ingressBandwidth", "egressBandwidth", "vfs"})
			framework.ExpectNoError(err, "Device capacity validation failed")

			g.By("Validating AllowMultipleAllocations is set on all devices")
			err = capacityValidator.ValidateAllowMultipleAllocations(ctx)
			framework.ExpectNoError(err, "AllowMultipleAllocations validation failed")

			g.By("Verifying NIC device count")
			count, err := capacityValidator.GetTotalDeviceCount(ctx)
			framework.ExpectNoError(err, "Failed to count capacity devices")
			o.Expect(count).To(o.BeNumerically(">", 0),
				"Expected capacity devices after switching to net profile")
			framework.Logf("Found %d NIC device(s) with capacity across all nodes", count)
		})

		g.It("should reject a request exceeding remaining capacity while accepting one that fits", func(ctx context.Context) {
			nodeName := targetNodeName

			deviceClassName := "test-consumable-partial-" + oc.Namespace()
			consumeClaimName := "test-partial-consume-claim"
			overflowClaimName := "test-partial-overflow-claim"
			fitClaimName := "test-partial-fit-claim"
			consumePodName := "test-partial-consume-pod"
			overflowPodName := "test-partial-overflow-pod"
			fitPodName := "test-partial-fit-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := dracommon.CreateDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
				defer cancel()
				if delErr := dracommon.DeleteDeviceClass(cleanupCtx, oc.KubeFramework().ClientSet, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			// Each NIC has 100G bandwidth. Consuming 90G on each of the 2
			// NICs leaves 10G free per NIC. A 15G request cannot fit on any
			// NIC, while a 10G request fits exactly.
			g.By(fmt.Sprintf("Creating ResourceClaim requesting %d NICs with 90G bandwidth to partially exhaust capacity", numNICs))
			consumeClaim := builder.BuildResourceClaimWithCapacity(consumeClaimName, deviceClassName, numNICs, map[string]string{
				"ingressBandwidth": "90G",
				"egressBandwidth":  "90G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), consumeClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaimAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), consumeClaimName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), consumeClaimName, delErr)
				}
			}()

			g.By("Creating pod pinned to target node to consume 90G per NIC")
			consumePod := builder.BuildLongRunningPodWithClaim(consumePodName, consumeClaimName, "")
			consumePod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			consumePod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, consumePod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeletePodAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), consumePodName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), consumePodName, delErr)
				}
			}()

			g.By("Waiting for consumption pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, consumePod)
			framework.ExpectNoError(err, "Consumption pod failed to start")

			g.By("Validating 90G bandwidth was consumed")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), consumeClaimName, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err)

			g.By("Creating overflow claim requesting 15G — exceeds remaining 10G on every NIC")
			overflowClaim := builder.BuildResourceClaimWithCapacity(overflowClaimName, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "15G",
				"egressBandwidth":  "15G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), overflowClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaimAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), overflowClaimName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), overflowClaimName, delErr)
				}
			}()

			g.By("Creating overflow pod pinned to same node — should be unschedulable")
			overflowPod := builder.BuildPodWithClaim(overflowPodName, overflowClaimName, "")
			overflowPod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			overflowPod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, overflowPod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeletePodAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), overflowPodName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), overflowPodName, delErr)
				}
			}()

			g.By("Verifying 15G overflow pod stays Pending with DRA-related Unschedulable condition")
			err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
				p, getErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, overflowPodName, metav1.GetOptions{})
				if getErr != nil {
					return false, getErr
				}
				if p.Status.Phase != corev1.PodPending {
					return false, fmt.Errorf("expected overflow pod to stay Pending but got %s", p.Status.Phase)
				}
				for _, cond := range p.Status.Conditions {
					if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
						msg := strings.ToLower(cond.Message)
						if strings.Contains(msg, "claim") || strings.Contains(msg, "insufficient") || strings.Contains(msg, "unresolvable") {
							framework.Logf("15G overflow pod correctly unschedulable: %s", cond.Message)
							return true, nil
						}
					}
				}
				framework.Logf("Overflow pod is Pending but no DRA-related Unschedulable condition yet")
				return false, nil
			})
			framework.ExpectNoError(err, "15G request should be unschedulable when only 10G remains per NIC")

			g.By("Creating fit claim requesting 10G — fits within remaining capacity")
			fitClaim := builder.BuildResourceClaimWithCapacity(fitClaimName, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "10G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), fitClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaimAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), fitClaimName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), fitClaimName, delErr)
				}
			}()

			g.By("Creating fit pod pinned to same node — should succeed")
			fitPod := builder.BuildPodWithClaim(fitPodName, fitClaimName, "")
			fitPod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			fitPod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, fitPod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeletePodAndWait(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), fitPodName, 1*time.Minute); delErr != nil {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), fitPodName, delErr)
				}
			}()

			g.By("Waiting for 10G fit pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, fitPod)
			framework.ExpectNoError(err, "10G request should succeed when 10G remains per NIC — fine-grained accounting may be broken")

			g.By("Validating ConsumedCapacity on the fit allocation")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), fitClaimName, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err, "ConsumedCapacity validation failed on 10G fit allocation")

			framework.Logf("Partial exhaustion validated: 15G rejected, 10G accepted with 10G remaining per NIC")
		})

	})
})
