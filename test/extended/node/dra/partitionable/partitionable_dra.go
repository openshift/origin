package partitionable

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

	dracommon "github.com/openshift/origin/test/extended/node/dra/common"
	draexample "github.com/openshift/origin/test/extended/node/dra/example"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	// These must match the Helm values passed in BeforeAll. The upstream
	// dra-example-driver defaults to numDevices=8; we override to 2 so
	// the counter-exhaustion test can deterministically consume all
	// partitions on a single node without requesting an excessive count.
	// Upstream chart values:
	//   https://github.com/kubernetes-sigs/dra-example-driver/blob/main/deployments/helm/dra-example-driver/values.yaml
	numGPUs          = 2
	partitionsPerGPU = 4

	totalPartitionsPerNode = numGPUs * partitionsPerGPU
)

var (
	prerequisitesOnce      sync.Once
	prerequisitesInstalled bool
	prerequisitesError     error
	// cachedInstaller retains the PrerequisitesInstaller created during the
	// first BeforeEach so that HelmUpgrade in AfterAll reuses its cloneDir
	// instead of re-cloning the upstream repo.
	cachedInstaller *draexample.PrerequisitesInstaller
)

var _ = g.Describe("[sig-scheduling][OCPFeatureGate:DRAPartitionableDevices][Feature:DRAPartitionableDevices][Suite:openshift/dra-example][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("dra-partitionable", admissionapi.LevelPrivileged)

	var (
		prereqInstaller  *draexample.PrerequisitesInstaller
		counterValidator *dracommon.CounterValidator
		builder          *dracommon.ResourceBuilder
		validator        *draexample.DeviceValidator
	)

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping DRA partitionable device tests on MicroShift cluster")
		}

		validator = draexample.NewDeviceValidator(oc.KubeFramework())
		counterValidator = dracommon.NewCounterValidator(oc.KubeFramework().ClientSet, "gpu.example.com")
		builder = dracommon.NewResourceBuilder(oc.Namespace(), dracommon.DriverConfig{
			DriverName:       "gpu.example.com",
			DefaultClass:     "gpu.example.com",
			RequestName:      "device",
			ContainerImage:   image.ShellImage(),
			ContainerCommand: []string{"sh", "-c", "echo DRA partition device allocated && sleep infinity"},
			LongRunCommand:   []string{"sh", "-c", "while true; do echo DRA partition active; sleep 60; done"},
		})
		if cachedInstaller == nil {
			cachedInstaller = draexample.NewPrerequisitesInstaller(oc.KubeFramework())
		}
		prereqInstaller = cachedInstaller

		prerequisitesOnce.Do(func() {
			framework.Logf("Checking DRA example driver prerequisites for partitionable device tests")

			if prereqInstaller.IsDriverInstalled(ctx) && validator.IsDriverPublishingDevices(ctx) {
				framework.Logf("DRA example driver already installed and publishing devices")
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

	g.Context("Partitionable Devices", g.Ordered, func() {
		g.BeforeAll(func(ctx context.Context) {
			framework.Logf("Upgrading DRA example driver with numDevices=%d, gpuPartitions=%d", numGPUs, partitionsPerGPU)
			err := prereqInstaller.HelmUpgrade(ctx,
				fmt.Sprintf("kubeletPlugin.numDevices=%d", numGPUs),
				fmt.Sprintf("kubeletPlugin.gpuPartitions=%d", partitionsPerGPU),
			)
			framework.ExpectNoError(err, "Failed to helm upgrade driver with partitioning enabled")

			framework.Logf("Waiting for driver to stabilize after upgrade...")
			err = prereqInstaller.WaitForDriver(ctx, 5*time.Minute)
			framework.ExpectNoError(err, "Driver failed to stabilize after helm upgrade")

			err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
				return counterValidator.HasSharedCounters(ctx), nil
			})
			if err != nil {
				g.Skip("DRAPartitionableDevices feature gate appears disabled — no SharedCounters published after upgrade")
			}

			framework.Logf("SharedCounters detected — DRAPartitionableDevices feature gate is active")
		})

		g.AfterAll(func(ctx context.Context) {
			framework.Logf("Restoring DRA example driver to default mode (no partitions)")
			err := prereqInstaller.HelmUpgrade(ctx,
				"kubeletPlugin.numDevices=8",
				"kubeletPlugin.gpuPartitions=0",
			)
			if err != nil {
				framework.Logf("Warning: failed to restore driver to default mode: %v", err)
				return
			}
			if waitErr := prereqInstaller.WaitForDriver(ctx, 5*time.Minute); waitErr != nil {
				framework.Logf("Warning: driver did not stabilize after restore: %v", waitErr)
				return
			}
			err = wait.PollUntilContextTimeout(ctx, 3*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
				return !counterValidator.HasSharedCounters(ctx), nil
			})
			if err != nil {
				framework.Logf("Warning: SharedCounters still present after restoring non-partitioned mode")
			}
		})

		g.It("should publish ResourceSlices with SharedCounters and ConsumesCounters", func(ctx context.Context) {
			g.By("Validating SharedCounters in counter slices")
			err := counterValidator.ValidateSharedCounters(ctx, []string{"memory", "compute"})
			framework.ExpectNoError(err, "SharedCounters validation failed")

			g.By("Validating Devices have ConsumesCounters in device slices")
			err = counterValidator.ValidateDeviceConsumesCounters(ctx)
			framework.ExpectNoError(err, "ConsumesCounters validation failed")

			g.By("Verifying partition devices exist")
			partitionCount, err := counterValidator.CountPartitionDevices(ctx)
			framework.ExpectNoError(err, "Failed to count partition devices")
			o.Expect(partitionCount).To(o.BeNumerically(">", 0),
				"Expected partition devices after enabling gpuPartitions=%d", partitionsPerGPU)
			framework.Logf("Found %d partition device(s) across all nodes", partitionCount)
		})

		g.It("should allocate partition device to pod via DRA", func(ctx context.Context) {
			deviceClassName := "test-partitionable-" + oc.Namespace()
			claimName := "test-partition-claim"
			podName := "test-partition-pod"

			g.By("Creating DeviceClass for partitionable driver")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := dracommon.CreateDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			g.By("Creating ResourceClaim requesting 2 partition devices")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 2)
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, delErr)
				}
			}()

			g.By("Creating Pod using the partition claim")
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
			framework.ExpectNoError(err, "Pod with partition devices failed to start")

			g.By("Validating device allocation shows 2 devices")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 2)
			framework.ExpectNoError(err, "Device allocation validation failed")

			g.By("Verifying allocated devices are partitions (not full GPUs)")
			allocatedClaim, err := oc.KubeFramework().ClientSet.ResourceV1().ResourceClaims(oc.Namespace()).Get(ctx, claimName, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to get ResourceClaim %s for partition verification", claimName)
			for _, result := range allocatedClaim.Status.Allocation.Devices.Results {
				o.Expect(result.Device).To(o.ContainSubstring("partition"),
					"Expected partition device but got %q", result.Device)
				framework.Logf("Allocated partition device: pool=%s, device=%s", result.Pool, result.Device)
			}
		})

		g.It("should mark pod unschedulable when all counters are exhausted on a node", func(ctx context.Context) {
			g.By("Finding a node where the driver publishes devices")
			nodeName, err := counterValidator.GetNodeWithDevices(ctx)
			framework.ExpectNoError(err, "Failed to find a node with published devices")
			framework.Logf("Using node %s for counter exhaustion test", nodeName)

			deviceClassName := "test-exhaustion-" + oc.Namespace()
			exhaustClaimName := "test-exhaust-claim"
			overflowClaimName := "test-overflow-claim"
			exhaustPodName := "test-exhaust-pod"
			overflowPodName := "test-overflow-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = dracommon.CreateDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); delErr != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, delErr)
				}
			}()

			// With numGPUs=2 and partitionsPerGPU=4, each node has 8 partition
			// devices plus 2 full-GPU devices. Requesting all 8 partitions
			// exhausts every GPU's shared counters, leaving no capacity for
			// additional allocations on this node.
			//
			// NodeSelector on the pod is sufficient to constrain device
			// allocation: the DRA scheduler evaluates resource claims in
			// the context of the pod's scheduling constraints, so it will
			// only allocate devices from ResourceSlices on the pinned node.
			g.By(fmt.Sprintf("Creating ResourceClaim requesting %d devices to exhaust all counters on node %s", totalPartitionsPerNode, nodeName))
			exhaustClaim := builder.BuildResourceClaim(exhaustClaimName, deviceClassName, totalPartitionsPerNode)
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), exhaustClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), exhaustClaimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), exhaustClaimName, delErr)
				}
			}()

			g.By("Creating pod pinned to target node to consume all partitions")
			exhaustPod := builder.BuildLongRunningPodWithClaim(exhaustPodName, exhaustClaimName, "")
			exhaustPod.Spec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": nodeName,
			}
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
			framework.ExpectNoError(err, "Exhaustion pod failed to start — scheduler may not support allocating %d partition devices", totalPartitionsPerNode)

			g.By("Validating all partitions were allocated")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), exhaustClaimName, totalPartitionsPerNode)
			framework.ExpectNoError(err)

			g.By("Creating overflow claim requesting 1 more device")
			overflowClaim := builder.BuildResourceClaim(overflowClaimName, deviceClassName, 1)
			err = dracommon.CreateResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), overflowClaim)
			framework.ExpectNoError(err)
			defer func() {
				if delErr := dracommon.DeleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), overflowClaimName); delErr != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), overflowClaimName, delErr)
				}
			}()

			g.By("Creating overflow pod pinned to same node — should be unschedulable")
			overflowPod := builder.BuildPodWithClaim(overflowPodName, overflowClaimName, "")
			overflowPod.Spec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": nodeName,
			}
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
			framework.ExpectNoError(err, "Overflow pod should be unschedulable when all counters are exhausted")
		})
	})
})
