package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/kubelet/events"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eevents "k8s.io/kubernetes/test/e2e/framework/events"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
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
		targetWorkerNode                    *v1.Node
		ec2Client                           *ec2.EC2
		originalAllocatableCount            int32
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
		operators := []operatorInfo{
			{"openshift-cluster-version", "cluster-version-operator"},
			{"openshift-cluster-storage-operator", "cluster-storage-operator"},
			{"openshift-cluster-csi-drivers", "aws-ebs-csi-driver-operator"},
		}
		scaleOperatorsDown(oc, operators)
		g.DeferCleanup(scaleOperatorsUp, ctx, oc, operators)

		// Store original CSIDriver configuration
		originalCSIDriverUpdatePeriod, err = getCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Stored original CSIDriver %s configuration", ebsCSIDriverName)

		g.By("# Get a schedulable worker node")
		targetWorkerNode, err = e2enode.GetRandomReadySchedulableNode(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get a schedulable worker node")
		e2e.Logf("Selected worker node: %s", targetWorkerNode.Name)

		g.By("# Get the schedulable worker node attached volume count from VolumeAttachments")
		targetWorkerNodeAttachedVolumeCount = getAttachedVolumeCountFromVolumeAttachments(ctx, oc, targetWorkerNode.Name)

		g.By("# Get the original CSINode allocatable count for worker node")
		originalAllocatableCount = getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode.Name, ebsCSIDriverName)
		o.Expect(originalAllocatableCount).To(o.BeNumerically(">", 0), "original allocatable count should be greater than 0")
		e2e.Logf("Original CSINode allocatable count for node %s: %d", targetWorkerNode.Name, originalAllocatableCount)

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
		g.DeferCleanup(updateCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, *originalCSIDriverUpdatePeriod)

		g.By("# Checking the nodeAllocatableUpdatePeriodSeconds should not reconcile back since operator is scaled down")
		currentNodeAllocatableUpdatePeriodSeconds, err := getCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*currentNodeAllocatableUpdatePeriodSeconds).To(o.Equal(csiNodeUpdatePeriodSecondsShort))

	})

	g.It("should automatically update CSINode allocatable count when instance attached ENI count changes", func() {
		g.By(fmt.Sprintf("# Setting CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", csiNodeUpdatePeriodSecondsShort))
		updateCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName, csiNodeUpdatePeriodSecondsShort)
		g.DeferCleanup(updateCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, *originalCSIDriverUpdatePeriod)

		g.By("# Attaching 2 ENIs to the worker node")
		// Get instance ID from node
		instanceID := getAWSInstanceIDFromNode(ctx, oc, targetWorkerNode.Name)
		e2e.Logf("Instance ID for node %s: %s", targetWorkerNode.Name, instanceID)

		testENIs := sets.New[string]()
		g.DeferCleanup(cleanupTestENIsAndRecoverCSINode, ctx, ec2Client, oc, testENIs, ebsCSIDriverName, targetWorkerNode.Name, originalAllocatableCount)

		for i := 0; i < 2; i++ {
			eniID := createAndAttachENI(ec2Client, instanceID)
			testENIs.Insert(eniID)
			e2e.Logf("Attached ENI: %s", eniID)
		}

		g.By(fmt.Sprintf("# Waiting for CSINode allocatable count to update to %d", originalAllocatableCount-2))
		expectedCount := originalAllocatableCount - 2
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode.Name, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should update to original - 2")
		e2e.Logf("Successfully verified CSINode allocatable count updated to %d", expectedCount)

		g.By(fmt.Sprintf("# Detaching 1 ENI from node and waiting for CSINode allocatable count update to %d", originalAllocatableCount-1))
		firstENIToRemove := testENIs.UnsortedList()[0]
		detachENI(ec2Client, firstENIToRemove)
		deleteENI(ec2Client, firstENIToRemove)
		testENIs.Delete(firstENIToRemove)
		e2e.Logf("Detached ENI: %s", firstENIToRemove)

		expectedCount = originalAllocatableCount - 1
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode.Name, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should update to original - 1")
		e2e.Logf("Successfully verified CSINode allocatable count updated to %d", expectedCount)
	})

	g.It("should immediately update CSINode allocatable count when ResourceExhausted errors occur", func() {

		g.By(fmt.Sprintf("# Setting CSIDriver nodeAllocatableUpdatePeriodSeconds to %d", csiNodeUpdatePeriodSecondsLong))
		updateCSIDriverUpdatePeriod(ctx, oc, ebsCSIDriverName, csiNodeUpdatePeriodSecondsLong)
		g.DeferCleanup(updateCSIDriverUpdatePeriod, ctx, oc, ebsCSIDriverName, *originalCSIDriverUpdatePeriod)

		g.By("# Attaching 1 ENI to the worker node")
		// Get instance ID from node
		instanceID := getAWSInstanceIDFromNode(ctx, oc, targetWorkerNode.Name)
		e2e.Logf("Instance ID for node %s: %s", targetWorkerNode.Name, instanceID)

		testENIs := sets.New[string]()
		g.DeferCleanup(cleanupTestENIsAndRecoverCSINode, ctx, ec2Client, oc, testENIs, ebsCSIDriverName, targetWorkerNode.Name, originalAllocatableCount)
		eniID := createAndAttachENI(ec2Client, instanceID)
		testENIs.Insert(eniID)
		e2e.Logf("Attached ENI: %q", eniID)

		g.By("# Checking CSINode allocatable count should not update")
		o.Consistently(func() int32 {
			return getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode.Name, ebsCSIDriverName)
		}, 6*csiNodeUpdatePollInterval, csiNodeUpdatePollInterval).Should(o.Equal(originalAllocatableCount),
			"CSINode allocatable count should not update less than nodeAllocatableUpdatePeriodSeconds")

		g.By("# Checking CSINode allocatable count updates after ResourceExhausted attach errors occur")
		storageClassName := GetCSIStorageClassByProvisioner(ctx, oc, ebsCSIDriverName)
		e2e.Logf("Using StorageClass: %s", storageClassName)

		// The StatefulSet will have replicas equal to the original allocatable count
		// This should trigger ResourceExhausted errors for the last pod
		replicas := originalAllocatableCount - targetWorkerNodeAttachedVolumeCount
		statefulSetName := "storage-mca-test-sts"

		g.By(fmt.Sprintf("# Creating StatefulSet %s with %d replicas scheduled to node %s", statefulSetName, replicas, targetWorkerNode.Name))
		statefulSet := createStatefulSetWithVolumeTemplate(oc.Namespace(), statefulSetName, replicas, targetWorkerNode.Name, storageClassName)
		_, err := oc.AdminKubeClient().AppsV1().StatefulSets(oc.Namespace()).Create(ctx, statefulSet, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create StatefulSet")
		g.DeferCleanup(cleanupStatefulSetWithPVCs, ctx, oc, oc.Namespace(), statefulSetName, replicas)

		// Observe that the last replica pod transitions from ContainerCreating to Pending
		// and that the ResourceExhausted error triggers an immediate CSINode allocatable count update
		lastPodName := fmt.Sprintf("%s-%d", statefulSetName, replicas-1)

		g.By(fmt.Sprintf("# Monitoring pod %s for ResourceExhausted error and CSINode update", lastPodName))
		// Wait for StatefulSet to attempt creating all replicas (currentReplicas == spec.replicas)
		e2e.Logf("Waiting for StatefulSet %s to have currentReplicas=%d equal to spec.replicas", statefulSetName, replicas)
		o.Eventually(func() bool {
			sts, err := oc.AdminKubeClient().AppsV1().StatefulSets(oc.Namespace()).Get(ctx, statefulSetName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get StatefulSet %s: %v", statefulSetName, err)
				return false
			}
			e2e.Logf("StatefulSet %s: currentReplicas=%d, replicas=%d", statefulSetName, sts.Status.CurrentReplicas, replicas)
			return sts.Status.CurrentReplicas == replicas
		}, resourceExhaustionTimeout, csiNodeUpdatePollInterval).Should(o.BeTrue(),
			"StatefulSet should attempt to create all %d replicas", replicas)
		e2e.Logf("StatefulSet %s has currentReplicas=%d", statefulSetName, replicas)

		// Wait for the last pod to show ResourceExhausted error
		expectedEventSelector := fields.Set{
			"involvedObject.kind":      "Pod",
			"involvedObject.name":      lastPodName,
			"involvedObject.namespace": oc.Namespace(),
			"reason":                   events.FailedAttachVolume,
		}.AsSelector().String()
		err = e2eevents.WaitTimeoutForEvent(ctx, oc.AdminKubeClient(), oc.Namespace(), expectedEventSelector, "ResourceExhausted", resourceExhaustionTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to observe ResourceExhausted event for pod %s", lastPodName)

		// Verify that CSINode allocatable count was updated immediately (not waiting for the long update period)
		// The count should remain at original-1 because we still have 1 ENI attached
		g.By(fmt.Sprintf("# Verifying that CSINode allocatable count updated to %d (immediate update triggered by ResourceExhausted)", originalAllocatableCount-1))
		expectedCount := originalAllocatableCount - 1
		o.Eventually(func() int32 {
			currentCount := getCSINodeAllocatableCountByDriver(ctx, oc, targetWorkerNode.Name, ebsCSIDriverName)
			e2e.Logf("Current CSINode allocatable count: %d, expected: %d", currentCount, expectedCount)
			return currentCount
		}, csiNodeUpdatePollTimeout, csiNodeUpdatePollInterval).Should(o.Equal(expectedCount),
			"CSINode allocatable count should be immediately updated despite long update period")
		e2e.Logf("Successfully verified that ResourceExhausted error triggered immediate CSINode update")

		g.By("# Verifying the last pod should FailedScheduling after CSINode allocatable count updated")
		expectedEventSelector = fields.Set{
			"involvedObject.kind":      "Pod",
			"involvedObject.name":      lastPodName,
			"involvedObject.namespace": oc.Namespace(),
			"reason":                   "FailedScheduling",
		}.AsSelector().String()
		err = e2eevents.WaitTimeoutForEvent(ctx, oc.AdminKubeClient(), oc.Namespace(), expectedEventSelector, "exceed max volume count", resourceExhaustionTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to observe FailedScheduling event for pod %s", lastPodName)
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

	// Retry until attach succeeds, DescribeInstances is eventually consistent.
	// Even after refreshing the instance info, the device index may still be reported as in-use
	// by AWS for a short period after ENI attached/detached. Retry refreshing the instance info and attaching until successful.
	o.Eventually(func() error {
		// Refresh the instance
		out, err = ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{aws.String(instanceID)},
		})
		if err != nil {
			return err
		}
		instance = out.Reservations[0].Instances[0]
		deviceIndex = findNextAvailableENIDeviceIndex(instance)

		// Try to attach
		attachInput := &ec2.AttachNetworkInterfaceInput{
			DeviceIndex:        aws.Int64(deviceIndex),
			InstanceId:         aws.String(instanceID),
			NetworkInterfaceId: aws.String(eniID),
		}
		_, err = ec2Client.AttachNetworkInterface(attachInput)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidParameterValue" {
				e2e.Logf("Device index %d still in use, retrying...", deviceIndex)
				return fmt.Errorf("device index still in use, retry attach")
			}
			return err
		}

		return nil
	}, 60*time.Second, 5*time.Second).Should(o.Succeed(), "failed to attach ENI after retries")
	e2e.Logf("Attached ENI %q to instance %q at device index %d", eniID, instanceID, deviceIndex)

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

// cleanupENIs detaches and deletes a set of ENIs
func cleanupENIs(ec2Client *ec2.EC2, enis sets.Set[string]) {
	for _, eniID := range enis.UnsortedList() {
		e2e.Logf("Cleaning up ENI %s: detaching", eniID)
		detachENI(ec2Client, eniID)
		e2e.Logf("Cleaning up ENI %s: deleting", eniID)
		deleteENI(ec2Client, eniID)
	}
}

// cleanupTestENIsAndRecoverCSINode handles ENI cleanup and CSINode recovery
func cleanupTestENIsAndRecoverCSINode(ctx context.Context, ec2Client *ec2.EC2, oc *exutil.CLI, enis sets.Set[string], driverName, nodeName string, originalAllocatableCount int32) {
	cleanupENIs(ec2Client, enis)

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

type operatorInfo struct {
	namespace string
	name      string
}

// scaleOperatorsDown scales the specified operators down to 0 replicas by order
func scaleOperatorsDown(oc *exutil.CLI, operators []operatorInfo) {
	for _, op := range operators {
		g.By(fmt.Sprintf("Scaling down %s in namespace %s", op.name, op.namespace))
		err := oc.AsAdmin().Run("scale").Args(
			"deployment/"+op.name,
			"--namespace="+op.namespace,
			"--replicas=0",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to scale down %s", op.name)
		e2e.Logf("Successfully scaled down %s", op.name)
	}
}

// scaleOperatorsUp scales the specified operators up to 1 replica and waits for them to be ready
// it also waits for the cluster storage operator to be healthy
func scaleOperatorsUp(ctx context.Context, oc *exutil.CLI, operators []operatorInfo) {
	// Scale up all operators
	for _, op := range operators {
		g.By(fmt.Sprintf("Scaling up %s in namespace %s", op.name, op.namespace))
		err := oc.AsAdmin().Run("scale").Args(
			"deployment/"+op.name,
			"--namespace="+op.namespace,
			"--replicas=1",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to scale up %s", op.name)
		e2e.Logf("Successfully scaled up %s", op.name)
	}

	// Waiting for each operator to be ready
	for _, op := range operators {
		g.By(fmt.Sprintf("Waiting for %s to be ready", op.name))
		o.Eventually(func() error {
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(op.namespace).Get(ctx, op.name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get deployment %s/%s: %v", op.namespace, op.name, err)
			}
			if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas == 0 {
				return fmt.Errorf("waiting for %s to be scaled up", op.name)
			}
			return e2edeployment.WaitForDeploymentComplete(oc.AdminKubeClient(), deployment)
		}).WithTimeout(csiNodeUpdatePollTimeout).WithPolling(csiNodeUpdatePollInterval).Should(o.Succeed(), "%s should be available", op.name)
		e2e.Logf("%s is ready", op.name)
	}

	// Wait for cluster storage operator to be healthy
	g.By("Waiting for cluster storage operator to be available")
	WaitForCSOHealthy(oc)
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
