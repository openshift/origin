package csi

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
	admissionapi "k8s.io/pod-security-admission/api"
)

func initPVCCloneLargerCSISuite() storageframework.TestSuite {
	return &pvcCloneLargerCSISuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "OpenShift CSI extended - CSI Clone",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPV,
				storageframework.BlockVolModeDynamicPV,
			},
			SupportedSizeRange: e2evolume.SizeRange{
				Min: "1Mi",
			},
		},
	}
}

// pvcCloneLargerCSISuite covers cloning a PVC into a larger destination volume
// for both filesystem and raw block volume modes.
type pvcCloneLargerCSISuite struct {
	tsInfo storageframework.TestSuiteInfo
}

var _ storageframework.TestSuite = &pvcCloneLargerCSISuite{}

func (s *pvcCloneLargerCSISuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return s.tsInfo
}

func (s *pvcCloneLargerCSISuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) string {
	dInfo := driver.GetDriverInfo()
	if !dInfo.Capabilities[storageframework.CapPVCDataSource] {
		return fmt.Sprintf("Driver %q does not support cloning - skipping", dInfo.Name)
	}
	if pattern.VolMode == v1.PersistentVolumeBlock && !dInfo.Capabilities[storageframework.CapBlock] {
		return fmt.Sprintf("Driver %s doesn't support %v -- skipping", dInfo.Name, pattern.VolMode)
	}
	if pattern.VolMode != v1.PersistentVolumeBlock && dInfo.Capabilities[storageframework.CapFSResizeFromSourceNotSupported] {
		return fmt.Sprintf("Driver %q does not support filesystem resizing from source - skipping", dInfo.Name)
	}
	return ""
}

func (s *pvcCloneLargerCSISuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	f := e2e.NewFrameworkWithCustomTimeouts("csi-clone-larger", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	dInfo := driver.GetDriverInfo()

	g.It("should provision volume with pvc data source larger than original volume", func(ctx context.Context) {
		dDriver, ok := driver.(storageframework.DynamicPVTestDriver)
		if !ok {
			e2eskipper.Skipf("Driver %q does not support dynamic provisioning - skipping", dInfo.Name)
		}

		config := driver.PrepareTest(ctx, f)
		cs := config.Framework.ClientSet

		// Some drivers cannot clone across topology segments; pin source and clone to one.
		pinToDriverTopology(ctx, &config.ClientNodeSelection, cs, dInfo)

		testConfig := storageframework.ConvertTestConfig(config)
		expectedContent := fmt.Sprintf("Hello from namespace %s", f.Namespace.Name)
		contentTest := func(pvcName string) e2evolume.Test {
			return e2evolume.Test{
				Volume:          *storageutils.CreateVolumeSource(pvcName, false /* readOnly */),
				Mode:            pattern.VolMode,
				File:            "index.html",
				ExpectedContent: expectedContent,
			}
		}

		claimSize, err := storageutils.GetSizeRangesIntersection(s.GetTestSuiteInfo().SupportedSizeRange, dInfo.SupportedSizeRange)
		e2e.ExpectNoError(err, "determine intersection of test and driver size ranges")

		sc := dDriver.GetDynamicProvisionStorageClass(ctx, config, pattern.FsType)
		if sc == nil {
			e2eskipper.Skipf("Driver %q does not define Dynamic Provision StorageClass - skipping", dInfo.Name)
		}

		g.By("Creating StorageClass and source PVC with test data")
		testsuites.SetupStorageClass(ctx, cs, sc)
		sourcePVC, err := cs.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx,
			e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
				ClaimSize:        claimSize,
				StorageClassName: &sc.Name,
				VolumeMode:       &pattern.VolMode,
			}, f.Namespace.Name),
			metav1.CreateOptions{})
		e2e.ExpectNoError(err)
		g.DeferCleanup(func(ctx context.Context) {
			_ = cs.CoreV1().PersistentVolumeClaims(sourcePVC.Namespace).Delete(ctx, sourcePVC.Name, metav1.DeleteOptions{})
		})
		e2evolume.InjectContent(ctx, f, testConfig, nil, "", []e2evolume.Test{contentTest(sourcePVC.Name)})

		g.By("Recording source PVC capacity and requesting a larger clone")
		sourcePVC, err = cs.CoreV1().PersistentVolumeClaims(sourcePVC.Namespace).Get(ctx, sourcePVC.Name, metav1.GetOptions{})
		e2e.ExpectNoError(err, "Failed to get source PVC: %v", err)
		storageRequest := resource.NewQuantity(sourcePVC.Status.Capacity.Storage().Value(), resource.BinarySI)
		storageRequest.Add(resource.MustParse("1Gi"))
		largerSize := storageRequest.String()

		clonePVC := sourcePVC.DeepCopy()
		clonePVC.ObjectMeta = metav1.ObjectMeta{
			GenerateName: "pvc-",
			Namespace:    sourcePVC.Namespace,
		}
		clonePVC.Status = v1.PersistentVolumeClaimStatus{}
		clonePVC.Spec.VolumeName = ""
		clonePVC.Spec.Resources.Requests = v1.ResourceList{v1.ResourceStorage: *storageRequest}
		clonePVC.Spec.DataSourceRef = &v1.TypedObjectReference{
			Kind: "PersistentVolumeClaim",
			Name: sourcePVC.Name,
		}

		testCase := &testsuites.StorageClassTest{
			Client:        cs,
			Timeouts:      f.Timeouts,
			Claim:         clonePVC,
			Class:         sc,
			Provisioner:   sc.Provisioner,
			ClaimSize:     largerSize,
			ExpectedSize:  largerSize,
			VolumeMode:    pattern.VolMode,
			NodeSelection: testConfig.ClientNodeSelection,
			PvCheck: func(ctx context.Context, claim *v1.PersistentVolumeClaim) {
				g.By("checking whether the cloned volume has the pre-populated data")
				e2evolume.TestVolumeClientSlow(ctx, f, testConfig, nil, "", []e2evolume.Test{contentTest(claim.Name)})
			},
		}

		// Cloning fails if the source disk is still detaching; wait for VolumeAttachment removal.
		volumeAttachment := e2evolume.GetVolumeAttachmentName(ctx, cs, testConfig, testCase.Provisioner, sourcePVC.Name, sourcePVC.Namespace)
		e2e.ExpectNoError(e2evolume.WaitForVolumeAttachmentTerminated(ctx, volumeAttachment, cs, f.Timeouts.DataSourceProvision))

		g.By("Provisioning clone PVC larger than the source and verifying data")
		testCase.TestDynamicProvisioning(ctx)
	})
}

// pinToDriverTopology constrains pods to one topology segment advertised by the driver.
// Matches the intent of upstream ensureTopologyRequirements for PVC cloning.
func pinToDriverTopology(ctx context.Context, nodeSelection *e2epod.NodeSelection, cs clientset.Interface, dInfo *storageframework.DriverInfo) {
	if nodeSelection.Name != "" || len(dInfo.TopologyKeys) == 0 {
		return
	}
	node, err := e2enode.GetRandomReadySchedulableNode(ctx, cs)
	e2e.ExpectNoError(err)
	topo := make(map[string]string, len(dInfo.TopologyKeys))
	for _, key := range dInfo.TopologyKeys {
		if val, ok := node.Labels[key]; ok && val != "" {
			topo[key] = val
		}
	}
	if len(topo) > 0 {
		e2epod.SetNodeAffinityTopologyRequirement(nodeSelection, topo)
	}
}
