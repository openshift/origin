package storage

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	opv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"gopkg.in/ini.v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/ptr"
)

const (
	projectName  = "csi-driver-configuration"
	providerName = "csi.vsphere.vmware.com"
	pollTimeout  = 5 * time.Minute
	pollInterval = 5 * time.Second
)

// This is [Serial] because it modifies ClusterCSIDriver.
var _ = g.Describe("[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration", func() {
	defer g.GinkgoRecover()
	var (
		oc                       = exutil.NewCLI(projectName)
		originalDriverConfigSpec *opv1.CSIDriverConfigSpec
		operatorShouldProgress   bool
	)

	g.BeforeEach(func(ctx g.SpecContext) {
		if !framework.ProviderIs("vsphere") {
			g.Skip("this test is only expected to work with vSphere clusters")
		}

		originalClusterCSIDriver, err := oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Get(ctx, providerName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		originalDriverConfigSpec = originalClusterCSIDriver.Spec.DriverConfig.DeepCopy()
		e2e.Logf("Storing original driverConfig of ClusterCSIDriver")
	})

	g.AfterEach(func(ctx g.SpecContext) {
		if originalDriverConfigSpec == nil {
			return
		}

		e2e.Logf("Restoring original driverConfig of ClusterCSIDriver")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			clusterCSIDriver, err := oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Get(ctx, providerName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			clusterCSIDriver.Spec.DriverConfig = *originalDriverConfigSpec
			_, err = oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Update(ctx, clusterCSIDriver, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update ClusterCSIDriver")

		// Wait for operator to stop progressing after config restore to ensure all pod creation events complete before
		// test ends. This allows the pathological event matcher (newVsphereConfigurationTestsRollOutTooOftenEventMatcher
		// in pkg/monitortestlibrary/pathologicaleventlibrary/duplicated_event_patterns.go) to accurately attribute
		// pod events to this test's time window (interval); any events emitted later would not be matched.
		if operatorShouldProgress {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			err := exutil.WaitForOperatorProgressingTrue(ctxWithTimeout, oc.AdminConfigClient(), "storage")
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForOperatorProgressingFalse(ctx, oc.AdminConfigClient(), "storage")
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		e2e.Logf("Successfully restored original driverConfig of ClusterCSIDriver")
	})

	g.Context("snapshot options in clusterCSIDriver should", func() {
		var tests = []struct {
			name                       string
			clusterCSIDriverOptions    *opv1.VSphereCSIDriverConfigSpec
			cloudConfigOptions         map[string]string
			successfulSnapshotsCreated int  // Number of snapshots that should be created successfully, 0 to skip.
			operatorShouldProgress     bool // Indicates if we expect to see storage operator change condition to Progressing=True
		}{
			{
				name:                       "use default when unset",
				clusterCSIDriverOptions:    nil,
				cloudConfigOptions:         map[string]string{},
				successfulSnapshotsCreated: 3,
				operatorShouldProgress:     false,
			},
			{
				name: "allow setting global snapshot limit",
				clusterCSIDriverOptions: &opv1.VSphereCSIDriverConfigSpec{
					GlobalMaxSnapshotsPerBlockVolume: ptr.To(uint32(4)),
				},
				cloudConfigOptions: map[string]string{
					"global-max-snapshots-per-block-volume": "4",
				},
				successfulSnapshotsCreated: 4,
				operatorShouldProgress:     true,
			},
			{
				name: "allow setting VSAN limit",
				clusterCSIDriverOptions: &opv1.VSphereCSIDriverConfigSpec{
					GranularMaxSnapshotsPerBlockVolumeInVSAN: ptr.To(uint32(4)),
				},
				cloudConfigOptions: map[string]string{
					"granular-max-snapshots-per-block-volume-vsan": "4",
				},
				successfulSnapshotsCreated: 0,
				operatorShouldProgress:     true,
			},
			{
				name: "allow setting VVOL limit",
				clusterCSIDriverOptions: &opv1.VSphereCSIDriverConfigSpec{
					GranularMaxSnapshotsPerBlockVolumeInVVOL: ptr.To(uint32(4)),
				},
				cloudConfigOptions: map[string]string{
					"granular-max-snapshots-per-block-volume-vvol": "4",
				},
				successfulSnapshotsCreated: 0,
				operatorShouldProgress:     true,
			},
			{
				name: "allow all limits to be set at once",
				clusterCSIDriverOptions: &opv1.VSphereCSIDriverConfigSpec{
					GlobalMaxSnapshotsPerBlockVolume:         ptr.To(uint32(5)),
					GranularMaxSnapshotsPerBlockVolumeInVSAN: ptr.To(uint32(10)),
					GranularMaxSnapshotsPerBlockVolumeInVVOL: ptr.To(uint32(15)),
				},
				cloudConfigOptions: map[string]string{
					"global-max-snapshots-per-block-volume":        "5",
					"granular-max-snapshots-per-block-volume-vsan": "10",
					"granular-max-snapshots-per-block-volume-vvol": "15",
				},
				successfulSnapshotsCreated: 0,
				operatorShouldProgress:     true,
			},
		}

		for _, t := range tests {
			t := t
			g.It(t.name, func(ctx g.SpecContext) {
				defer g.GinkgoRecover()
				operatorShouldProgress = t.operatorShouldProgress

				setClusterCSIDriverSnapshotOptions(ctx, oc, t.clusterCSIDriverOptions)

				if operatorShouldProgress {
					// Wait for Progressing=True within 10 seconds
					{
						ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
						defer cancel()
						err := exutil.WaitForOperatorProgressingTrue(ctxWithTimeout, oc.AdminConfigClient(), "storage")
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					// Then wait for Progressing=False within next 10 seconds
					{
						ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
						defer cancel()
						err := exutil.WaitForOperatorProgressingFalse(ctxWithTimeout, oc.AdminConfigClient(), "storage")
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}

				o.Eventually(func() error {
					return loadAndCheckCloudConf(ctx, oc, "Snapshot", t.cloudConfigOptions, t.clusterCSIDriverOptions)
				}, pollTimeout, pollInterval).Should(o.Succeed())

				validateSnapshotCreation(ctx, oc, t.successfulSnapshotsCreated)
			})
		}
	})
})

func setClusterCSIDriverSnapshotOptions(ctx context.Context, oc *exutil.CLI, clusterCSIDriverOptions *opv1.VSphereCSIDriverConfigSpec) {
	e2e.Logf("updating ClusterCSIDriver driver config to: %+v", clusterCSIDriverOptions)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		clusterCSIDriver, err := oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Get(ctx, providerName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterCSIDriver.Spec.DriverConfig.VSphere = clusterCSIDriverOptions
		_, err = oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Update(ctx, clusterCSIDriver, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to update ClusterCSIDriver")
}

func loadAndCheckCloudConf(ctx context.Context, oc *exutil.CLI, sectionName string, cloudConfigOptions map[string]string, clusterCSIDriverOptions *opv1.VSphereCSIDriverConfigSpec) error {
	e2e.Logf("Validating cloud.conf section %s", sectionName)

	if clusterCSIDriverOptions == nil {
		e2e.Logf("Skipping cloud.conf check, no snapshot options are set for the driver.")
		return nil
	}

	configSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-cluster-csi-drivers").Get(ctx, "vsphere-csi-config-secret", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Secret: %v", err)
	}

	cloudConfData, ok := configSecret.Data["cloud.conf"]
	if !ok {
		return fmt.Errorf("cloud.conf key not found in Secret")
	}

	cfg, err := ini.Load([]byte(cloudConfData))
	if err != nil {
		return fmt.Errorf("failed to load cloud.conf: %v", err)
	}

	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return fmt.Errorf("section %s not found in cloud.conf: %v", sectionName, err)
	}

	if !reflect.DeepEqual(section.KeysHash(), cloudConfigOptions) {
		return fmt.Errorf("check of %s section in cloud.conf failed, got: %v expected: %v", sectionName, section.KeysHash(), cloudConfigOptions)
	}

	e2e.Logf("Validation of %s section in cloud.conf succeeded", sectionName)

	return nil
}

func validateSnapshotCreation(ctx context.Context, oc *exutil.CLI, successfulSnapshotsCreated int) {
	e2e.Logf("Validating snapshot creation.")

	if successfulSnapshotsCreated == 0 {
		e2e.Logf("Skipping snapshot validation, successfulSnapshotsCreated is set to 0 for this test.")
		return
	}

	pvc, err := createTestPVC(ctx, oc, oc.Namespace(), "test-pvc", "1Gi")
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
	}()

	pod, err := createTestPod(ctx, oc, pvc.Name, oc.Namespace())
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()

	// Wait for pvc to be bound.
	o.Eventually(func() v1.PersistentVolumeClaimPhase {
		pvc, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(oc.Namespace()).Get(ctx, "test-pvc", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		return pvc.Status.Phase
	}, pollTimeout, pollInterval).Should(o.Equal(v1.ClaimBound))

	var wg sync.WaitGroup
	var snapshotsCreated = make([]string, 0, successfulSnapshotsCreated)
	for i := 0; i < successfulSnapshotsCreated; i++ {
		wg.Add(1)
		snapshotName := fmt.Sprintf("test-snapshot-%d", i)
		snapshotsCreated = append(snapshotsCreated, snapshotName)
		go func(snapshotName string) {
			defer wg.Done()
			err := createSnapshot(oc, oc.Namespace(), snapshotName, "test-pvc")
			if err != nil {
				e2e.Failf("failed to create snapshot: %v", err)
				return
			}
		}(snapshotName)
	}
	wg.Wait()

	defer func() {
		for _, snapshotName := range snapshotsCreated {
			oc.AsAdmin().Run("delete").Args("volumesnapshot", snapshotName).Execute()
		}
	}()

	// Wait for snapshots to be readyToUse.
	o.Eventually(func() int {
		snapshotsReady := 0
		for _, snapshotName := range snapshotsCreated {
			if ready, _ := isSnapshotReady(oc, snapshotName); ready {
				e2e.Logf("Snapshot %s is ready", snapshotName)
				snapshotsReady++
			} else {
				e2e.Logf("Snapshot %s is not ready yet", snapshotName)
			}
		}
		e2e.Logf("Snapshots ready: %d/%d", snapshotsReady, successfulSnapshotsCreated)
		return snapshotsReady
	}, pollTimeout, pollInterval).Should(o.Equal(successfulSnapshotsCreated), "not all snapshots are ready")

	// Next snapshot creation should be over the set limit and fail.
	failedSnapshotName := "test-snapshot-failed"
	err = createSnapshot(oc, oc.Namespace(), failedSnapshotName, "test-pvc")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create snapshot")

	e2e.Logf("Validating that snapshot creation now fails since the limit is reached.")

	o.Eventually(func() bool {
		ready := false
		errMsg := ""
		if ready, err = isSnapshotReady(oc, failedSnapshotName); err != nil {
			return false
		}
		if errMsg, err = getSnapshotErrorMessage(oc, failedSnapshotName); err != nil || errMsg == "" {
			return false
		}
		e2e.Logf("Error validation successful - snapshot: %s readyToUse: %t, error message: %s", failedSnapshotName, ready, errMsg)
		return strings.Contains(errMsg, "reaches the configured maximum") && !ready
	}, pollTimeout, pollInterval).Should(o.BeTrue(), "snapshot creation should fail")
}

func createTestPod(ctx context.Context, oc *exutil.CLI, pvcName string, namespace string) (*v1.Pod, error) {
	allowPrivEsc := false
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-driver-conf",
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test",
					Image: k8simage.GetE2EImage(k8simage.BusyBox),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "pvc-data",
							MountPath: "/mnt",
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
					Name: "pvc-data",
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

func createTestPVC(ctx context.Context, oc *exutil.CLI, namespace string, pvcName string, volumeSize string) (*v1.PersistentVolumeClaim, error) {
	e2e.Logf("Creating PVC %s in namespace %s with size %s", pvcName, namespace, volumeSize)

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse(volumeSize),
				},
			},
		},
	}

	return oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func createSnapshot(oc *exutil.CLI, namespace string, snapshotName string, pvcName string) error {
	e2e.Logf("Creating snapshot %s for PVC %s in namespace %s", snapshotName, pvcName, namespace)

	snapshot := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
 name: %s
 namespace: %s
spec:
 source:
   persistentVolumeClaimName: %s
`, snapshotName, namespace, pvcName)

	err := oc.AsAdmin().Run("apply").Args("-f", "-").InputString(snapshot).Execute()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %v", err)
	}

	return nil
}

func isSnapshotReady(oc *exutil.CLI, snapshotName string) (bool, error) {
	readyToUse, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshotName), "-o", "jsonpath={.status.readyToUse}").Output()
	if err != nil {
		e2e.Logf("failed to get snapshot readyToUse: %v", err)
		return false, err
	}

	return readyToUse == "true", nil
}

func getSnapshotErrorMessage(oc *exutil.CLI, snapshotName string) (string, error) {
	errMsg, err := oc.Run("get").Args(fmt.Sprintf("volumesnapshot/%s", snapshotName), "-o", "jsonpath={.status.error.message}").Output()
	if err != nil {
		e2e.Logf("failed to get snapshot error message: %v", err)
		return "", err
	}

	return errMsg, nil
}
