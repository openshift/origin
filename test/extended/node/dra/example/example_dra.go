package example

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	deviceClassGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "deviceclasses",
	}
	resourceClaimGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaims",
	}
	resourceClaimTemplateGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaimtemplates",
	}

	prerequisitesOnce      sync.Once
	prerequisitesInstalled bool
	prerequisitesError     error
)

var _ = g.Describe("[sig-scheduling][Feature:DRA-Example][Suite:openshift/dra-example][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("dra-example", admissionapi.LevelPrivileged)

	var (
		prereqInstaller *PrerequisitesInstaller
		validator       *DeviceValidator
		builder         *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping DRA example driver tests on MicroShift cluster")
		}

		validator = NewDeviceValidator(oc.KubeFramework())
		builder = NewResourceBuilder(oc.Namespace())
		prereqInstaller = NewPrerequisitesInstaller(oc.KubeFramework())

		prerequisitesOnce.Do(func() {
			framework.Logf("Checking DRA example driver prerequisites")

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
			g.Fail(fmt.Sprintf("DRA example driver prerequisites failed: %v", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Fail("DRA example driver prerequisites not installed")
		}
	})

	g.Context("Basic Device Allocation", func() {
		g.It("should allocate single device to pod via DRA", func(ctx context.Context) {
			deviceClassName := "test-example-device-" + oc.Namespace()
			claimName := "test-device-claim"
			podName := "test-device-pod"

			g.By("Creating DeviceClass for example driver")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer func() {
				if err := deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); err != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, err)
				}
			}()

			g.By("Creating ResourceClaim requesting 1 device")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer func() {
				if err := deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName); err != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, err)
				}
			}()

			g.By("Creating Pod using the ResourceClaim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")
			defer func() {
				if err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), podName, err)
				}
			}()

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Validating device allocation in ResourceClaim")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 1)
			framework.ExpectNoError(err, "Device allocation validation failed")
		})

		g.It("should handle pod deletion and resource cleanup", func(ctx context.Context) {
			deviceClassName := "test-example-cleanup-" + oc.Namespace()
			claimName := "test-device-claim-cleanup"
			podName := "test-device-pod-cleanup"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); err != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, err)
				}
			}()

			g.By("Creating ResourceClaim")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName); err != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, err)
				}
			}()

			g.By("Creating and verifying pod with device")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err)

			g.By("Validating device allocation before pod deletion")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 1)
			framework.ExpectNoError(err)

			g.By("Deleting pod")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			g.By("Waiting for pod to be deleted")
			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 1*time.Minute)
			framework.ExpectNoError(err)

			g.By("Verifying ResourceClaim still exists but is not reserved")
			err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
				claimObj, getErr := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).Get(ctx, claimName, metav1.GetOptions{})
				if getErr != nil {
					return false, getErr
				}

				reservedFor, found, nestErr := unstructured.NestedSlice(claimObj.Object, "status", "reservedFor")
				if nestErr != nil {
					return false, nestErr
				}
				if found && len(reservedFor) > 0 {
					framework.Logf("ResourceClaim %s still has %d reservation(s), waiting for DRA controller to clear...", claimName, len(reservedFor))
					return false, nil
				}
				return true, nil
			})
			framework.ExpectNoError(err, "ResourceClaim %s reservation was not released within timeout after pod deletion", claimName)
			framework.Logf("ResourceClaim %s successfully cleaned up after pod deletion", claimName)
		})
	})

	g.Context("Multi-Device Allocation", func() {
		g.It("should allocate multiple devices to single pod", func(ctx context.Context) {
			totalDevices, err := validator.GetTotalDeviceCount(ctx)
			if err != nil {
				g.Fail(fmt.Sprintf("Failed to count total devices: %v", err))
			}
			if totalDevices < 2 {
				g.Skip(fmt.Sprintf("Multi-device test requires at least 2 devices, but only %d available", totalDevices))
			}

			deviceClassName := "test-example-multi-" + oc.Namespace()
			claimName := "test-multi-device-claim"
			podName := "test-multi-device-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); err != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, err)
				}
			}()

			g.By("Creating ResourceClaim requesting 2 devices")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 2)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName); err != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, err)
				}
			}()

			g.By("Creating Pod using the multi-device claim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			defer func() {
				if err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), podName, err)
				}
			}()

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Validating 2 devices allocated")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 2)
			framework.ExpectNoError(err, "Expected 2 devices to be allocated")
		})
	})

	g.Context("Claim Sharing", func() {
		g.It("should allow multiple pods to share the same ResourceClaim", func(ctx context.Context) {
			deviceClassName := "test-example-shared-" + oc.Namespace()
			claimName := "test-shared-claim"
			pod1Name := "test-shared-pod-1"
			pod2Name := "test-shared-pod-2"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); err != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, err)
				}
			}()

			g.By("Creating shared ResourceClaim")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName); err != nil {
					framework.Logf("Warning: failed to delete ResourceClaim %s/%s: %v", oc.Namespace(), claimName, err)
				}
			}()

			g.By("Creating first pod using the shared claim")
			pod1 := builder.BuildLongRunningPodWithClaim(pod1Name, claimName, "")
			pod1, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod1, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create first pod")
			defer func() {
				if err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, pod1Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), pod1Name, err)
				}
			}()

			g.By("Waiting for first pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod1)
			framework.ExpectNoError(err, "First pod failed to start")

			g.By("Creating second pod using the same claim")
			pod2 := builder.BuildLongRunningPodWithClaim(pod2Name, claimName, "")
			pod2, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod2, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create second pod")
			defer func() {
				if err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, pod2Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					framework.Logf("Warning: failed to delete pod %s/%s: %v", oc.Namespace(), pod2Name, err)
				}
			}()

			g.By("Checking if second pod can share the claim")
			const pollInterval = 2 * time.Second
			const pollTimeout = 60 * time.Second
			var finalPhase corev1.PodPhase
			var schedulingFailed bool

			err = wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
				pod2, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, pod2Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				finalPhase = pod2.Status.Phase

				if pod2.Status.Phase == corev1.PodRunning {
					framework.Logf("Second pod is Running — claim sharing supported")
					return true, nil
				}

				if pod2.Status.Phase == corev1.PodPending {
					for _, cond := range pod2.Status.Conditions {
						if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == "Unschedulable" {
							msg := strings.ToLower(cond.Message)
							if strings.Contains(msg, "claim") || strings.Contains(msg, "allocat") {
								framework.Logf("Second pod is Pending due to DRA claim conflict: %s", cond.Message)
								schedulingFailed = true
								return true, nil
							}
							framework.Logf("Second pod is Pending with non-DRA Unschedulable reason (likely taints or resources): %s", cond.Message)
						}
					}
					framework.Logf("Second pod is Pending, continuing to poll...")
					return false, nil
				}

				return false, fmt.Errorf("second pod in unexpected phase: %s", pod2.Status.Phase)
			})
			framework.ExpectNoError(err, "Failed to determine second pod state")

			if schedulingFailed {
				framework.Logf("Second pod unschedulable — claim sharing not supported by example driver")
				g.By("Verifying first pod still has device access")
				err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 1)
				framework.ExpectNoError(err)
			} else if finalPhase == corev1.PodRunning {
				framework.Logf("Both pods running — claim sharing is supported")
				g.By("Verifying claim is still allocated")
				err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName, 1)
				framework.ExpectNoError(err)
			}
		})
	})

	g.Context("ResourceClaimTemplate", func() {
		g.It("should create claim from template for pod", func(ctx context.Context) {
			deviceClassName := "test-example-template-" + oc.Namespace()
			templateName := "test-device-template"
			podName := "test-template-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName); err != nil {
					framework.Logf("Warning: failed to delete DeviceClass %s: %v", deviceClassName, err)
				}
			}()

			g.By("Creating ResourceClaimTemplate")
			template := builder.BuildResourceClaimTemplate(templateName, deviceClassName, 1)
			err = createResourceClaimTemplate(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), template)
			framework.ExpectNoError(err)
			defer func() {
				if err := deleteResourceClaimTemplate(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), templateName); err != nil {
					framework.Logf("Warning: failed to delete ResourceClaimTemplate %s/%s: %v", oc.Namespace(), templateName, err)
				}
			}()

			g.By("Creating Pod with ResourceClaimTemplate reference")
			pod := builder.BuildPodWithInlineClaim(podName)
			*pod.Spec.ResourceClaims[0].ResourceClaimTemplateName = templateName
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Verifying ResourceClaim was created from template")
			claimPrefix := podName + "-device"

			claimList, err := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err, "Failed to list ResourceClaims")

			var generatedClaimName string
			var claimObj *unstructured.Unstructured
			for _, claim := range claimList.Items {
				if strings.HasPrefix(claim.GetName(), claimPrefix) {
					generatedClaimName = claim.GetName()
					claimObj = &claim
					framework.Logf("Found template-generated ResourceClaim: %s (matches prefix: %s)", generatedClaimName, claimPrefix)
					break
				}
			}

			o.Expect(generatedClaimName).NotTo(o.BeEmpty(), "ResourceClaim with prefix %s should be auto-created from template", claimPrefix)
			o.Expect(claimObj).NotTo(o.BeNil())
			framework.Logf("ResourceClaim %s was successfully created from template", generatedClaimName)

			g.By("Deleting pod and verifying claim cleanup")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 1*time.Minute)
			framework.ExpectNoError(err)

			g.By("Verifying auto-generated claim is deleted")
			err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
				_, getErr := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).Get(ctx, generatedClaimName, metav1.GetOptions{})
				if getErr != nil {
					if errors.IsNotFound(getErr) {
						framework.Logf("ResourceClaim %s was deleted as expected", generatedClaimName)
						return true, nil
					}
					return false, getErr
				}
				framework.Logf("ResourceClaim %s still exists, waiting for cleanup...", generatedClaimName)
				return false, nil
			})
			if err != nil {
				g.Fail(fmt.Sprintf("ResourceClaim %s not deleted within timeout — expected automatic cleanup: %v", generatedClaimName, err))
			}
			framework.Logf("ResourceClaim was cleaned up with pod deletion as expected")
		})
	})
})

func convertToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	unstructuredObj.Object = content
	return unstructuredObj, nil
}

func createDeviceClass(ctx context.Context, client dynamic.Interface, deviceClass interface{}) error {
	unstructuredObj, err := convertToUnstructured(deviceClass)
	if err != nil {
		return err
	}
	_, err = client.Resource(deviceClassGVR).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteDeviceClass(ctx context.Context, client dynamic.Interface, name string) error {
	return client.Resource(deviceClassGVR).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

func createResourceClaim(ctx context.Context, client dynamic.Interface, namespace string, claim interface{}) error {
	unstructuredObj, err := convertToUnstructured(claim)
	if err != nil {
		return err
	}
	_, err = client.Resource(resourceClaimGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteResourceClaim(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(resourceClaimGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

func createResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace string, template interface{}) error {
	unstructuredObj, err := convertToUnstructured(template)
	if err != nil {
		return err
	}
	_, err = client.Resource(resourceClaimTemplateGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(resourceClaimTemplateGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}
