package csi

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	resource2 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

	// propagate the timeoutString from the test config to ginkgo.It("[Timeout:xyz]") to set test suite timeoutString
	timeoutString := DefaultLUNStressTestTimeout
	if csiSuite.lunStressTestConfig != nil && csiSuite.lunStressTestConfig.Timeout != "" {
		timeoutString = csiSuite.lunStressTestConfig.Timeout
	}
	timeout, err := time.ParseDuration(timeoutString)
	if err != nil {
		panic(fmt.Sprintf("Cannot parse %s as time.Duration: %s", timeoutString, err))
	}
	testName := fmt.Sprintf("should use many PVs on a single node [Serial][Timeout:%s]", timeoutString)

	g.It(testName, g.Label("Size:L"), func(ctx context.Context) {
		if csiSuite.lunStressTestConfig == nil {
			g.Skip("lunStressTestConfig is empty")
		}
		if csiSuite.lunStressTestConfig.PodsTotal == 0 {
			g.Skip("lunStressTestConfig is explicitly disabled")
		}
		e2e.Logf("Starting LUN stress test with config: %+v", csiSuite.lunStressTestConfig)
		until := time.Now().Add(timeout)

		g.By("Selecting a schedulable node")
		node, err := node2.GetRandomReadySchedulableNode(ctx, f.ClientSet)
		e2e.ExpectNoError(err, "getting a schedulable node")

		g.By("Creating a StorageClass")
		config := driver.PrepareTest(ctx, f)
		sc, err := createSC(ctx, f, driver, config)
		e2e.ExpectNoError(err, "creating StorageClass")
		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up StorageClass %s", sc.Name)
			err := f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
			e2e.ExpectNoError(err, "deleting StorageClass", sc.Name)
		})

		podCount := csiSuite.lunStressTestConfig.PodsTotal
		e2e.Logf("Starting %d pods", podCount)
		for i := 0; i < podCount; i++ {
			startTestPod(ctx, f, node.Name, config, sc.Name, i)
		}
		e2e.Logf("All pods created, waiting for them to start until %s", until.String())

		// Some time was already spent when creating pods.
		waitTimeout := until.Sub(time.Now())
		err = waitForPodsComplete(ctx, f, podCount, waitTimeout)
		e2e.ExpectNoError(err, "waiting for pods to complete")
		e2e.Logf("All pods completed, cleaning up")
	})
}

// Create one PVC + Pod. Do not wait for the pod to start!
func startTestPod(ctx context.Context, f *e2e.Framework, nodeName string, config *storageframework.PerTestConfig, scName string, podNumber int) {
	pvcName := fmt.Sprintf("pvc-%d", podNumber)

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

	g.DeferCleanup(func(ctx context.Context) {
		err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting PVC %s", pvc.Name)
	})

	podConfig := &e2epod.Config{
		NS:            f.Namespace.Name,
		PVCs:          []*corev1.PersistentVolumeClaim{pvc},
		NodeSelection: e2epod.NodeSelection{Name: nodeName},
		Command:       "ls -la " + e2epod.VolumeMountPath1,
	}
	pod, err := e2epod.MakeSecPod(podConfig)
	e2e.ExpectNoError(err, "preparing pod %d", podNumber)
	// Make the pod name nicer to users, it has a random uuid otherwise
	pod.Name = fmt.Sprintf("pod-%d", podNumber)

	pod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
	e2e.ExpectNoError(err, "creating pod %d", podNumber)
	e2e.Logf("Pod %s + PVC %s created", pod.Name, pvc.Name)
	g.DeferCleanup(func(ctx context.Context) {
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		e2e.ExpectNoError(err, "deleting pod %s", pod.Name)
	})
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

func waitForPodsComplete(ctx context.Context, f *e2e.Framework, podCount int, timeout time.Duration) error {
	var incomplete, complete []*corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, false, func(ctx context.Context) (done bool, err error) {
		pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, fmt.Errorf("error listing pods: %w", err)
		}
		complete = nil
		incomplete = nil

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded {
				complete = append(complete, &pod)
			} else {
				incomplete = append(incomplete, &pod)
			}
		}

		if len(complete) == podCount {
			return true, nil
		}
		if len(complete)+len(incomplete) != podCount {
			return false, fmt.Errorf("unexpected pod count: expected %d, got %d", len(complete)+len(incomplete), podCount)
		}
		e2e.Logf("Waiting for %d pods to complete, %d done", podCount, len(complete))
		return false, nil
	})

	if err != nil {
		e2e.Logf("Wait failed")
		for i := range incomplete {
			e2e.Logf("Incomplete pod %s: %s", incomplete[i].Name, incomplete[i].Status.Phase)
		}
	}
	return err
}
