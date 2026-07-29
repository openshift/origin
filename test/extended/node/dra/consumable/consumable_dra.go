package consumable

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		nodeutils.SkipOnMicroShift(oc)

		capacityValidator = dracommon.NewCapacityValidator(oc.KubeFramework().ClientSet, netDriverName)
		builder = dracommon.NewResourceBuilder(oc.Namespace(), dracommon.DriverConfig{
			DriverName:       netDriverName,
			DefaultClass:     netDeviceClass,
			RequestName:      "nic",
			ContainerImage:   image.ShellImage(),
			ContainerCommand: []string{"sh", "-c", "echo DRA NIC device allocated && sleep infinity"},
			LongRunCommand:   []string{"sh", "-c", "while true; do echo DRA NIC active; sleep 60; done"},
		})
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
			if strings.Contains(prerequisitesError.Error(), "not found or failed") {
				g.Skip(fmt.Sprintf("Required tooling unavailable: %v", prerequisitesError))
			}
			g.Fail(fmt.Sprintf("DRA example driver prerequisites failed: %v", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Fail("DRA example driver prerequisites not installed")
		}
	})

	g.Context("Consumable Capacity", g.Ordered, func() {
		g.BeforeAll(func(ctx context.Context) {
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
				return capacityValidator.HasCapacityDevices(ctx), nil
			})
			if err != nil {
				g.Skip("DRAConsumableCapacity not available — no devices with Capacity published after upgrade")
			}

			framework.Logf("Capacity devices detected — DRAConsumableCapacity is active")
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

		g.It("should allocate NIC with bandwidth capacity to pod", func(ctx context.Context) {
			deviceClassName := "test-consumable-basic-" + oc.Namespace()
			claimName := "test-nic-claim"
			podName := "test-nic-pod"

			g.By("Creating DeviceClass for net driver")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := dracommon.CreateDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			g.By("Creating ResourceClaim requesting 10G ingress / 5G egress")
			claim := builder.BuildResourceClaimWithCapacity(claimName, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "5G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, delErr)
				}
			}()

			g.By("Creating Pod using the NIC claim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")
			defer func() {
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), podName, delErr)
				}
			}()

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod with NIC bandwidth claim failed to start")

			g.By("Validating ConsumedCapacity in allocation results")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), claimName, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err, "ConsumedCapacity validation failed")
		})

		g.It("should allow multiple allocations on the same NIC device", func(ctx context.Context) {
			g.By("Finding a node where the driver publishes devices")
			nodeName, err := capacityValidator.GetNodeWithDevices(ctx)
			framework.ExpectNoError(err, "Failed to find a node with published devices")
			framework.Logf("Using node %s for multi-allocation test", nodeName)

			deviceClassName := "test-consumable-multi-" + oc.Namespace()
			claim1Name := "test-multi-nic-claim-1"
			claim2Name := "test-multi-nic-claim-2"
			pod1Name := "test-multi-nic-pod-1"
			pod2Name := "test-multi-nic-pod-2"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = dracommon.CreateDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			// Each NIC has 100G bandwidth. Requesting 10G from each claim leaves
			// plenty of room for both to be allocated on the same device.
			g.By("Creating first ResourceClaim requesting 10G ingress / 10G egress")
			claim1 := builder.BuildResourceClaimWithCapacity(claim1Name, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "10G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claim1)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claim1Name); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claim1Name, delErr)
				}
			}()

			g.By("Creating first pod pinned to target node")
			pod1 := builder.BuildLongRunningPodWithClaim(pod1Name, claim1Name, "")
			pod1.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			pod1, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod1, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				gracePeriod := int64(10)
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, pod1Name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), pod1Name, delErr)
				}
			}()

			g.By("Waiting for first pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod1)
			framework.ExpectNoError(err, "First pod failed to start")

			g.By("Creating second ResourceClaim requesting 10G ingress / 10G egress")
			claim2 := builder.BuildResourceClaimWithCapacity(claim2Name, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "10G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claim2)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), claim2Name); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claim2Name, delErr)
				}
			}()

			g.By("Creating second pod pinned to same node")
			pod2 := builder.BuildLongRunningPodWithClaim(pod2Name, claim2Name, "")
			pod2.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			pod2, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod2, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				gracePeriod := int64(10)
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, pod2Name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), pod2Name, delErr)
				}
			}()

			g.By("Waiting for second pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod2)
			framework.ExpectNoError(err, "Second pod failed to start — AllowMultipleAllocations may not be working")

			g.By("Verifying both claims have ConsumedCapacity")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), claim1Name, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err)
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), claim2Name, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err)

			framework.Logf("Both pods running on same node with separate bandwidth allocations — consumable capacity sharing works")
		})

		g.It("should mark pod unschedulable when all NIC bandwidth is exhausted on a node", func(ctx context.Context) {
			g.By("Finding a node where the driver publishes devices")
			nodeName, err := capacityValidator.GetNodeWithDevices(ctx)
			framework.ExpectNoError(err, "Failed to find a node with published devices")
			framework.Logf("Using node %s for capacity exhaustion test", nodeName)

			deviceClassName := "test-consumable-exhaust-" + oc.Namespace()
			exhaustClaimName := "test-exhaust-bw-claim"
			overflowClaimName := "test-overflow-bw-claim"
			exhaustPodName := "test-exhaust-bw-pod"
			overflowPodName := "test-overflow-bw-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = dracommon.CreateDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			// With numNICs=2, each NIC has 100G bandwidth. Request all 2 NICs
			// with 100G each to exhaust all bandwidth capacity on the node.
			g.By(fmt.Sprintf("Creating ResourceClaim requesting %d NICs with 100G bandwidth each to exhaust all capacity", numNICs))
			exhaustClaim := builder.BuildResourceClaimWithCapacity(exhaustClaimName, deviceClassName, numNICs, map[string]string{
				"ingressBandwidth": "100G",
				"egressBandwidth":  "100G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), exhaustClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), exhaustClaimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), exhaustClaimName, delErr)
				}
			}()

			g.By("Creating pod pinned to target node to consume all bandwidth")
			exhaustPod := builder.BuildLongRunningPodWithClaim(exhaustPodName, exhaustClaimName, "")
			exhaustPod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			exhaustPod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, exhaustPod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				gracePeriod := int64(10)
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, exhaustPodName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), exhaustPodName, delErr)
				}
			}()

			g.By("Waiting for exhaustion pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, exhaustPod)
			framework.ExpectNoError(err, "Exhaustion pod failed to start")

			g.By("Validating bandwidth was consumed")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), exhaustClaimName, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err)

			g.By("Creating overflow claim requesting more bandwidth")
			overflowClaim := builder.BuildResourceClaimWithCapacity(overflowClaimName, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "10G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), overflowClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), overflowClaimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), overflowClaimName, delErr)
				}
			}()

			g.By("Creating overflow pod pinned to same node — should be unschedulable")
			overflowPod := builder.BuildPodWithClaim(overflowPodName, overflowClaimName, "")
			overflowPod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			overflowPod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, overflowPod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				gracePeriod := int64(10)
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, overflowPodName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), overflowPodName, delErr)
				}
			}()

			g.By("Verifying overflow pod stays Pending with Unschedulable condition")
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
							framework.Logf("Overflow pod correctly unschedulable: %s", cond.Message)
							return true, nil
						}
					}
				}
				framework.Logf("Overflow pod is Pending but no DRA-related Unschedulable condition yet")
				return false, nil
			})
			framework.ExpectNoError(err, "Overflow pod should be unschedulable when all NIC bandwidth is exhausted")
		})

		g.It("should release capacity when pod is deleted and allow new allocation", func(ctx context.Context) {
			g.By("Finding a node where the driver publishes devices")
			nodeName, err := capacityValidator.GetNodeWithDevices(ctx)
			framework.ExpectNoError(err, "Failed to find a node with published devices")
			framework.Logf("Using node %s for capacity release test", nodeName)

			deviceClassName := "test-consumable-release-" + oc.Namespace()
			consumeClaimName := "test-consume-bw-claim"
			reuseClaimName := "test-reuse-bw-claim"
			consumePodName := "test-consume-bw-pod"
			reusePodName := "test-reuse-bw-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = dracommon.CreateDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().ClientSet, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			// Consume all bandwidth on all NICs so the node is fully exhausted.
			g.By(fmt.Sprintf("Creating ResourceClaim requesting %d NICs with 100G bandwidth to exhaust capacity", numNICs))
			consumeClaim := builder.BuildResourceClaimWithCapacity(consumeClaimName, deviceClassName, numNICs, map[string]string{
				"ingressBandwidth": "100G",
				"egressBandwidth":  "100G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), consumeClaim)
			framework.ExpectNoError(err)

			g.By("Creating pod to consume all bandwidth")
			consumePod := builder.BuildLongRunningPodWithClaim(consumePodName, consumeClaimName, "")
			consumePod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			consumePod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, consumePod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			g.By("Waiting for consumption pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, consumePod)
			framework.ExpectNoError(err, "Consumption pod failed to start")

			g.By("Deleting consumption pod and claim to release capacity")
			gracePeriod := int64(10)
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, consumePodName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, consumePodName, oc.Namespace(), 1*time.Minute)
			framework.ExpectNoError(err)

			err = dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), consumeClaimName)
			framework.ExpectNoError(err, "Failed to delete consumption claim")

			g.By("Creating new claim to verify capacity was released")
			reuseClaim := builder.BuildResourceClaimWithCapacity(reuseClaimName, deviceClassName, 1, map[string]string{
				"ingressBandwidth": "10G",
				"egressBandwidth":  "10G",
			})
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), reuseClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().ClientSet, oc.Namespace(), reuseClaimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), reuseClaimName, delErr)
				}
			}()

			g.By("Creating pod to use released capacity")
			reusePod := builder.BuildPodWithClaim(reusePodName, reuseClaimName, "")
			reusePod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			reusePod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, reusePod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				if delErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, reusePodName, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); delErr != nil && !errors.IsNotFound(delErr) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), reusePodName, delErr)
				}
			}()

			g.By("Waiting for reuse pod to be running — capacity should be available again")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, reusePod)
			framework.ExpectNoError(err, "Reuse pod failed to start — capacity may not have been released after pod deletion")

			g.By("Validating ConsumedCapacity on the new allocation")
			err = capacityValidator.ValidateConsumedCapacity(ctx, oc.Namespace(), reuseClaimName, []string{"ingressBandwidth", "egressBandwidth"})
			framework.ExpectNoError(err, "ConsumedCapacity validation failed on reused capacity")

			framework.Logf("Capacity successfully released and reused after pod deletion")
		})
	})
})
