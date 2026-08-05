package csi

import (
	"context"
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
	admissionapi "k8s.io/pod-security-admission/api"
)

// CapPodDeleteAfterUmount indicates the driver supports pod deletion after the volume
// was force-unmounted on the node.
const CapPodDeleteAfterUmount OpenShiftCSICapability = "podDeleteAfterUmount"

func initPodDeleteAfterUmountCSISuite() storageframework.TestSuite {
	return &podDeleteAfterUmountCSISuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "OpenShift CSI extended - Pod delete after umount",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPV,
			},
			SupportedSizeRange: e2evolume.SizeRange{
				Min: "1Mi",
			},
		},
	}
}

// podDeleteAfterUmountCSISuite verifies that a pod can be deleted after its CSI
// volume mount has already been unmounted on the node.
type podDeleteAfterUmountCSISuite struct {
	tsInfo storageframework.TestSuiteInfo
}

var _ storageframework.TestSuite = &podDeleteAfterUmountCSISuite{}

func (s *podDeleteAfterUmountCSISuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return s.tsInfo
}

func (s *podDeleteAfterUmountCSISuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	cfg, ok := OpenShiftCSIDriverConfigFor(driver.GetDriverInfo().Name)
	if !ok || !cfg.Capabilities[CapPodDeleteAfterUmount] {
		e2eskipper.Skipf("Driver %q does not support pod delete after umount - skipping", driver.GetDriverInfo().Name)
	}
}

func (s *podDeleteAfterUmountCSISuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	f := e2e.NewFrameworkWithCustomTimeouts("csi-pod-delete-umount", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	g.It("should delete pod after volume directory was umounted on the node", func(ctx context.Context) {
		config := driver.PrepareTest(ctx, f)
		hostExec := storageutils.NewHostExec(f)
		g.DeferCleanup(hostExec.Cleanup)

		g.By("Creating a dynamically provisioned volume")
		resource := storageframework.CreateVolumeResource(ctx, driver, config, pattern, s.GetTestSuiteInfo().SupportedSizeRange)
		g.DeferCleanup(resource.CleanupResource)

		g.By("Creating a pod that mounts the volume")
		podConfig := e2epod.Config{
			NS:            f.Namespace.Name,
			PVCs:          []*v1.PersistentVolumeClaim{resource.Pvc},
			SeLinuxLabel:  e2epod.GetLinuxLabel(),
			NodeSelection: config.ClientNodeSelection,
			ImageID:       e2epod.GetDefaultTestImageID(),
		}
		pod, err := e2epod.CreateSecPodWithNodeSelection(ctx, f.ClientSet, &podConfig, f.Timeouts.PodStart)
		e2e.ExpectNoError(err, "creating pod with PVC")
		g.DeferCleanup(e2epod.DeletePodWithWait, f.ClientSet, pod)

		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(resource.Pvc.Namespace).Get(ctx, resource.Pvc.Name, metav1.GetOptions{})
		e2e.ExpectNoError(err, "re-fetching PVC after pod is running")
		pvName := pvc.Spec.VolumeName
		if pvName == "" {
			e2e.Failf("PVC %s has empty Spec.VolumeName after pod is running", pvc.Name)
		}

		node, err := f.ClientSet.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		e2e.ExpectNoError(err, "getting pod node %s", pod.Spec.NodeName)

		mountPath := csiPodVolumeMountPath(string(pod.UID), pvName)
		g.By("Verifying volume is mounted on the pod node")
		err = hostExec.IssueCommand(ctx, fmt.Sprintf("mountpoint -q %q", mountPath), node)
		e2e.ExpectNoError(err, "expected %s to be a mountpoint before umount", mountPath)

		g.By("Unmounting and removing the volume directory on the node")
		err = hostExec.IssueCommand(ctx, fmt.Sprintf("umount -f %q && rmdir %q", mountPath, mountPath), node)
		e2e.ExpectNoError(err, "umount and rmdir of volume mount path %s", mountPath)

		g.By("Verifying the path is no longer a mountpoint")
		err = hostExec.IssueCommand(ctx, fmt.Sprintf("mountpoint -q %q", mountPath), node)
		if err == nil {
			e2e.Failf("expected %s to not be a mountpoint after umount", mountPath)
		}

		g.By("Deleting the pod; TearDown must succeed despite the missing mount [OCPBUGS-10816]")
		err = e2epod.DeletePodWithWait(ctx, f.ClientSet, pod)
		e2e.ExpectNoError(err, "deleting pod after volume directory was umounted")
	})
}

// csiPodVolumeMountPath returns the kubelet CSI NodePublish mount path for a pod volume.
func csiPodVolumeMountPath(podUID, pvName string) string {
	return filepath.Join("/var/lib/kubelet/pods", podUID, "volumes", "kubernetes.io~csi", pvName, "mount")
}
