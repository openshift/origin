package storage

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

const (
	gcpSnapshotImagesProjectName                    = "gcp-snapshot-images"
	gcpSnapshotImagesVolumeSnapshotClassName        = "csi-gce-pd-vsc-images"
	gcpSnapshotImagesDefaultVolumeSnapshotClassName = "csi-gce-pd-vsc"
	gcpDriverName                                   = "pd.csi.storage.gke.io"
	gcpSnapshotPollTimeout                          = 10 * time.Minute
	gcpSnapshotPollInterval                         = 10 * time.Second
)

var _ = g.Describe("[sig-storage][Driver:gcp-pd][FeatureGate:CSIDriverSharedResource] GCP PD CSI Driver VolumeSnapshotClass with snapshot-type images", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI(gcpSnapshotImagesProjectName)

	g.BeforeEach(func(ctx g.SpecContext) {
		if !framework.ProviderIs("gce") {
			g.Skip("this test is only expected to work with GCP clusters")
		}
	})

	g.It("should create VolumeSnapshotClass with snapshot-type images parameter", func(ctx g.SpecContext) {
		e2e.Logf("Verifying VolumeSnapshotClass %s exists", gcpSnapshotImagesVolumeSnapshotClassName)

		vsc, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesVolumeSnapshotClassName, "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "VolumeSnapshotClass %s should exist", gcpSnapshotImagesVolumeSnapshotClassName)
		o.Expect(vsc).To(o.ContainSubstring(gcpSnapshotImagesVolumeSnapshotClassName))

		// Verify driver name
		driver, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesVolumeSnapshotClassName, "-o", "jsonpath={.driver}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(driver).To(o.Equal(gcpDriverName), "driver should be %s", gcpDriverName)

		// Verify deletionPolicy
		deletionPolicy, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesVolumeSnapshotClassName, "-o", "jsonpath={.deletionPolicy}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deletionPolicy).To(o.Equal("Delete"), "deletionPolicy should be Delete")

		// Verify snapshot-type parameter
		snapshotType, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesVolumeSnapshotClassName, "-o", "jsonpath={.parameters.snapshot-type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(snapshotType).To(o.Equal("images"), "snapshot-type parameter should be images")

		e2e.Logf("VolumeSnapshotClass %s is correctly configured", gcpSnapshotImagesVolumeSnapshotClassName)
	})

	g.It("should create snapshot using images type and restore data successfully with RWO filesystem ", func(ctx g.SpecContext) {
		const (
			pvcName         = "test-pvc-snapshot-images-rwo"
			snapshotName    = "test-snapshot-images-rwo"
			restoredPVCName = "restored-pvc-from-images-rwo"
			testData        = "test-data-for-gcp-snapshot-images"
			testFileName    = "/mnt/test/testfile.txt"
		)

		// Create PVC with RWO
		e2e.Logf("Creating RWO filesystem PVC %s", pvcName)
		pvc, err := createGCPTestPVC(ctx, oc, oc.Namespace(), pvcName, "10Gi", "", v1.ReadWriteOnce, v1.PersistentVolumeFilesystem)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		}()

		// Create Pod to write data
		e2e.Logf("Creating Pod to write test data")
		writerPod, err := writeDataPod(ctx, oc, oc.Namespace(), "writer-pod-rwo", pvcName, testFileName, testData)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, writerPod.Name, metav1.DeleteOptions{})
		}()

		// Wait for PVC to be bound
		e2e.Logf("Waiting for PVC %s to be bound", pvcName)
		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		// Wait for Pod to complete
		waitForPodSucceeded(ctx, oc, oc.Namespace(), writerPod.Name, "writer")

		// Create snapshot with images type
		e2e.Logf("Creating VolumeSnapshot %s with volumeSnapshotClassName %s", snapshotName, gcpSnapshotImagesVolumeSnapshotClassName)
		err = createGCPSnapshot(oc, oc.Namespace(), snapshotName, pvcName, gcpSnapshotImagesVolumeSnapshotClassName)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().Run("delete").Args("volumesnapshot", snapshotName, "-n", oc.Namespace()).Execute()
		}()

		// Wait for snapshot to be ready
		e2e.Logf("Waiting for VolumeSnapshot %s to be ready", snapshotName)
		o.Eventually(func() bool {
			ready, err := isSnapshotReady(oc, snapshotName)
			if err != nil {
				e2e.Logf("Failed to check if snapshot %s is ready: %v", snapshotName, err)
				return false
			}
			if !ready {
				errMsg, _ := getSnapshotErrorMessage(oc, snapshotName)
				if errMsg != "" {
					e2e.Failf("Snapshot %s is not ready and has error: %s", snapshotName, errMsg)
				}
			}
			return ready
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.BeTrue(), "snapshot should be ready")

		// Verify snapshot has correct size
		restoreSize, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshotName), "-o", "jsonpath={.status.restoreSize}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Snapshot restore size: %s", restoreSize)

		// Create PVC from snapshot
		e2e.Logf("Creating PVC %s from snapshot %s", restoredPVCName, snapshotName)
		restoredPVC, err := createGCPTestPVC(ctx, oc, oc.Namespace(), restoredPVCName, "10Gi", snapshotName, v1.ReadWriteOnce, v1.PersistentVolumeFilesystem)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, restoredPVC.Name, metav1.DeleteOptions{})
		}()

		// Create Pod to verify restored data
		e2e.Logf("Creating Pod to verify restored data")
		readerPod, err := readDataPod(ctx, oc, oc.Namespace(), "reader-pod-rwo", restoredPVCName, testFileName, testData)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, readerPod.Name, metav1.DeleteOptions{})
		}()

		// Wait for restored PVC to be bound
		e2e.Logf("Waiting for restored PVC %s to be bound", restoredPVCName)
		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, restoredPVCName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		// Wait for reader Pod to complete successfully
		waitForPodSucceeded(ctx, oc, oc.Namespace(), readerPod.Name, "reader")

		e2e.Logf("Successfully verified snapshot creation with images type and data restoration for RWO filesystem")
	})

	g.It("should create snapshot using images type and restore data successfully with RWO block", func(ctx g.SpecContext) {
		const (
			pvcName         = "test-pvc-snapshot-images-rwo-block"
			snapshotName    = "test-snapshot-images-rwo-block"
			restoredPVCName = "restored-pvc-from-images-rwo-block"
			testData        = "test-data-for-gcp-snapshot-images-block"
			blockDevice     = "/dev/xvda"
		)

		// Create Block PVC with RWO using default storage class
		e2e.Logf("Creating RWO block PVC %s", pvcName)
		pvc, err := createGCPTestPVC(ctx, oc, oc.Namespace(), pvcName, "10Gi", "", v1.ReadWriteOnce, v1.PersistentVolumeBlock)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		}()

		// Wait for PVC to be bound

		// Create Pod to write data to block device
		e2e.Logf("Creating Pod to write test data to block device")
		writerPod, err := blockWriteDataPod(ctx, oc, oc.Namespace(), "writer-pod-rwo-block", pvcName, blockDevice, testData)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, writerPod.Name, metav1.DeleteOptions{})
		}()

		e2e.Logf("Waiting for PVC %s to be bound", pvcName)
		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		// Wait for Pod to complete
		waitForPodSucceeded(ctx, oc, oc.Namespace(), writerPod.Name, "writer")

		// Create snapshot with images type
		e2e.Logf("Creating VolumeSnapshot %s with volumeSnapshotClassName %s", snapshotName, gcpSnapshotImagesVolumeSnapshotClassName)
		err = createGCPSnapshot(oc, oc.Namespace(), snapshotName, pvcName, gcpSnapshotImagesVolumeSnapshotClassName)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().Run("delete").Args("volumesnapshot", snapshotName, "-n", oc.Namespace()).Execute()
		}()

		// Wait for snapshot to be ready
		e2e.Logf("Waiting for VolumeSnapshot %s to be ready", snapshotName)
		o.Eventually(func() bool {
			ready, err := isSnapshotReady(oc, snapshotName)
			if err != nil {
				e2e.Logf("Failed to check if snapshot %s is ready: %v", snapshotName, err)
				return false
			}
			if !ready {
				errMsg, _ := getSnapshotErrorMessage(oc, snapshotName)
				if errMsg != "" {
					e2e.Failf("Snapshot %s is not ready and has error: %s", snapshotName, errMsg)
				}
			}
			return ready
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.BeTrue(), "snapshot should be ready")

		// Verify snapshot has correct size
		restoreSize, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshotName), "-o", "jsonpath={.status.restoreSize}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Snapshot restore size: %s", restoreSize)

		// Create PVC from snapshot using default storage class
		e2e.Logf("Creating PVC %s from snapshot %s", restoredPVCName, snapshotName)
		restoredPVC, err := createGCPTestPVC(ctx, oc, oc.Namespace(), restoredPVCName, "10Gi", snapshotName, v1.ReadWriteOnce, v1.PersistentVolumeBlock)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, restoredPVC.Name, metav1.DeleteOptions{})
		}()

		// Create Pod to verify restored data from block device
		e2e.Logf("Creating Pod to verify restored data from block device")
		readerPod, err := blockReadDataPod(ctx, oc, oc.Namespace(), "reader-pod-rwo-block", restoredPVCName, blockDevice, testData)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, readerPod.Name, metav1.DeleteOptions{})
		}()

		// Wait for restored PVC to be bound
		e2e.Logf("Waiting for restored PVC %s to be bound", restoredPVCName)
		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, restoredPVCName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		// Wait for reader Pod to complete successfully
		waitForPodSucceeded(ctx, oc, oc.Namespace(), readerPod.Name, "reader")

		e2e.Logf("Successfully verified snapshot creation with images type and data restoration for RWO block")
	})

	g.It("should support multiple snapshots from same PVC using images type", func(ctx g.SpecContext) {
		const (
			pvcName      = "test-pvc-multi-snapshots"
			snapshot1    = "test-snapshot-images-1"
			snapshot2    = "test-snapshot-images-2"
			testData1    = "first-snapshot-data"
			testData2    = "second-snapshot-data"
			testFileName = "/mnt/test/testfile.txt"
		)

		// Create PVC
		e2e.Logf("Creating PVC %s", pvcName)
		pvc, err := createGCPTestPVC(ctx, oc, oc.Namespace(), pvcName, "10Gi", "", v1.ReadWriteOnce, v1.PersistentVolumeFilesystem)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		}()

		// Write first data and create first snapshot
		e2e.Logf("Writing first test data")
		writerPod1, err := writeDataPod(ctx, oc, oc.Namespace(), "writer-pod-1", pvcName, testFileName, testData1)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, writerPod1.Name, metav1.DeleteOptions{})
		}()

		// Wait for PVC to be bound
		e2e.Logf("Waiting for PVC %s to be bound", pvcName)
		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		waitForPodSucceeded(ctx, oc, oc.Namespace(), writerPod1.Name, "writer-1")

		e2e.Logf("Creating first snapshot %s", snapshot1)
		err = createGCPSnapshot(oc, oc.Namespace(), snapshot1, pvcName, gcpSnapshotImagesVolumeSnapshotClassName)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().Run("delete").Args("volumesnapshot", snapshot1, "-n", oc.Namespace()).Execute()
		}()

		o.Eventually(func() bool {
			ready, err := isSnapshotReady(oc, snapshot1)
			if err != nil {
				e2e.Logf("Failed to check if snapshot %s is ready: %v", snapshot1, err)
				return false
			}
			if !ready {
				errMsg, _ := getSnapshotErrorMessage(oc, snapshot1)
				if errMsg != "" {
					e2e.Failf("Snapshot %s is not ready and has error: %s", snapshot1, errMsg)
				}
			}
			return ready
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.BeTrue())

		// Write second data and create second snapshot
		e2e.Logf("Writing second test data")
		writerPod2, err := writeDataPod(ctx, oc, oc.Namespace(), "writer-pod-2", pvcName, testFileName, testData2)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, writerPod2.Name, metav1.DeleteOptions{})
		}()

		waitForPodSucceeded(ctx, oc, oc.Namespace(), writerPod2.Name, "writer-2")

		e2e.Logf("Creating second snapshot %s", snapshot2)
		err = createGCPSnapshot(oc, oc.Namespace(), snapshot2, pvcName, gcpSnapshotImagesVolumeSnapshotClassName)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().Run("delete").Args("volumesnapshot", snapshot2, "-n", oc.Namespace()).Execute()
		}()

		o.Eventually(func() bool {
			ready, err := isSnapshotReady(oc, snapshot2)
			if err != nil {
				e2e.Logf("Failed to check if snapshot %s is ready: %v", snapshot2, err)
				return false
			}
			if !ready {
				errMsg, _ := getSnapshotErrorMessage(oc, snapshot2)
				if errMsg != "" {
					e2e.Failf("Snapshot %s is not ready and has error: %s", snapshot2, errMsg)
				}
			}
			return ready
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.BeTrue())

		// Verify both snapshots have different handles
		handle1, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshot1), "-o", "jsonpath={.status.boundVolumeSnapshotContentName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		handle2, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshot2), "-o", "jsonpath={.status.boundVolumeSnapshotContentName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(handle1).NotTo(o.Equal(handle2), "snapshot handles should be different")

		// Restore from snapshot1 and verify it contains testData1
		e2e.Logf("Restoring from snapshot1 to verify point-in-time data")
		const restoredPVC1Name = "restored-pvc-from-snapshot1"
		restoredPVC1, err := createGCPTestPVC(ctx, oc, oc.Namespace(), restoredPVC1Name, "10Gi", snapshot1, v1.ReadWriteOnce, v1.PersistentVolumeFilesystem)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, restoredPVC1.Name, metav1.DeleteOptions{})
		}()

		readerPod1, err := readDataPod(ctx, oc, oc.Namespace(), "reader-pod-snapshot1", restoredPVC1Name, testFileName, testData1)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, readerPod1.Name, metav1.DeleteOptions{})
		}()

		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, restoredPVC1Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		waitForPodSucceeded(ctx, oc, oc.Namespace(), readerPod1.Name, "reader-snapshot1")

		// Restore from snapshot2 and verify it contains testData2
		e2e.Logf("Restoring from snapshot2 to verify point-in-time data")
		const restoredPVC2Name = "restored-pvc-from-snapshot2"
		restoredPVC2, err := createGCPTestPVC(ctx, oc, oc.Namespace(), restoredPVC2Name, "10Gi", snapshot2, v1.ReadWriteOnce, v1.PersistentVolumeFilesystem)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, restoredPVC2.Name, metav1.DeleteOptions{})
		}()

		readerPod2, err := readDataPod(ctx, oc, oc.Namespace(), "reader-pod-snapshot2", restoredPVC2Name, testFileName, testData2)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, readerPod2.Name, metav1.DeleteOptions{})
		}()

		o.Eventually(func() v1.PersistentVolumeClaimPhase {
			pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, restoredPVC2Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return pvc.Status.Phase
		}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.Equal(v1.ClaimBound))

		waitForPodSucceeded(ctx, oc, oc.Namespace(), readerPod2.Name, "reader-snapshot2")

		e2e.Logf("Successfully verified multiple snapshots with images type from same PVC captured correct point-in-time data")
	})

	g.It("should not be marked as default VolumeSnapshotClass", func(ctx g.SpecContext) {
		e2e.Logf("Verifying that %s is not marked as default", gcpSnapshotImagesVolumeSnapshotClassName)

		// Check for default annotation
		annotation, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesVolumeSnapshotClassName, "-o", "jsonpath={.metadata.annotations.snapshot\\.storage\\.kubernetes\\.io/is-default-class}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(annotation).NotTo(o.Equal("true"), "images VolumeSnapshotClass should not be marked as default")

		// Verify the default class is the standard one
		defaultAnnotation, err := oc.AsAdmin().Run("get").Args("volumesnapshotclass", gcpSnapshotImagesDefaultVolumeSnapshotClassName, "-o", "jsonpath={.metadata.annotations.snapshot\\.storage\\.kubernetes\\.io/is-default-class}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(defaultAnnotation).To(o.Equal("true"), "default VolumeSnapshotClass should be %s", gcpSnapshotImagesDefaultVolumeSnapshotClassName)

		e2e.Logf("Verified that %s is correctly not marked as default", gcpSnapshotImagesVolumeSnapshotClassName)
	})
})

// Helper functions

// waitForPodSucceeded waits for a pod to succeed, failing fast if the pod enters Failed state
func waitForPodSucceeded(ctx context.Context, oc *exutil.CLI, namespace string, podName string, description string) {
	e2e.Logf("Waiting for %s pod %s to complete", description, podName)
	o.Eventually(func() bool {
		pod, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if pod.Status.Phase == v1.PodSucceeded {
			return true
		}

		if pod.Status.Phase == v1.PodFailed {
			// Collect pod logs and status for debugging
			logs, _ := oc.Run("logs").Args(podName, "-n", namespace).Output()
			o.Expect(pod.Status.Phase).NotTo(o.Equal(v1.PodFailed),
				"Pod %s failed. Reason: %s, Message: %s, Logs:\n%s",
				podName, pod.Status.Reason, pod.Status.Message, logs)
		}

		return false
	}, gcpSnapshotPollTimeout, gcpSnapshotPollInterval).Should(o.BeTrue(),
		"%s pod %s should complete successfully", description, podName)
}

func createGCPTestPVC(ctx context.Context, oc *exutil.CLI, namespace string, pvcName string, volumeSize string, snapshotName string, accessMode v1.PersistentVolumeAccessMode, volumeMode v1.PersistentVolumeMode) (*v1.PersistentVolumeClaim, error) {
	return createGCPTestPVCWithStorageClass(ctx, oc, namespace, pvcName, volumeSize, snapshotName, accessMode, volumeMode, "")
}

func createGCPTestPVCWithStorageClass(ctx context.Context, oc *exutil.CLI, namespace string, pvcName string, volumeSize string, snapshotName string, accessMode v1.PersistentVolumeAccessMode, volumeMode v1.PersistentVolumeMode, storageClassName string) (*v1.PersistentVolumeClaim, error) {
	e2e.Logf("Creating PVC %s in namespace %s with size %s, accessMode %s, volumeMode %s, storageClass %s", pvcName, namespace, volumeSize, accessMode, volumeMode, storageClassName)

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{accessMode},
			VolumeMode:  &volumeMode,
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse(volumeSize),
				},
			},
		},
	}

	// Set storage class if specified
	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}

	// If snapshotName is provided, restore from snapshot
	if snapshotName != "" {
		pvc.Spec.DataSource = &v1.TypedLocalObjectReference{
			APIGroup: func() *string { s := "snapshot.storage.k8s.io"; return &s }(),
			Kind:     "VolumeSnapshot",
			Name:     snapshotName,
		}
	}

	return oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

// createPod creates a pod with filesystem volume mount
func createPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, containerName string, command []string, args []string, mountPath string) (*v1.Pod, error) {
	e2e.Logf("Creating Pod %s with filesystem volume", podName)

	allowPrivEsc := false
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    containerName,
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: command,
					Args:    args,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "test-volume",
							MountPath: mountPath,
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
			Volumes: []v1.Volume{
				{
					Name: "test-volume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	return oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// createBlockPod creates a pod with block volume device
func createBlockPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, containerName string, command []string, args []string, devicePath string) (*v1.Pod, error) {
	e2e.Logf("Creating Pod %s with block volume device", podName)

	allowPrivEsc := false
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    containerName,
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: command,
					Args:    args,
					VolumeDevices: []v1.VolumeDevice{
						{
							Name:       "test-block",
							DevicePath: devicePath,
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
			Volumes: []v1.Volume{
				{
					Name: "test-block",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	return oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// writeDataPod creates a pod that writes data to a filesystem volume
func writeDataPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, fileName string, data string) (*v1.Pod, error) {
	e2e.Logf("Creating writer Pod %s to write data to %s", podName, fileName)

	command := []string{"/bin/sh"}
	args := []string{
		"-c",
		fmt.Sprintf("echo '%s' > %s && sync && echo 'Data written successfully'", data, fileName),
	}

	return createPod(ctx, oc, namespace, podName, pvcName, "writer", command, args, "/mnt/test")
}

// readDataPod creates a pod that reads and verifies data from a filesystem volume
func readDataPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, fileName string, expectedData string) (*v1.Pod, error) {
	e2e.Logf("Creating reader Pod %s to verify data in %s", podName, fileName)

	command := []string{"/bin/sh"}
	args := []string{
		"-c",
		fmt.Sprintf("ACTUAL=$(cat %s) && echo \"Read data: $ACTUAL\" && if [ \"$ACTUAL\" = \"%s\" ]; then echo 'Data verification successful'; exit 0; else echo 'Data verification failed'; exit 1; fi", fileName, expectedData),
	}

	return createPod(ctx, oc, namespace, podName, pvcName, "reader", command, args, "/mnt/test")
}

// blockWriteDataPod creates a pod that writes data to a block device
func blockWriteDataPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, blockDevice string, data string) (*v1.Pod, error) {
	e2e.Logf("Creating block writer Pod %s to write data to %s", podName, blockDevice)

	command := []string{"/bin/sh"}
	args := []string{
		"-c",
		fmt.Sprintf("echo '%s' | dd of=%s bs=512 count=1 conv=fsync && echo 'Data written successfully to block device'", data, blockDevice),
	}

	return createBlockPod(ctx, oc, namespace, podName, pvcName, "writer", command, args, blockDevice)
}

// blockReadDataPod creates a pod that reads and verifies data from a block device
func blockReadDataPod(ctx context.Context, oc *exutil.CLI, namespace string, podName string, pvcName string, blockDevice string, expectedData string) (*v1.Pod, error) {
	e2e.Logf("Creating block reader Pod %s to verify data in %s", podName, blockDevice)

	command := []string{"/bin/sh"}
	args := []string{
		"-c",
		fmt.Sprintf("ACTUAL=$(dd if=%s bs=512 count=1 2>/dev/null | tr -d '\\0') && echo \"Read data: $ACTUAL\" && if echo \"$ACTUAL\" | grep -q \"%s\"; then echo 'Data verification successful'; exit 0; else echo 'Data verification failed'; exit 1; fi", blockDevice, expectedData),
	}

	return createBlockPod(ctx, oc, namespace, podName, pvcName, "reader", command, args, blockDevice)
}

func createGCPSnapshot(oc *exutil.CLI, namespace string, snapshotName string, pvcName string, volumeSnapshotClassName string) error {
	e2e.Logf("Creating VolumeSnapshot %s for PVC %s in namespace %s with class %s", snapshotName, pvcName, namespace, volumeSnapshotClassName)

	snapshot := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: %s
  namespace: %s
spec:
  volumeSnapshotClassName: %s
  source:
    persistentVolumeClaimName: %s
`, snapshotName, namespace, volumeSnapshotClassName, pvcName)

	err := oc.AsAdmin().Run("apply").Args("-f", "-").InputString(snapshot).Execute()
	if err != nil {
		return fmt.Errorf("failed to create VolumeSnapshot: %v", err)
	}

	return nil
}
