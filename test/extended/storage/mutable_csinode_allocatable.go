package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	exutil "github.com/openshift/origin/test/extended/util"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

const (
	ebsCSIDriverName                      = "ebs.csi.aws.com"
	csiNodeUpdatePollTimeout              = 5 * time.Minute
	csiNodeUpdatePollInterval             = 10 * time.Second
	csiNodeUpdatePeriodSecondsShort int64 = 10
	csiNodeUpdatePeriodSecondsLong  int64 = 3600 // 1 hour - nearly never update automatically
	eniWaitTimeout                        = 2 * time.Minute
	eniPollInterval                       = 5 * time.Second
	resourceExhaustionTimeout             = 15 * time.Minute
)

// This is [Serial] because it scales down operators and attaches/detaches ENIs to worker nodes.
var _ = g.Describe(`[sig-storage][FeatureGate:MutableCSINodeAllocatableCount][Jira:"Storage"][Serial][Driver: ebs.csi.aws.com]`, func() {
	defer g.GinkgoRecover()
	var (
		ctx                                 = context.Background()
		oc                                  = exutil.NewCLI("mutable-csinode-allocatable-test")
		originalCSIDriverUpdatePeriod       *int64
		targetWorkerNode                    string
		ec2Client                           *ec2.EC2
		originalAllocatableCount            int32
		originalCVOReplicas                 *int32
		targetWorkerNodeAttachedVolumeCount int32
	)

	g.BeforeEach(func() {
		// TODO: remove the check when MutableCSINodeAllocatableCount is supported for GA
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}

		// Skip if not AWS platform
		if !e2e.ProviderIs("aws") {
			g.Skip("skipping, this test is only expected to work with AWS clusters")
		}

		// TODO: After https://issues.redhat.com/browse/OCPBUGS-65972 is resolved, remove the check,
		// we can use the change the clustercsidriver Unmanaged way which will be also suitable for hosted clusters.
		// Skip if on hosted clusters
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("skipping, this test is only expected to work with standalone clusters.")
		}

		// Check to see if we have Storage enabled
		isStorageEnabled, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityStorage)
		if err != nil || !isStorageEnabled {
			g.Skip("skipping, this test is only expected to work with storage enabled clusters")
		}

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MutableCSINodeAllocatableCount tests are not supported on MicroShift")
		}
		// TODO: After https://issues.redhat.com/browse/OCPBUGS-65972 is resolved,
		// we can use the change the clustercsidriver Unmanaged way which will be even better.
		g.By("Scaling down cluster-version-operator, cluster-storage-operator, and aws-ebs-csi-driver-operator")
		// Scale down cluster-version-operator
		cvoDeployment, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-cluster-version").Get(ctx, "cluster-version-operator", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-version-operator deployment")
		originalCVOReplicas = cvoDeployment.Spec.Replicas

		zero := int32(0)
		_, err = e2edeployment.UpdateDeploymentWithRetries(oc.AdminKubeClient(), "openshift-cluster-version", "cluster-version-operator", func(d *appsv1.Deployment) {
			d.Spec.Replicas = &zero
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to scale down cluster-version-operator")
		e2e.Logf("Successfully scaled down cluster-version-operator")

		g.DeferCleanup(func(ctx context.Context, replicas *int32) {
			g.By("Restoring cluster-version-operator replica counts")
			if replicas != nil {
				e2e.Logf("Restoring cluster-version-operator to %d replicas", *replicas)
				_, err := e2edeployment.UpdateDeploymentWithRetries(oc.AdminKubeClient(), "openshift-cluster-version", "cluster-version-operator", func(d *appsv1.Deployment) {
					d.Spec.Replicas = replicas
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore cluster-version-operator")
				e2e.Logf("Successfully restored cluster-version-operator")
			}

			g.By("Waiting for aws-ebs-csi-driver-operator to be reconciled and ready")
			o.Eventually(func() error {
				ebsCSIDriverOperator, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-cluster-csi-drivers").Get(ctx, "aws-ebs-csi-driver-operator", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if ebsCSIDriverOperator.Spec.Replicas == nil || *ebsCSIDriverOperator.Spec.Replicas == 0 {
					return fmt.Errorf("still waiting for CVO reconciles the ebs csi driver operator")
				}
				return e2edeployment.WaitForDeploymentComplete(oc.AdminKubeClient(), ebsCSIDriverOperator)
			}).WithTimeout(csiNodeUpdatePollTimeout).WithPolling(csiNodeUpdatePollInterval).Should(o.Succeed(), "aws-ebs-csi-driver-operator should be available")
			e2e.Logf("aws-ebs-csi-driver-operator is ready")

			g.By("Waiting for cluster storage operator to be available")
			WaitForCSOHealthy(oc)
		}, ctx, originalCVOReplicas)

		// Scale down cluster-storage-operator
		_, err = e2edeployment.UpdateDeploymentWithRetries(oc.AdminKubeClient(), "openshift-cluster-storage-operator", "cluster-storage-operator", func(d *appsv1.Deployment) {
			d.Spec.Replicas = &zero
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to scale down cluster-storage-operator")
		e2e.Logf("Successfully scaled down cluster-storage-operator")

		// Scale down aws-ebs-csi-driver-operator
		_, err = e2edeployment.UpdateDeploymentWithRetries(oc.AdminKubeClient(), "openshift-cluster-csi-drivers", "aws-ebs-csi-driver-operator", func(d *appsv1.Deployment) {
			d.Spec.Replicas = &zero
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to scale down aws-ebs-csi-driver-operator")
		e2e.Logf("Successfully scaled down aws-ebs-csi-driver-operator")

		// Store original CSIDriver configuration
		originalCSIDriverUpdatePeriod, err = getCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Stored original CSIDriver %s configuration", ebsCSIDriverName)

		g.By("# Get a schedulable worker node")
		workerNodes, err := GetSchedulableLinuxWorkerNodes(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0), "no schedulable linux worker nodes found")
		targetWorkerNode = workerNodes[0].Name
		e2e.Logf("Selected worker node: %s", targetWorkerNode)

		g.By("# Get the schedulable worker node attached volume count from VolumeAttachments")
		targetWorkerNodeAttachedVolumeCount = getAttachedVolumeCountFromVolumeAttachments(ctx, oc, targetWorkerNode)

		g.By("# Get the original CSINode allocatable count for worker node")
		originalAllocatableCount = getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
		o.Expect(originalAllocatableCount).To(o.BeNumerically(">", 0), "original allocatable count should be greater than 0")
		e2e.Logf("Original CSINode allocatable count for node %s: %d", targetWorkerNode, originalAllocatableCount)

		// Get AWS credentials
		exutil.GetAwsCredentialFromCluster(oc)

		// Create AWS EC2 client
		mySession := exutil.InitAwsSession(os.Getenv("AWS_REGION"))
		ec2Client = ec2.New(mySession, aws.NewConfig())
	})

	g.It("should be set the nodeAllocatableUpdatePeriodSeconds correctly by driver operator", func() {
		g.By("# Checking the CSI driver operator correctly sets nodeAllocatableUpdatePeriodSeconds")
		o.Expect(originalCSIDriverUpdatePeriod).ShouldNot(o.BeNil(), "originalCSIDriverUpdatePeriod should not be nil")
		o.Expect(*originalCSIDriverUpdatePeriod).Should(o.BeNumerically(">", int64(0)), "originalCSIDriverUpdatePeriod should be greater than 0")

		g.By(fmt.Sprintf("# Setting CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", csiNodeUpdatePeriodSecondsShort))
		updateCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName, csiNodeUpdatePeriodSecondsShort)
		g.DeferCleanup(restoreCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, originalCSIDriverUpdatePeriod)

		g.By("# Checking the nodeAllocatableUpdatePeriodSeconds should not reconcile back since operator is scaled down")
		currentNodeAllocatableUpdatePeriodSeconds, err := getCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*currentNodeAllocatableUpdatePeriodSeconds).To(o.Equal(csiNodeUpdatePeriodSecondsShort))

	})

	g.It("should automatically update CSINode allocatable count when instance attached ENI count changes", func() {
		g.By(fmt.Sprintf("# Setting CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", csiNodeUpdatePeriodSecondsShort))
		updateCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName, csiNodeUpdatePeriodSecondsShort)
		g.DeferCleanup(restoreCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, originalCSIDriverUpdatePeriod)

		g.By("# Attaching 2 ENIs to the worker node")
		// Get instance ID from node
		instanceID := getAWSInstanceIDFromNode(ctx, oc, targetWorkerNode)
		e2e.Logf("Instance ID for node %s: %s", targetWorkerNode, instanceID)

		testENIs := make(map[string]bool)
		g.DeferCleanup(cleanupTestENIsAndRecoverCSINode, ctx, ec2Client, oc, &testENIs, ebsCSIDriverName, targetWorkerNode, originalAllocatableCount)

		var firstENI string
		for i := 0; i < 2; i++ {
			eniID := createAndAttachENI(ec2Client, instanceID)
			testENIs[eniID] = true
			e2e.Logf("Attached ENI: %s", eniID)
			if i == 0 {
				firstENI = eniID
			}
		}

		g.By(fmt.Sprintf("# Waiting for CSINode allocatable count to update to %d", originalAllocatableCount-2))
		expectedCount := originalAllocatableCount - 2
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should update to original - 2")
		e2e.Logf("Successfully verified CSINode allocatable count updated to %d", expectedCount)

		g.By(fmt.Sprintf("# Detaching 1 ENI from node and waiting for CSINode allocatable count update to %d", originalAllocatableCount-1))
		detachENI(ec2Client, firstENI)
		deleteENI(ec2Client, firstENI)
		delete(testENIs, firstENI)
		testENIs[firstENI] = false
		e2e.Logf("Detached ENI: %s", firstENI)

		expectedCount = originalAllocatableCount - 1
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should update to original - 1")
		e2e.Logf("Successfully verified CSINode allocatable count updated to %d", expectedCount)
	})

	g.It("should immediately update CSINode allocatable count when ResourceExhausted errors occur", func() {

		g.By(fmt.Sprintf("# Setting CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", csiNodeUpdatePeriodSecondsLong))
		updateCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName, csiNodeUpdatePeriodSecondsLong)
		g.DeferCleanup(restoreCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, originalCSIDriverUpdatePeriod)

		g.By("# Get the original CSINode allocatable count for worker node")
		originalAllocatableCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
		o.Expect(originalAllocatableCount).To(o.BeNumerically(">", 0), "original allocatable count should be greater than 0")
		e2e.Logf("Original CSINode allocatable count for node %s: %d", targetWorkerNode, originalAllocatableCount)

		g.By("# Attaching 1 ENI to the worker node")
		// Get instance ID from node
		instanceID := getAWSInstanceIDFromNode(ctx, oc, targetWorkerNode)
		e2e.Logf("Instance ID for node %s: %s", targetWorkerNode, instanceID)

		testENIs := make(map[string]bool)
		g.DeferCleanup(cleanupTestENIsAndRecoverCSINode, ctx, ec2Client, oc, &testENIs, ebsCSIDriverName, targetWorkerNode, originalAllocatableCount)
		eniID := createAndAttachENI(ec2Client, instanceID)
		testENIs[eniID] = true
		e2e.Logf("Attached ENI: %q", eniID)

		g.By("# Checking CSINode allocatable count should not update")
		o.Consistently(func() int32 {
			return getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
		}, 6*csiNodeUpdatePollInterval, csiNodeUpdatePollInterval).Should(o.Equal(originalAllocatableCount),
			"CSINode allocatable count should not update less than nodeAllocatableUpdatePeriodSeconds")

		g.By("# Checking CSINode allocatable count should not update")
		storageClassName := GetCSIStorageClassByProvisioner(ctx, oc, ebsCSIDriverName)
		e2e.Logf("Using StorageClass: %s", storageClassName)

		// The StatefulSet will have replicas equal to the original allocatable count
		// This should trigger ResourceExhausted errors for the last pod
		replicas := originalAllocatableCount - targetWorkerNodeAttachedVolumeCount
		statefulSetName := "storage-mca-test-sts"

		g.By(fmt.Sprintf("# Creating StatefulSet %s with %d replicas scheduled to node %s", statefulSetName, replicas, targetWorkerNode))
		statefulSet := createStatefulSetWithVolumeTemplate(oc.Namespace(), statefulSetName, replicas, targetWorkerNode, storageClassName)
		_, err := oc.AdminKubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(ctx, statefulSet, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create StatefulSet")
		g.DeferCleanup(cleanupStatefulSetWithPVCs, ctx, oc, oc.Namespace(), statefulSetName, replicas)

		// Observe that the last replica pod transitions from ContainerCreating to Pending
		// and that the ResourceExhausted error triggers an immediate CSINode allocatable count update
		lastPodName := fmt.Sprintf("%s-%d", statefulSetName, replicas-1)
		g.By(fmt.Sprintf("# Monitoring pod %s for ResourceExhausted error and CSINode update", lastPodName))
		// Wait for the last pod to show ResourceExhausted error
		o.Eventually(func() bool {
			pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, lastPodName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Pod %s not found yet: %v", lastPodName, err)
				return false
			}

			// Check pod events for ResourceExhausted error
			events, err := oc.AdminKubeClient().CoreV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", lastPodName),
			})
			if err == nil {
				for _, event := range events.Items {
					if strings.Contains(event.Message, "ResourceExhausted") ||
						strings.Contains(event.Message, "volume attach limit exceeded") ||
						strings.Contains(event.Reason, "FailedAttachVolume") {
						e2e.Logf("ResourceExhausted error detected for pod %s: %s - %s", lastPodName, event.Reason, event.Message)
						return true
					}
				}
			}

			// Also check container status messages
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil {
					message := containerStatus.State.Waiting.Message
					if strings.Contains(message, "ResourceExhausted") || strings.Contains(message, "attach limit") {
						e2e.Logf("ResourceExhausted detected in container status: %s", message)
						return true
					}
				}
			}

			e2e.Logf("Pod %s status: %s, waiting for ResourceExhausted error", lastPodName, pod.Status.Phase)
			return false
		}, resourceExhaustionTimeout, csiNodeUpdatePollInterval).Should(o.BeTrue(),
			"ResourceExhausted error should be detected")

		// Verify that CSINode allocatable count was updated immediately (not waiting for the long update period)
		// The count should remain at original-1 because we still have 1 ENI attached
		g.By(fmt.Sprintf("# Verifying that CSINode allocatable count updated to %d (immediate update triggered by ResourceExhausted)", originalAllocatableCount-1))
		expectedCount := originalAllocatableCount - 1
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should be immediately updated despite long update period")
		e2e.Logf("Successfully verified that ResourceExhausted error triggered immediate CSINode update")

		g.By("# Verifying the last pod should FailedScheduling after CSINode allocatable count updated")
		o.Eventually(func() bool {
			pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, lastPodName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Pod %s not found yet: %v", lastPodName, err)
				return false
			}

			if pod.Status.Phase != v1.PodPending {
				e2e.Logf("Pod %s not Pending yet, current phase: %s", lastPodName, pod.Status.Phase)
				return false
			}

			events, err := oc.AdminKubeClient().CoreV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", lastPodName),
			})
			if err != nil {
				return false
			}

			for _, e := range events.Items {
				if e.Reason == "FailedScheduling" && strings.Contains(e.Message, "exceed max volume count") {
					e2e.Logf("Pod %s is Pending due to volume attach limit as expected", lastPodName)
					return true
				}
			}

			return false
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.BeTrue(),
			"Pod should be Pending due to volume attach limit")
	})

})

// createStatefulSetWithVolumeTemplate creates a StatefulSet with volumeClaimTemplates
func createStatefulSetWithVolumeTemplate(namespace, name string, replicas int32, nodeName, storageClassName string) *appsv1.StatefulSet {
	allowPrivEsc := false
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
					Containers: []v1.Container{
						{
							Name:  "test-container",
							Image: k8simage.GetE2EImage(k8simage.BusyBox),
							Command: []string{
								"sh",
								"-c",
								"while true; do sleep 3600; done",
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/mnt/data",
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivEsc,
								SeccompProfile: &v1.SeccompProfile{
									Type: v1.SeccompProfileTypeRuntimeDefault,
								},
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						StorageClassName: &storageClassName,
					},
				},
			},
		},
	}
}

// cleanupStatefulSetWithPVCs deletes a StatefulSet and its associated PVCs.
// Useful for tests using a StatefulSet with PVC retention policy "WhenDeleted: Delete".
func cleanupStatefulSetWithPVCs(ctx context.Context, oc *exutil.CLI, namespace string, name string, replicas int32) {
	e2e.Logf("Cleaning up StatefulSet %s", name)
	err := oc.AdminKubeClient().AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to cleanup StatefulSet")

	// Delete PVCs created by StatefulSet
	for i := int32(0); i < replicas; i++ {
		pvcName := fmt.Sprintf("data-%s-%d", name, i)
		// Ignore errors during PVC deletion since we already used StatefulSet PersistentVolumeClaimRetentionPolicy WhenDeleted: delete
		_ = oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	}
}

// updateCSIDriverUpdatePeriod updates the CSIDriver's nodeAllocatableUpdatePeriodSeconds
func updateCSIDriverUpdatePeriod(ctx context.Context, oc *exutil.CLI, name string, expectedCSINodeUpdatePeriodSeconds int64) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		d, err := oc.AdminKubeClient().StorageV1().CSIDrivers().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		d.Spec.NodeAllocatableUpdatePeriodSeconds = &expectedCSINodeUpdatePeriodSeconds
		_, err = oc.AdminKubeClient().StorageV1().CSIDrivers().Update(ctx, d, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		g.Fail("failed to update CSIDriver nodeAllocatableUpdatePeriodSeconds: " + err.Error())
	}
	e2e.Logf("Successfully set CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", expectedCSINodeUpdatePeriodSeconds)
}

// waitForENIStatus waits until the ENI reaches the expected status
func waitForENIStatus(ec2Client *ec2.EC2, eniID string, expectedStatus string) {
	o.Eventually(func() (string, error) {
		out, err := ec2Client.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []*string{aws.String(eniID)},
		})
		if err != nil {
			awsErr, ok := err.(awserr.Error)
			if ok {
				if awsErr.Code() == "InvalidNetworkInterfaceID.NotFound" && expectedStatus == "deleted" {
					e2e.Logf("ENI %q is deleted", eniID)
					return "deleted", nil
				}
			}
			e2e.Logf("Failed to get ENI %q: %v, try again ...", eniID, err)
			return "", nil
		}
		return aws.StringValue(out.NetworkInterfaces[0].Status), nil
	}).WithTimeout(eniWaitTimeout).WithPolling(eniPollInterval).Should(o.Equal(expectedStatus),
		"ENI %s should reach status %s", eniID, expectedStatus)
}

// findNextAvailableENIDeviceIndex returns the next free device index for a node
func findNextAvailableENIDeviceIndex(instance *ec2.Instance) int64 {
	used := map[int64]bool{}
	for _, ni := range instance.NetworkInterfaces {
		if ni.Attachment != nil && ni.Attachment.DeviceIndex != nil {
			used[*ni.Attachment.DeviceIndex] = true
		}
	}
	var i int64 = 1
	for {
		if !used[i] {
			return i
		}
		i++
	}
}

// createAndAttachENI creates and attaches an ENI, waits until in-use
func createAndAttachENI(ec2Client *ec2.EC2, instanceID string) string {
	// Get instance info
	out, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	instance := out.Reservations[0].Instances[0]

	deviceIndex := findNextAvailableENIDeviceIndex(instance)
	createInput := &ec2.CreateNetworkInterfaceInput{
		SubnetId:    instance.SubnetId,
		Groups:      []*string{},
		Description: aws.String(fmt.Sprintf("Test ENI device index %d", deviceIndex)),
	}
	for _, sg := range instance.SecurityGroups {
		createInput.Groups = append(createInput.Groups, sg.GroupId)
	}
	eni, err := ec2Client.CreateNetworkInterface(createInput)
	o.Expect(err).NotTo(o.HaveOccurred())
	eniID := *eni.NetworkInterface.NetworkInterfaceId
	e2e.Logf("Created ENI %q", eniID)

	waitForENIStatus(ec2Client, eniID, "available")

	attachInput := &ec2.AttachNetworkInterfaceInput{
		DeviceIndex:        aws.Int64(deviceIndex),
		InstanceId:         aws.String(instanceID),
		NetworkInterfaceId: aws.String(eniID),
	}
	_, err = ec2Client.AttachNetworkInterface(attachInput)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Attached ENI %s to instance %s", eniID, instanceID)

	waitForENIStatus(ec2Client, eniID, "in-use")

	return eniID
}

// detachENI detaches an ENI and waits until available
func detachENI(ec2Client *ec2.EC2, eniID string) {
	out, err := ec2Client.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []*string{aws.String(eniID)},
	})
	if err != nil || len(out.NetworkInterfaces) == 0 {
		e2e.Logf("ENI %q not found (maybe deleted already)", eniID)
		return
	}
	eni := out.NetworkInterfaces[0]
	if eni.Attachment == nil {
		e2e.Logf("ENI %q is not attached", eniID)
		return
	}

	_, err = ec2Client.DetachNetworkInterface(&ec2.DetachNetworkInterfaceInput{
		AttachmentId: eni.Attachment.AttachmentId,
		Force:        aws.Bool(true),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Triggered detach for ENI %q", eniID)

	waitForENIStatus(ec2Client, eniID, "available")
}

// deleteENI deletes an ENI and waits until gone
func deleteENI(ec2Client *ec2.EC2, eniID string) {
	o.Eventually(func() error {
		_, err := ec2Client.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(eniID),
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == "InvalidNetworkInterfaceID.NotFound" {
					return nil
				}
				return fmt.Errorf("retryable ENI %q deletion error: %v", eniID, err)
			}
			return err
		}
		return nil
	}).WithTimeout(eniWaitTimeout).WithPolling(eniPollInterval).Should(o.Succeed(), "ENI %q should be deleted", eniID)
}

// cleanupTestENIsAndRecoverCSINode handles ENI cleanup and CSINode recovery
func cleanupTestENIsAndRecoverCSINode(ctx context.Context, ec2Client *ec2.EC2, oc *exutil.CLI, enisPtr *map[string]bool, driverName, nodeName string, originalAllocatableCount int32) {
	for eniID, attached := range *enisPtr {
		if !attached {
			continue
		}
		e2e.Logf("Cleaning up ENI %s: detaching", eniID)
		detachENI(ec2Client, eniID)
		e2e.Logf("Cleaning up ENI %s: deleting", eniID)
		deleteENI(ec2Client, eniID)
	}

	// Ensuring CSINode allocatable count recovers to original after cleanup ENIs
	// Update CSIDriver to use a shorter update period
	updateCSIDriverUpdatePeriod(ctx, oc, driverName, csiNodeUpdatePeriodSecondsShort)

	// Wait for the CSINode allocatable count to reach the expected value
	o.Eventually(func() int32 {
		current := getCSINodeAllocatableCountByDriver(ctx, oc, nodeName, driverName)
		e2e.Logf("Current CSINode allocatable count: %d, expected: %d", current, originalAllocatableCount)
		return current
	}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(
		o.Equal(originalAllocatableCount),
		"CSINode allocatable count should recover to original",
	)
}

// getCSIDriverUpdatePeriod retrieves a safe copy of the
// CSIDriver.Spec.NodeAllocatableUpdatePeriodSeconds value.
// Returns nil if the field is not set.
func getCSIDriverUpdatePeriod(ctx context.Context, oc *exutil.CLI, driverName string) (*int64, error) {

	client := oc.AdminKubeClient().StorageV1().CSIDrivers()

	csiDriver, err := client.Get(ctx, driverName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CSIDriver %s: %w", driverName, err)
	}

	if period := csiDriver.Spec.NodeAllocatableUpdatePeriodSeconds; period != nil {
		// Make a safe copy (avoid aliasing API object memory)
		val := *period
		e2e.Logf("Stored original CSIDriver %s NodeAllocatableUpdatePeriodSeconds=%d",
			driverName, val)
		return &val, nil
	}

	e2e.Logf("CSIDriver %s has no NodeAllocatableUpdatePeriodSeconds set", driverName)
	return nil, nil
}

// restoreCSIDriverUpdatePeriod restores the original
// NodeAllocatableUpdatePeriodSeconds if it was modified.
func restoreCSIDriverUpdatePeriod(ctx context.Context, oc *exutil.CLI, driverName string, original *int64) {
	if original == nil {
		return
	}

	e2e.Logf("Restoring original CSIDriver NodeAllocatableUpdatePeriodSeconds")
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		csiDriver, err := oc.AdminKubeClient().
			StorageV1().
			CSIDrivers().
			Get(ctx, driverName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		csiDriver.Spec.NodeAllocatableUpdatePeriodSeconds = original

		_, err = oc.AdminKubeClient().
			StorageV1().
			CSIDrivers().
			Update(ctx, csiDriver, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore CSIDriver")
}
