package csi

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	rwxExpandResizeWaitPeriod = 10 * time.Minute
	azureFileCSIDriver        = "file.csi.azure.com"
	rwxExpandDelta            = "5Gi"
)

func initVolumeExpandRWXCSISuite() storageframework.TestSuite {
	return &volumeExpandRWXCSISuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "OpenShift CSI extended - RWX volume expand",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPVAllowExpansion,
			},
			SupportedSizeRange: e2evolume.SizeRange{
				Min: "1Gi",
			},
		},
	}
}

// volumeExpandRWXCSISuite covers online filesystem expansion of a ReadWriteMany
// volume while multiple pods on different nodes hold the mount.
type volumeExpandRWXCSISuite struct {
	tsInfo storageframework.TestSuiteInfo
}

var _ storageframework.TestSuite = &volumeExpandRWXCSISuite{}

func (s *volumeExpandRWXCSISuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return s.tsInfo
}

func (s *volumeExpandRWXCSISuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) string {
	dInfo := driver.GetDriverInfo()
	if dInfo.Name != azureFileCSIDriver {
		return fmt.Sprintf("RWX volume expand is only enabled for %s (got %q) - skipping", azureFileCSIDriver, dInfo.Name)
	}
	return ""
}

func (s *volumeExpandRWXCSISuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	f := e2e.NewFrameworkWithCustomTimeouts("csi-rwx-expand", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	g.It("should resize filesystem volume while mounted ReadWriteMany from multiple nodes", func(ctx context.Context) {
		config := driver.PrepareTest(ctx, f)
		cs := f.ClientSet

		if config.ClientNodeSelection.Name != "" {
			e2eskipper.Skipf("Driver %q requires a specific node - skipping", driver.GetDriverInfo().Name)
		}
		nodes, err := e2enode.GetReadySchedulableNodes(ctx, cs)
		e2e.ExpectNoError(err)
		if len(nodes.Items) < 2 {
			e2eskipper.Skipf("Need at least 2 schedulable nodes for multi-node RWX expand - skipping")
		}

		g.By("Creating a dynamically provisioned RWX volume")
		volResource := storageframework.CreateVolumeResourceWithAccessModes(
			ctx,
			driver,
			config,
			pattern,
			s.GetTestSuiteInfo().SupportedSizeRange,
			[]v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			nil,
		)
		g.DeferCleanup(volResource.CleanupResource)

		nodeSelection := config.ClientNodeSelection
		var pods []*v1.Pod
		for i := 0; i < 2; i++ {
			g.By(fmt.Sprintf("Creating pod%d with the RWX volume", i+1))
			podConfig := e2epod.Config{
				NS:            f.Namespace.Name,
				PVCs:          []*v1.PersistentVolumeClaim{volResource.Pvc},
				SeLinuxLabel:  e2epod.GetLinuxLabel(),
				NodeSelection: nodeSelection,
				ImageID:       e2epod.GetTestImageID(imageutils.GlibcDnsTesting),
			}
			pod, err := e2epod.CreateSecPodWithNodeSelection(ctx, cs, &podConfig, f.Timeouts.PodStart)
			e2e.ExpectNoError(err, "creating pod%d", i+1)
			g.DeferCleanup(e2epod.DeletePodWithWait, cs, pod)
			pods = append(pods, pod)
			e2epod.SetAntiAffinity(&nodeSelection, pod.Spec.NodeName)
		}

		seed := time.Now().UTC().UnixNano()
		const byteLen = 64

		g.By("Writing test data from pod1 and verifying RWX read from pod2")
		storageutils.CheckWriteToPath(ctx, f, pods[0], pattern.VolMode, false, e2epod.VolumeMountPath1, byteLen, seed)
		storageutils.CheckReadFromPath(ctx, f, pods[1], pattern.VolMode, false, e2epod.VolumeMountPath1, byteLen, seed)

		g.By("Re-fetching PVC to get latest capacity before expansion")
		pvc, err := cs.CoreV1().PersistentVolumeClaims(volResource.Pvc.Namespace).Get(ctx, volResource.Pvc.Name, metav1.GetOptions{})
		e2e.ExpectNoError(err, "re-fetching PVC before expansion")
		volResource.Pvc = pvc
		originalSize := pvc.Status.Capacity[v1.ResourceStorage]
		newSize := originalSize.DeepCopy()
		newSize.Add(resource.MustParse(rwxExpandDelta))
		e2e.Logf("originalSize %v, newSize %v", originalSize, newSize)

		g.By("Recording mounted capacity before expansion")
		dfBefore, err := mountedFilesystemSize(ctx, f, pods[0], e2epod.VolumeMountPath1)
		e2e.ExpectNoError(err, "df before expand")

		g.By("Expanding PVC while both pods are using it")
		volResource.Pvc, err = testsuites.ExpandPVCSize(ctx, volResource.Pvc, newSize, cs)
		e2e.ExpectNoError(err, "expanding PVC")

		g.By("Waiting for controller volume resize to finish")
		e2e.ExpectNoError(testsuites.WaitForControllerVolumeResize(ctx, volResource.Pvc, cs, rwxExpandResizeWaitPeriod))

		g.By("Waiting for filesystem resize to finish")
		volResource.Pvc, err = testsuites.WaitForFSResize(ctx, volResource.Pvc, cs)
		e2e.ExpectNoError(err, "waiting for filesystem resize")
		testsuites.ExpectNoResizeConditions(volResource.Pvc.Status.Conditions)

		g.By("Verifying pre-expand data is still readable from both pods")
		storageutils.CheckReadFromPath(ctx, f, pods[0], pattern.VolMode, false, e2epod.VolumeMountPath1, byteLen, seed)
		storageutils.CheckReadFromPath(ctx, f, pods[1], pattern.VolMode, false, e2epod.VolumeMountPath1, byteLen, seed)

		// CIFS can keep reporting the old size/free space until the attribute cache expires.
		// So poll the mount before writing past the original capacity.
		g.By("Waiting for mounted capacity to grow on pod1")
		e2e.Gomega().Eventually(ctx, func(ctx context.Context) error {
			dfAfter, err := mountedFilesystemSize(ctx, f, pods[0], e2epod.VolumeMountPath1)
			if err != nil {
				return err
			}
			if dfAfter <= dfBefore {
				return fmt.Errorf("df size has not grown yet (before=%dG after=%dG)", dfBefore, dfAfter)
			}
			return nil
		}).WithTimeout(3 * time.Minute).WithPolling(5 * time.Second).Should(o.Succeed())

		fallocateBytes := originalSize.DeepCopy()
		fallocateBytes.Add(resource.MustParse("1Gi"))
		g.By(fmt.Sprintf("Writing %s past original capacity with fallocate", fallocateBytes.String()))
		err = e2epod.VerifyExecInPodSucceed(ctx, f, pods[0],
			fmt.Sprintf("fallocate -l %d %s/expand-check", fallocateBytes.Value(), e2epod.VolumeMountPath1))
		e2e.ExpectNoError(err, "fallocate past original capacity")
	})
}

// mountedFilesystemSize returns the size column from `df -BG` for path inside pod.
func mountedFilesystemSize(ctx context.Context, f *e2e.Framework, pod *v1.Pod, path string) (int64, error) {
	stdout, stderr, err := e2epod.ExecShellInPodWithFullOutput(ctx, f, pod.Name,
		fmt.Sprintf("df -BG --output=size %q | tail -n1", path))
	if err != nil {
		return 0, fmt.Errorf("df in pod %s: %w (stderr=%q)", pod.Name, err, stderr)
	}
	var size int64
	if _, err := fmt.Sscanf(strings.TrimSpace(stdout), "%dG", &size); err != nil {
		return 0, fmt.Errorf("parse df size %q: %w", stdout, err)
	}
	return size, nil
}
