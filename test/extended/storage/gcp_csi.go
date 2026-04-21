package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	v1 "k8s.io/api/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/ptr"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	gcpPDCSIProvisioner = "pd.csi.storage.gke.io"

	// gcpPDImagesVolumeSnapshotClass must match the VolumeSnapshotClass installed by openshift/gcp-pd-csi-driver-operator for snapshot-type images (used by the operator-exposure test only).
	gcpPDImagesVolumeSnapshotClass = "csi-gce-pd-vsc-images"

	// gcpE2EImagesVSCPrefix is the name prefix for test-created VolumeSnapshotClasses with snapshot-type images (filesystem and block snapshot tests).
	gcpE2EImagesVSCPrefix = "e2e-gcp-pd-vsc-images"

	// gcpSnapshotPollTimeout is longer than generic snapshot polls because image snapshots can take longer to reach readyToUse.
	gcpSnapshotPollTimeout  = 10 * time.Minute
	gcpSnapshotPollInterval = 5 * time.Second
)

var (
	gcpSnapshotGVR        = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	gcpSnapshotContentGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}
	gcpSnapshotClassGVR   = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
)

var _ = g.Describe("[sig-storage][Feature:VolumeSnapshotDataSource][Driver: pd.csi.storage.gke.io] GCP PD CSI image volumesnapshot tests", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("gcp-snapshot-images", admissionapi.LevelPrivileged)
		dc dynamic.Interface
	)

	g.BeforeEach(func() {
		if !e2e.ProviderIs("gce") {
			g.Skip("GCP PD CSI image snapshot tests only run on GCE-based clusters")
		}

		// Check if Storage is enabled
		isStorageEnabled, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityStorage)
		if err != nil || !isStorageEnabled {
			g.Skip("skipping, this test is only expected to work with storage enabled clusters")
		}
	})

	g.It("should expose a VolumeSnapshotClass for snapshot-type images that is not the default", func(ctx g.SpecContext) {
		dc = oc.AdminDynamicClient()

		g.By(fmt.Sprintf("Checking if the operator VolumeSnapshotClass %q exists", gcpPDImagesVolumeSnapshotClass))
		imagesVSC, err := dc.Resource(gcpSnapshotClassGVR).Get(ctx, gcpPDImagesVolumeSnapshotClass, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			e2e.Failf("CSI driver generated VolumeSnapshotClass %q must exist", gcpPDImagesVolumeSnapshotClass)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking if the VolumeSnapshotClass uses the GCP PD CSI driver")
		driver, _, err := unstructured.NestedString(imagesVSC.Object, "driver")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(driver).To(o.Equal(gcpPDCSIProvisioner))

		g.By("Checking if deletionPolicy is Delete so snapshot deletion removes backing resources")
		deletionPolicy, _, err := unstructured.NestedString(imagesVSC.Object, "deletionPolicy")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deletionPolicy).To(o.Equal("Delete"), "operator sets Delete so snapshot deletion removes backing resources")

		g.By("Checking if parameters: 'snapshot-type: images' is set in the VolumeSnapshotClass")
		params, _, err := unstructured.NestedStringMap(imagesVSC.Object, "parameters")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(params["snapshot-type"]).To(o.Equal("images"))

		g.By("Listing VolumeSnapshotClasses to resolve defaults for the GCP PD driver")
		list, err := dc.Resource(gcpSnapshotClassGVR).List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var defaultForGCPDriver []string
		for _, item := range list.Items {
			d, _, err := unstructured.NestedString(item.Object, "driver")
			if err != nil || d != gcpPDCSIProvisioner {
				continue
			}
			a := item.GetAnnotations()
			if a["snapshot.storage.kubernetes.io/is-default-class"] == "true" {
				defaultForGCPDriver = append(defaultForGCPDriver, item.GetName())
			}
		}

		g.By("Checking if a default VolumeSnapshotClass exists for the driver, and that the images class is not the default")
		o.Expect(defaultForGCPDriver).NotTo(o.BeEmpty(), "expected a default VolumeSnapshotClass for driver %s", gcpPDCSIProvisioner)
		o.Expect(defaultForGCPDriver).NotTo(o.ContainElement(gcpPDImagesVolumeSnapshotClass))
	})

	g.It("should create an image snapshot from RWO filesystem PVC and restore data", func(ctx g.SpecContext) {
		ns := oc.Namespace()
		dc = oc.AdminDynamicClient()
		customVolumeSnapshotClass := fmt.Sprintf("%s-%d", gcpE2EImagesVSCPrefix, time.Now().UnixNano())

		g.By(fmt.Sprintf("Creating test VolumeSnapshotClass %q with snapshot-type images", customVolumeSnapshotClass))
		createImagesVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)
		defer deleteVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)

		var (
			pvcName      = "test-pvc-snapshot-image-rwo"
			snapName     = "test-snapshot-image-rwo"
			restorePVC   = "restored-pvc-from-snapshot-image-rwo"
			testFileName = "/mnt/test/testfile.txt"
			testData     = fmt.Sprintf("gcp-pd-snapshot-image-test-data-%d", time.Now().UnixNano())
		)

		g.By("Creating source RWO filesystem PVC")
		_, err := createTestPVC(ctx, oc, ns, pvcName, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeFilesystem), nil))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePVC(ctx, oc, ns, pvcName)

		g.By("Creating writer Pod so the volume can bind")
		writerPod := newBusyBoxWriterPod(ns, "gcp-fs-writer", pvcName, fmt.Sprintf(`set -e; mkdir -p /mnt/test; echo -n '%s' > %s`, testData, testFileName))
		_, err = createTestPodFromSpec(ctx, oc, writerPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, writerPod.Name, metav1.DeleteOptions{})

		g.By("Waiting for source PVC to reach Bound")
		waitPVCBound(ctx, oc, ns, pvcName)

		g.By("Waiting for writer pod to complete")
		waitPodSucceeded(ctx, oc, ns, writerPod.Name, e2e.PodStartTimeout)

		g.By(fmt.Sprintf("Creating VolumeSnapshot %q from source PVC using image snapshot class", snapName))
		err = createSnapshot(oc, ns, snapName, pvcName, customVolumeSnapshotClass)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteVolumeSnapshot(ctx, oc, ns, snapName)

		g.By("Waiting for VolumeSnapshot readyToUse")
		waitForSnapshotReady(oc, snapName)

		g.By("Resolving bound VolumeSnapshotContent name")
		contentName := snapshotBoundContentName(ctx, dc, ns, snapName)
		o.Expect(contentName).NotTo(o.BeEmpty())

		g.By("Creating restore PVC from VolumeSnapshot")
		_, err = createTestPVC(ctx, oc, ns, restorePVC, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeFilesystem), snapshotDataSource(snapName)))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePVC(ctx, oc, ns, restorePVC)

		g.By("Creating reader Pod to verify restored file contents")
		readerPod := newBusyBoxReaderPod(ns, "gcp-fs-reader", restorePVC, fmt.Sprintf(`set -e; test "$(cat '%s')" = '%s'`, testFileName, testData))
		_, err = createTestPodFromSpec(ctx, oc, readerPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, readerPod.Name, metav1.DeleteOptions{})

		g.By("Waiting for restore PVC to reach Bound")
		waitPVCBound(ctx, oc, ns, restorePVC)

		g.By("Waiting for reader Pod to complete successfully")
		waitPodSucceeded(ctx, oc, ns, readerPod.Name, e2e.PodStartTimeout)

		g.By("Deleting VolumeSnapshot and waiting for VolumeSnapshotContent removal")
		o.Expect(oc.AsAdmin().Run("delete").Args("volumesnapshot", snapName, "-n", ns).Execute()).NotTo(o.HaveOccurred())
		waitUntilVolumeSnapshotDeleted(ctx, dc, ns, snapName)
		waitUntilSnapshotContentDeleted(ctx, dc, contentName)
	})

	g.It("should create an image snapshot from RWO block PVC and restore data", func(ctx g.SpecContext) {
		ns := oc.Namespace()
		dc = oc.AdminDynamicClient()
		customVolumeSnapshotClass := fmt.Sprintf("%s-%d", gcpE2EImagesVSCPrefix, time.Now().UnixNano())

		g.By(fmt.Sprintf("Creating test VolumeSnapshotClass %q with snapshot-type images", customVolumeSnapshotClass))
		createImagesVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)
		defer deleteVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)

		var (
			pvcName    = "test-pvc-snapshot-image-rwo-block"
			snapName   = "test-snapshot-image-rwo-block"
			restorePVC = "restored-pvc-from-snapshot-image-rwo-block"
			blockDev   = "/dev/e2eblock"
			testData   = fmt.Sprintf("gcp-pd-snapshot-image-test-data-block-%d", time.Now().UnixNano())
		)

		g.By("Creating source RWO block PVC")
		_, err := createTestPVC(ctx, oc, ns, pvcName, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeBlock), nil))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePVC(ctx, oc, ns, pvcName)

		g.By("Creating block writer Pod so the volume can bind and writing test data")
		writerPod := newBusyBoxBlockPod(ns, "gcp-blk-writer", pvcName, blockDev,
			fmt.Sprintf("echo '%s' | dd of=%s bs=512 count=1 conv=fsync && echo 'Data written successfully to block device'", testData, blockDev))
		_, err = createTestPodFromSpec(ctx, oc, writerPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, writerPod.Name, metav1.DeleteOptions{})

		g.By("Waiting for source block PVC to reach Bound")
		waitPVCBound(ctx, oc, ns, pvcName)

		g.By("Waiting for block writer Pod to complete")
		waitPodSucceeded(ctx, oc, ns, writerPod.Name, e2e.PodStartTimeout)

		g.By(fmt.Sprintf("Creating block VolumeSnapshot %q from source PVC using image snapshot class", snapName))
		err = createSnapshot(oc, ns, snapName, pvcName, customVolumeSnapshotClass)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteVolumeSnapshot(ctx, oc, ns, snapName)

		g.By("Waiting for block VolumeSnapshot readyToUse")
		waitForSnapshotReady(oc, snapName)

		g.By("Creating restore block PVC from VolumeSnapshot")
		_, err = createTestPVC(ctx, oc, ns, restorePVC, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeBlock), snapshotDataSource(snapName)))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePVC(ctx, oc, ns, restorePVC)

		g.By("Creating block reader Pod to read back and compare block test data")
		readerPod := newBusyBoxBlockPod(ns, "gcp-blk-reader", restorePVC, blockDev,
			fmt.Sprintf("ACTUAL=$(dd if=%s bs=512 count=1 2>/dev/null | tr -d '\\0') && echo \"Read data: $ACTUAL\" && if echo \"$ACTUAL\" | grep -q \"%s\"; then echo 'Data verification successful'; exit 0; else echo 'Data verification failed'; exit 1; fi", blockDev, testData))
		_, err = createTestPodFromSpec(ctx, oc, readerPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, readerPod.Name, metav1.DeleteOptions{})

		g.By("Waiting for restored block PVC to reach Bound")
		waitPVCBound(ctx, oc, ns, restorePVC)

		g.By("Waiting for reader Pod to complete successfully")
		waitPodSucceeded(ctx, oc, ns, readerPod.Name, e2e.PodStartTimeout)
	})

	g.It("should support multiple point-in-time image snapshots with independent restores using same source PVC", func(ctx g.SpecContext) {
		ns := oc.Namespace()
		dc = oc.AdminDynamicClient()
		customVolumeSnapshotClass := fmt.Sprintf("%s-%d", gcpE2EImagesVSCPrefix, time.Now().UnixNano())

		g.By(fmt.Sprintf("Creating test VolumeSnapshotClass %q with snapshot-type images", customVolumeSnapshotClass))
		createImagesVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)
		defer deleteVolumeSnapshotClass(ctx, dc, customVolumeSnapshotClass)

		var (
			pvcName      = "test-pvc-multi-snapshots"
			snap1        = "test-snapshot-images-1"
			snap2        = "test-snapshot-images-2"
			testData1    = fmt.Sprintf("first-snapshot-data-%d", time.Now().UnixNano())
			testData2    = fmt.Sprintf("second-snapshot-data-%d", time.Now().UnixNano())
			restorePVC1  = "pvc-restore-1"
			restorePVC2  = "pvc-restore-2"
			testFileName = "/mnt/test/testfile.txt"
		)

		g.By("Creating shared source RWO PVC for sequential writes and snapshots")
		_, err := createTestPVC(ctx, oc, ns, pvcName, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeFilesystem), nil))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePVC(ctx, oc, ns, pvcName)

		g.By("Writing first dataset, then deleting writer pod before first snapshot")
		writerPod1 := newBusyBoxWriterPod(ns, "gcp-multi-writer1", pvcName, fmt.Sprintf(`set -e; echo -n '%s' > %s`, testData1, testFileName))
		_, err = createTestPodFromSpec(ctx, oc, writerPod1)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for source PVC to reach Bound")
		waitPVCBound(ctx, oc, ns, pvcName)

		g.By("Waiting for writer pod1 to complete")
		waitPodSucceeded(ctx, oc, ns, writerPod1.Name, e2e.PodStartTimeout)

		g.By("Deleting writer pod1")
		o.Expect(oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, writerPod1.Name, metav1.DeleteOptions{})).NotTo(o.HaveOccurred())
		waitUntilPodRemoved(ctx, oc, ns, writerPod1.Name)

		g.By(fmt.Sprintf("Creating first image VolumeSnapshot %q from source PVC", snap1))
		err = createSnapshot(oc, ns, snap1, pvcName, customVolumeSnapshotClass)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteVolumeSnapshot(ctx, oc, ns, snap1)

		g.By("Waiting for first VolumeSnapshot to be readyToUse")
		waitForSnapshotReady(oc, snap1)

		g.By("Writing second dataset, then deleting writer pod before second snapshot")
		writerPod2 := newBusyBoxWriterPod(ns, "gcp-multi-writer2", pvcName, fmt.Sprintf(`set -e; echo -n '%s' > %s`, testData2, testFileName))
		_, err = createTestPodFromSpec(ctx, oc, writerPod2)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for writer pod2 to complete")
		waitPodSucceeded(ctx, oc, ns, writerPod2.Name, e2e.PodStartTimeout)

		g.By("Deleting writer pod2")
		o.Expect(oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, writerPod2.Name, metav1.DeleteOptions{})).NotTo(o.HaveOccurred())
		waitUntilPodRemoved(ctx, oc, ns, writerPod2.Name)

		g.By(fmt.Sprintf("Creating second image VolumeSnapshot %q from source PVC", snap2))
		err = createSnapshot(oc, ns, snap2, pvcName, customVolumeSnapshotClass)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteVolumeSnapshot(ctx, oc, ns, snap2)

		g.By("Waiting for second VolumeSnapshot readyToUse")
		waitForSnapshotReady(oc, snap2)

		verifyRestore := func(restorePVCName, snap, expected string) {
			g.By(fmt.Sprintf("Creating restore PVC %q from VolumeSnapshot %q", restorePVCName, snap))
			_, rerr := createTestPVC(ctx, oc, ns, restorePVCName, "10Gi", withGCPPDClaimSpec(ptr.To(v1.PersistentVolumeFilesystem), snapshotDataSource(snap)))
			o.Expect(rerr).NotTo(o.HaveOccurred())
			defer deletePVC(ctx, oc, ns, restorePVCName)

			readerPodName := fmt.Sprintf("reader-pod-%s", restorePVCName)
			readerPod := newBusyBoxReaderPod(ns, readerPodName, restorePVCName,
				fmt.Sprintf(`set -e; test "$(cat '%s')" = '%s'`, testFileName, expected))

			g.By(fmt.Sprintf("Creating reader pod for restore PVC %q", restorePVCName))
			_, rerr = createTestPodFromSpec(ctx, oc, readerPod)
			o.Expect(rerr).NotTo(o.HaveOccurred())
			defer oc.AdminKubeClient().CoreV1().Pods(ns).Delete(ctx, readerPod.Name, metav1.DeleteOptions{})

			g.By(fmt.Sprintf("Waiting for restore PVC %q to reach Bound", restorePVCName))
			waitPVCBound(ctx, oc, ns, restorePVCName)

			g.By(fmt.Sprintf("Verifying restored data for snapshot %q matches expected point-in-time content", snap))
			waitPodSucceeded(ctx, oc, ns, readerPod.Name, e2e.PodStartTimeout)
		}

		g.By("Verifying restored data for first snapshot")
		verifyRestore(restorePVC1, snap1, testData1)

		g.By("Verifying restored data for second snapshot")
		verifyRestore(restorePVC2, snap2, testData2)
	})
})

func gcpPDDefaultStorageClass() string {
	provider := strings.ToLower(e2e.TestContext.Provider)
	if cfg, ok := Platforms[provider]; ok && cfg.DefaultStorageClass != "" {
		return cfg.DefaultStorageClass
	}
	return "standard-csi"
}

func withGCPPDClaimSpec(volumeMode *v1.PersistentVolumeMode, dataSource *v1.TypedLocalObjectReference) func(*v1.PersistentVolumeClaim) {
	return func(pvc *v1.PersistentVolumeClaim) {
		sc := gcpPDDefaultStorageClass()
		pvc.Spec.StorageClassName = &sc
		pvc.Spec.DataSource = dataSource
		if volumeMode != nil {
			pvc.Spec.VolumeMode = volumeMode
		}
	}
}

func snapshotDataSource(snapshotName string) *v1.TypedLocalObjectReference {
	apiGroup := "snapshot.storage.k8s.io"
	return &v1.TypedLocalObjectReference{
		Kind:     "VolumeSnapshot",
		Name:     snapshotName,
		APIGroup: &apiGroup,
	}
}

// createImagesVolumeSnapshotClass installs a cluster VolumeSnapshotClass with 'snapshot-type: "images"' for the GCP PD CSI driver
func createImagesVolumeSnapshotClass(ctx context.Context, dc dynamic.Interface, name string) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion":     "snapshot.storage.k8s.io/v1",
			"kind":           "VolumeSnapshotClass",
			"driver":         gcpPDCSIProvisioner,
			"deletionPolicy": "Delete",
			"parameters": map[string]interface{}{
				"snapshot-type": "images",
			},
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
	_, err := dc.Resource(gcpSnapshotClassGVR).Create(ctx, obj, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteVolumeSnapshotClass(ctx context.Context, dc dynamic.Interface, name string) {
	err := dc.Resource(gcpSnapshotClassGVR).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitPVCBound(ctx context.Context, oc *exutil.CLI, ns, pvcName string) {
	o.Eventually(func() v1.PersistentVolumeClaimPhase {
		p, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(ns).Get(ctx, pvcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		return p.Status.Phase
	}).WithTimeout(e2e.ClaimProvisionTimeout).WithPolling(e2e.Poll).Should(o.Equal(v1.ClaimBound))
}

func deletePVC(ctx context.Context, oc *exutil.CLI, ns, name string) {
	_ = oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

func newBusyBoxWriterPod(ns, name, pvcName, command string) *v1.Pod {
	allowPriv := false
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{{
				Name:    "write",
				Image:   k8simage.GetE2EImage(k8simage.BusyBox),
				Command: []string{"/bin/sh", "-c", command},
				VolumeMounts: []v1.VolumeMount{{
					Name: "vol", MountPath: "/mnt/test",
				}},
				SecurityContext: lowRiskSecctx(&allowPriv),
			}},
			Volumes: []v1.Volume{{
				Name: "vol",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
		},
	}
}

func newBusyBoxReaderPod(ns, name, pvcName, command string) *v1.Pod {
	allowPriv := false
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{{
				Name:    "read",
				Image:   k8simage.GetE2EImage(k8simage.BusyBox),
				Command: []string{"/bin/sh", "-c", command},
				VolumeMounts: []v1.VolumeMount{{
					Name: "vol", MountPath: "/mnt/test",
				}},
				SecurityContext: lowRiskSecctx(&allowPriv),
			}},
			Volumes: []v1.Volume{{
				Name: "vol",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
		},
	}
}

func newBusyBoxBlockPod(ns, name, pvcName, devicePath, command string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{{
				Name:    "blk",
				Image:   k8simage.GetE2EImage(k8simage.BusyBox),
				Command: []string{"/bin/sh", "-c", command},
				SecurityContext: &v1.SecurityContext{
					Privileged: ptr.To(true),
				},
				VolumeDevices: []v1.VolumeDevice{{
					Name:       "vol",
					DevicePath: devicePath,
				}},
			}},
			Volumes: []v1.Volume{{
				Name: "vol",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
		},
	}
}

func lowRiskSecctx(allow *bool) *v1.SecurityContext {
	return &v1.SecurityContext{
		AllowPrivilegeEscalation: allow,
		SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
		Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
	}
}

func waitPodSucceeded(ctx context.Context, oc *exutil.CLI, ns, name string, timeout time.Duration) {
	o.Eventually(func() v1.PodPhase {
		p, err := oc.AdminKubeClient().CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if p.Status.Phase == v1.PodFailed {
			logs, _ := oc.Run("logs").Args(name).Output()
			e2e.Failf("pod %s/%s failed: reason=%q message=%q logs=%s", ns, name, p.Status.Reason, p.Status.Message, logs)
		}
		return p.Status.Phase
	}).WithTimeout(timeout).WithPolling(e2e.Poll).Should(o.Equal(v1.PodSucceeded))
}

func waitUntilPodRemoved(ctx context.Context, oc *exutil.CLI, ns, name string) {
	err := wait.PollUntilContextTimeout(ctx, e2e.Poll, e2e.PodDeleteTimeout, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForSnapshotReady(oc *exutil.CLI, snapshotName string) {
	o.Eventually(func() bool {
		ready, err := isSnapshotReady(oc, snapshotName)
		if err != nil {
			msg, merr := getSnapshotErrorMessage(oc, snapshotName)
			if merr != nil {
				e2e.Failf("checking snapshot %s: %v; could not read status error: %v", snapshotName, err, merr)
			}
			e2e.Failf("checking snapshot %s: %v; status error message: %q", snapshotName, err, msg)
		}
		if ready {
			return true
		}
		msg, err := getSnapshotErrorMessage(oc, snapshotName)
		if err != nil {
			e2e.Failf("snapshot %s not ready; could not read error message: %v", snapshotName, err)
		}
		if msg != "" {
			e2e.Failf("snapshot %s not ready: %s", snapshotName, msg)
		}
		return false
	}).WithTimeout(gcpSnapshotPollTimeout).WithPolling(gcpSnapshotPollInterval).Should(o.BeTrue(), "snapshot should reach readyToUse")
}

func snapshotBoundContentName(ctx context.Context, dc dynamic.Interface, ns, snapName string) string {
	obj, err := dc.Resource(gcpSnapshotGVR).Namespace(ns).Get(ctx, snapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	name, found, err := unstructured.NestedString(obj.Object, "status", "boundVolumeSnapshotContentName")
	o.Expect(err).NotTo(o.HaveOccurred())
	if !found || name == "" {
		return ""
	}
	return name
}

func deleteVolumeSnapshot(_ context.Context, oc *exutil.CLI, ns, name string) {
	_ = oc.AsAdmin().Run("delete").Args("volumesnapshot", name, "-n", ns, "--ignore-not-found=true").Execute()
}

func waitUntilVolumeSnapshotDeleted(ctx context.Context, dc dynamic.Interface, ns, name string) {
	o.Eventually(func() bool {
		_, err := dc.Resource(gcpSnapshotGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		return false
	}).WithTimeout(e2e.SnapshotDeleteTimeout).WithPolling(e2e.Poll).Should(o.BeTrue())
}

func waitUntilSnapshotContentDeleted(ctx context.Context, dc dynamic.Interface, name string) {
	o.Eventually(func() bool {
		_, err := dc.Resource(gcpSnapshotContentGVR).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		return false
	}).WithTimeout(e2e.SnapshotDeleteTimeout).WithPolling(e2e.Poll).Should(o.BeTrue())
}
