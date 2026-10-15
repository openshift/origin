package nvidia

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

// setupGPUTest runs the checks every spec in this suite needs and returns the two helpers they
// all use. A cluster with no GPU nodes skips, while every other problem fails the spec, so a
// broken prerequisite is never reported as "this cluster has no hardware".
//
// area names the group of tests in the skip and failure messages.
func setupGPUTest(ctx context.Context, oc *exutil.CLI, area string) (*GPUValidator, *ResourceBuilder) {
	validator := NewGPUValidator(oc.KubeFramework(), oc)
	builder := NewResourceBuilder(oc.Namespace())

	// Look for GPU nodes before touching the installer so non-GPU clusters skip cleanly.
	nodes, err := validator.GetGPUNodes(ctx)
	if err != nil {
		g.Fail(fmt.Sprintf("Failed to discover GPU nodes: %v", err))
	}
	if len(nodes) == 0 {
		g.Skip(fmt.Sprintf("No GPU nodes available in the cluster - skipping %s tests", area))
	}
	framework.Logf("Found %d GPU node(s) available for %s tests", len(nodes), area)

	// Installs the DRA driver with Helm when it is absent. The GPU Operator must already exist.
	ensurePrerequisites(ctx, NewPrerequisitesInstaller(oc.KubeFramework()))

	if prerequisitesError != nil {
		g.Fail(fmt.Sprintf("Prerequisites validation failed: %v. Ensure the GPU Operator is installed on the cluster.", prerequisitesError))
	}
	if !prerequisitesInstalled {
		g.Fail(fmt.Sprintf("Prerequisites not installed - cannot run %s tests", area))
	}

	return validator, builder
}

// createGPUClaim creates a DeviceClass and a ResourceClaim for gpuCount GPUs, registering the
// cleanup for both.
func createGPUClaim(ctx context.Context, oc *exutil.CLI, builder *ResourceBuilder, deviceClassName, claimName string, gpuCount int) {
	g.By(fmt.Sprintf("Creating DeviceClass %s and ResourceClaim %s for %d GPU(s)", deviceClassName, claimName, gpuCount))

	createGPUDeviceClass(ctx, oc, builder, deviceClassName)
	registerGPUClaim(ctx, oc, claimName, builder.BuildResourceClaim(claimName, deviceClassName, gpuCount))
}

// createGPUClaimWithSelector is createGPUClaim with an extra CEL selector on the device request
// itself, so the claim can narrow the DeviceClass further.
func createGPUClaimWithSelector(ctx context.Context, oc *exutil.CLI, builder *ResourceBuilder, deviceClassName, claimName, celExpression string, gpuCount int) {
	g.By(fmt.Sprintf("Creating DeviceClass %s and ResourceClaim %s selecting on %s", deviceClassName, claimName, celExpression))

	createGPUDeviceClass(ctx, oc, builder, deviceClassName)
	registerGPUClaim(ctx, oc, claimName, builder.BuildResourceClaimWithCELSelector(claimName, deviceClassName, celExpression, gpuCount))
}

func createGPUDeviceClass(ctx context.Context, oc *exutil.CLI, builder *ResourceBuilder, deviceClassName string) {
	err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, builder.BuildDeviceClass(deviceClassName))
	framework.ExpectNoError(err, "Failed to create DeviceClass %s", deviceClassName)
	g.DeferCleanup(func(ctx context.Context) error {
		return deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)
	})
}

func registerGPUClaim(ctx context.Context, oc *exutil.CLI, claimName string, claim *resourceapi.ResourceClaim) {
	err := createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
	framework.ExpectNoError(err, "Failed to create ResourceClaim %s", claimName)
	g.DeferCleanup(func(ctx context.Context) error {
		return deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)
	})
}

// startGPUPod creates pod, registers the pod and claim cleanup, waits for the pod to run, and
// returns it with its running status populated.
//
// The creating client is a parameter because the security specs deliberately create their
// workloads as the project user, where SCC admission can only choose an SCC that user is allowed,
// rather than as admin, where it can choose any SCC in the cluster. description names the
// workload in the step and failure messages.
func startGPUPod(ctx context.Context, oc *exutil.CLI, client kubernetes.Interface, pod *corev1.Pod, claimName, description string) *corev1.Pod {
	podName := pod.Name

	g.By(fmt.Sprintf("Creating %s pod %s and waiting for it to run", description, podName))
	_, err := client.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
	framework.ExpectNoError(err, "Failed to create %s pod %s", description, podName)
	registerPodCleanup(oc.KubeFramework().ClientSet, oc.Namespace(), podName, claimName)

	err = e2epod.WaitForPodNameRunningInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace())
	framework.ExpectNoError(err, "%s pod %s failed to start", description, podName)

	running, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
	framework.ExpectNoError(err, "Failed to get running %s pod %s", description, podName)
	return running
}
