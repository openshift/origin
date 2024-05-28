package csi

import (
	"context"
	"fmt"
	"math"
	"sync"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	resource2 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	node2 "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

func initSCSILUNOverflowCSISuite(cfg *LUNStressTestConfig) func() storageframework.TestSuite {
	return func() storageframework.TestSuite {
		return &scsiLUNOverflowCSISuite{
			tsInfo: storageframework.TestSuiteInfo{
				Name: "OpenShift CSI extended - SCSI LUN Overflow",
				TestPatterns: []storageframework.TestPattern{
					storageframework.FsVolModeDynamicPV,
				},
			},
			lunStressTestConfig: cfg,
		}
	}
}

// scsiLUNOverflowCSISuite is a test suite for the LUN stress test.
type scsiLUNOverflowCSISuite struct {
	tsInfo              storageframework.TestSuiteInfo
	lunStressTestConfig *LUNStressTestConfig
}

var _ storageframework.TestSuite = &scsiLUNOverflowCSISuite{}

func (csiSuite *scsiLUNOverflowCSISuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return csiSuite.tsInfo
}

func (csiSuite *scsiLUNOverflowCSISuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	return
}

func (csiSuite *scsiLUNOverflowCSISuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	f := e2e.NewFrameworkWithCustomTimeouts("storage-lun-overflow", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	g.It("should use many PVs on a single node [Serial][Timeout:40m]", func(ctx context.Context) {
		if csiSuite.lunStressTestConfig == nil {
			g.Skip("lunStressTestConfig is empty")
		}
		if csiSuite.lunStressTestConfig.PodsTotal == 0 {
			g.Skip("lunStressTestConfig is explicitly disabled")
		}
		e2e.Logf("Starting LUN stress test with config: %+v", csiSuite.lunStressTestConfig)

		driverName := driver.GetDriverInfo().Name
		config := driver.PrepareTest(ctx, f)

		g.By("Selecting a schedulable node")
		node, err := node2.GetRandomReadySchedulableNode(ctx, f.ClientSet)
		e2e.ExpectNoError(err, "getting a schedulable node")

		attachLimit, err := getNodeLimitForDriver(ctx, f, node.Name, driverName)
		e2e.ExpectNoError(err, "checking attach limit for node", node.Name)
		e2e.Logf("Discovered attach limit of node %s for driver %s: %d", node.Name, driverName, attachLimit)
		attachLimit = int(math.Floor(float64(attachLimit) * 0.9)) // give 10% of the limit to other PVs

		workerCount := csiSuite.lunStressTestConfig.MaxPodsPerNode
		if attachLimit < workerCount && attachLimit > 0 {
			workerCount = attachLimit
			e2e.Logf("Adjusted nr. of volumes per node to %d, which is 90%% of node attach limit %d", workerCount, attachLimit)
		}

		g.By("Creating a StorageClass")
		sc, err := createSC(ctx, f, driver, config)
		e2e.ExpectNoError(err, "creating StorageClass")
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up StorageClass %s", sc.Name)
			err := f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
			e2e.ExpectNoError(err, "deleting StorageClass", sc.Name)
		})

		e2e.Logf("Starting %d workers", workerCount)
		work := make(chan int, csiSuite.lunStressTestConfig.PodsTotal)
		wg := sync.WaitGroup{}
		for i := 0; i < workerCount; i++ {
			id := i
			wg.Add(1)
			go func() {
				defer g.GinkgoRecover()
				defer wg.Done()
				worker(ctx, id, work, f, node.Name, config, sc.Name)
			}()
		}
		for i := 0; i < csiSuite.lunStressTestConfig.PodsTotal; i++ {
			work <- i
		}
		e2e.Logf("All work items sent")
		close(work)
		wg.Wait()
		e2e.Logf("All workers finished")
	})
}

// Worker function processes work items from the work channel and creates a PVC + Pod for each of them.
func worker(ctx context.Context, workerID int, work <-chan int, f *e2e.Framework, nodeName string, config *storageframework.PerTestConfig, scName string) {
	for {
		select {
		case <-ctx.Done():
			e2e.Logf("Worker %d context expired", workerID)
			return
		case i, ok := <-work:
			if !ok {
				e2e.Logf("Worker %d all work done", workerID)
				return
			}
			e2e.Logf("Worker %d processing pod %d", workerID, i)
			runTestPod(ctx, f, nodeName, config, scName, i)
		}
	}
}

func getNodeLimitForDriver(ctx context.Context, f *e2e.Framework, nodeName, driverName string) (int, error) {
	csinode, err := f.ClientSet.StorageV1().CSINodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	for _, driver := range csinode.Spec.Drivers {
		if driver.Name == driverName {
			if driver.Allocatable != nil && driver.Allocatable.Count != nil {
				return int(*driver.Allocatable.Count), nil
			}
		}
	}
	return 0, nil
}

// Create one PVC + Pod, wait for the Pod running and delete everything.
func runTestPod(ctx context.Context, f *e2e.Framework, nodeName string, config *storageframework.PerTestConfig, scName string, podNumber int) {
	pvcName := fmt.Sprintf("pvc-%d", podNumber)
	g.By(fmt.Sprintf("Creating PVC %s on node %s", pvcName, nodeName))

	claimSize := config.Driver.GetDriverInfo().SupportedSizeRange.Min
	if claimSize == "" {
		claimSize = "1Gi"
	}
	claimQuantity, err := resource2.ParseQuantity(claimSize)
	e2e.ExpectNoError(err, "parsing claim size %s", claimSize)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: f.Namespace.Name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: claimQuantity,
				},
			},
		},
	}
	pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
	e2e.ExpectNoError(err, "creating PVC %s", pvcName)
	defer func() {
		e2e.Logf("Cleaning up PVC %s", pvcName)
		err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(ctx, pvcName, metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting PVC", pvcName)
	}()

	g.By(fmt.Sprintf("Creating pod %d on node %s", podNumber, nodeName))
	podConfig := &e2epod.Config{
		NS:            f.Namespace.Name,
		PVCs:          []*corev1.PersistentVolumeClaim{pvc},
		NodeSelection: e2epod.NodeSelection{Name: nodeName},
	}
	pod, err := e2epod.CreateSecPod(ctx, f.ClientSet, podConfig, f.Timeouts.DataSourceProvision)
	if pod != nil {
		defer e2epod.DeletePodOrFail(ctx, f.ClientSet, pod.Namespace, pod.Name)
	}
	e2e.ExpectNoError(err, "creating pod %d", podNumber)
	defer func() {
		e2e.Logf("Cleaning up pod %d / %s", podNumber, pod.Name)
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting pod %d / %s", podNumber, pod.Name)
	}()
}

func createSC(ctx context.Context, f *e2e.Framework, driver storageframework.TestDriver, config *storageframework.PerTestConfig) (*storagev1.StorageClass, error) {
	pvTester, ok := driver.(storageframework.DynamicPVTestDriver)
	if !ok {
		return nil, fmt.Errorf("driver %s does not support dynamic provisioning", driver.GetDriverInfo().Name)
	}

	sc := pvTester.GetDynamicProvisionStorageClass(ctx, config, "")
	_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
	return sc, err
}
