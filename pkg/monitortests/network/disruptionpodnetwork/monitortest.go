package disruptionpodnetwork

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var (
	//go:embed *.yaml
	yamls embed.FS

	namespace                                *corev1.Namespace
	pollerRoleBinding                        *rbacv1.RoleBinding
	podNetworkToPodNetworkPollerDeployment   *appsv1.Deployment
	podNetworkToHostNetworkPollerDeployment  *appsv1.Deployment
	hostNetworkToPodNetworkPollerDeployment  *appsv1.Deployment
	hostNetworkToHostNetworkPollerDeployment *appsv1.Deployment
	podNetworkServicePollerDep               *appsv1.Deployment
	hostNetworkServicePollerDep              *appsv1.Deployment
	podNetworkTargetDeployment               *appsv1.Deployment
	podNetworkTargetService                  *corev1.Service
	hostNetworkTargetDeployment              *appsv1.Deployment
	hostNetworkTargetService                 *corev1.Service
)

func yamlOrDie(name string) []byte {
	ret, err := yamls.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return ret
}

func init() {
	namespace = resourceread.ReadNamespaceV1OrDie(yamlOrDie("namespace.yaml"))
	pollerRoleBinding = resourceread.ReadRoleBindingV1OrDie(yamlOrDie("poller-rolebinding.yaml"))
	podNetworkToPodNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-pod-network-poller-deployment.yaml"))
	podNetworkToHostNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-host-network-poller-deployment.yaml"))
	hostNetworkToPodNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-pod-network-poller-deployment.yaml"))
	hostNetworkToHostNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-host-network-poller-deployment.yaml"))
	podNetworkServicePollerDep = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-service-poller-deployment.yaml"))
	hostNetworkServicePollerDep = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-service-poller-deployment.yaml"))
	podNetworkTargetDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-target-deployment.yaml"))
	podNetworkTargetService = resourceread.ReadServiceV1OrDie(yamlOrDie("pod-network-target-service.yaml"))
	hostNetworkTargetDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-target-deployment.yaml"))
	hostNetworkTargetService = resourceread.ReadServiceV1OrDie(yamlOrDie("host-network-target-service.yaml"))
}

type podNetworkAvalibility struct {
	payloadImagePullSpec string
	notSupportedReason   error
	namespaceName        string
	targetService        *corev1.Service
	kubeClient           kubernetes.Interface
	adminRESTConfig      *rest.Config
}

// OVNPodDebugInfo holds debugging information for an OVN pod
type OVNPodDebugInfo struct {
	PodName        string
	NodeName       string
	RestartCount   int32
	CPUUsage       string
	MemoryUsage    string
	Reason         string // Why this pod was selected for debugging
	OVSDPCTLOutput string
	ContainerName  string
}

func NewPodNetworkAvalibilityInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &podNetworkAvalibility{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (pna *podNetworkAvalibility) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	deploymentID := uuid.New().String()

	// Store the admin config for later use in debugging
	pna.adminRESTConfig = adminRESTConfig

	oc := util.NewCLIWithoutNamespace("openshift-tests")
	openshiftTestsImagePullSpec, err := GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, pna.payloadImagePullSpec, oc)
	if err != nil {
		pna.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("unable to determine openshift-tests image: %v", err)}
		return pna.notSupportedReason
	}

	isManagedServiceCluster, err := util.IsManagedServiceCluster(ctx, oc.AdminKubeClient())
	if isManagedServiceCluster {
		pna.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("pod network tests are unschedulable on ROSA TRT-1869")}
		return pna.notSupportedReason
	}

	pna.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	actualNamespace, err := pna.kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	pna.namespaceName = actualNamespace.Name

	if _, err = pna.kubeClient.RbacV1().RoleBindings(pna.namespaceName).Create(context.Background(), pollerRoleBinding, metav1.CreateOptions{}); err != nil {
		return err
	}

	// our pods tolerate masters, so create one for each of them.
	nodes, err := pna.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	numNodes := int32(len(nodes.Items))

	klog.Infof("Starting deployment: %s", podNetworkToPodNetworkPollerDeployment.Name)
	podNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	podNetworkToPodNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(podNetworkToPodNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", podNetworkToHostNetworkPollerDeployment.Name)
	podNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	podNetworkToHostNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(podNetworkToHostNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", hostNetworkToPodNetworkPollerDeployment.Name)
	hostNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	hostNetworkToPodNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(hostNetworkToPodNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", hostNetworkToHostNetworkPollerDeployment.Name)
	hostNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	hostNetworkToHostNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(hostNetworkToHostNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", podNetworkTargetDeployment.Name)
	// force the image to use the "normal" global mapping.
	originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
	podNetworkTargetDeployment.Spec.Replicas = &numNodes
	podNetworkTargetDeployment.Spec.Template.Spec.Containers[0].Image = image.LocationFor(originalAgnhost.GetE2EImage())
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	service, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), podNetworkTargetService, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	pna.targetService = service
	klog.Infof("Starting deployment: %s", hostNetworkTargetDeployment.Name)
	hostNetworkTargetDeployment.Spec.Replicas = &numNodes
	hostNetworkTargetDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), hostNetworkTargetService, metav1.CreateOptions{}); err != nil {
		return err
	}

	// we need to have the service network pollers wait until we have at least one healthy endpoint before starting.
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 300*time.Second, true, pna.serviceHasEndpoints)
	if err != nil {
		return err
	}

	for _, deployment := range []*appsv1.Deployment{podNetworkServicePollerDep, hostNetworkServicePollerDep} {
		time.Sleep(30 * time.Second)
		klog.Infof("Starting deployment: %s", deployment.Name)
		deployment.Spec.Replicas = &numNodes
		deployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
		deployment = disruptionlibrary.UpdateDeploymentENVs(deployment, deploymentID, service.Spec.ClusterIP)
		if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), deployment, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (pna *podNetworkAvalibility) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (pna *podNetworkAvalibility) serviceHasEndpoints(ctx context.Context) (bool, error) {
	targetServiceLabel, err := labels.NewRequirement("kubernetes.io/service-name", selection.Equals, []string{pna.targetService.Name})
	if err != nil {
		return false, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*targetServiceLabel).String(),
	}
	endpointSlices, err := pna.kubeClient.DiscoveryV1().EndpointSlices(pna.targetService.Namespace).List(ctx, listOptions)
	if err != nil {
		klog.Error(err.Error())
		return false, nil
	}

	for _, endpointSlice := range endpointSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Serving != nil && *endpoint.Conditions.Serving {
				// we have at least one endpoint
				return true, nil
			}
		}
	}

	return false, nil
}

func (pna *podNetworkAvalibility) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if pna.notSupportedReason != nil {
		return nil, nil, pna.notSupportedReason
	}

	// create the stop collecting configmap and wait for 30s to thing to have stopped.  the 30s is just a guess
	if _, err := pna.kubeClient.CoreV1().ConfigMaps(pna.namespaceName).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-collecting"},
	}, metav1.CreateOptions{}); err != nil {
		return nil, nil, err
	}

	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	retIntervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	for _, typeOfConnection := range []string{"pod-to-pod", "pod-to-host", "host-to-pod", "host-to-host", "pod-to-service", "host-to-service"} {
		localIntervals, localJunit, localErrs := pna.collectDetailsForPoller(ctx, typeOfConnection)
		retIntervals = append(retIntervals, localIntervals...)
		junits = append(junits, localJunit...)
		errs = append(errs, localErrs...)

	}

	return retIntervals, junits, utilerrors.NewAggregate(errs)
}

func (pna *podNetworkAvalibility) collectDetailsForPoller(ctx context.Context, typeOfConnection string) (monitorapi.Intervals, []*junitapi.JUnitTestCase, []error) {
	pollerLabel, err := labels.NewRequirement("network.openshift.io/disruption-actor", selection.Equals, []string{"poller"})
	if err != nil {
		return nil, nil, []error{err}
	}
	typeLabel, err := labels.NewRequirement("network.openshift.io/disruption-target", selection.Equals, []string{typeOfConnection})
	if err != nil {
		return nil, nil, []error{err}
	}
	labelSelector := labels.NewSelector().Add(*pollerLabel).Add(*typeLabel)
	return disruptionlibrary.CollectIntervalsForPods(ctx, pna.kubeClient, "sig-network", pna.namespaceName, labelSelector)
}

func (pna *podNetworkAvalibility) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, pna.notSupportedReason
}

func (pna *podNetworkAvalibility) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, pna.notSupportedReason
}

func (pna *podNetworkAvalibility) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if pna.notSupportedReason != nil {
		return pna.notSupportedReason
	}

	// Collect OVN debugging information after the pod-to-pod test execution
	debugInfo, err := pna.collectOVNDebugInfo(ctx)
	if err != nil {
		klog.Warningf("Failed to collect OVN debug info: %v", err)
		// Don't fail the test if debug collection fails, just log the warning
	}

	if len(debugInfo) > 0 {
		// Create gather-extra directory if it doesn't exist
		gatherExtraDir := filepath.Join(storageDir, "gather-extra")
		if err := os.MkdirAll(gatherExtraDir, 0755); err != nil {
			klog.Warningf("Failed to create gather-extra directory: %v", err)
			return nil
		}

		// Write OVN debug information to file
		debugFileName := fmt.Sprintf("ovn-debug-info%s.txt", timeSuffix)
		debugFilePath := filepath.Join(gatherExtraDir, debugFileName)

		debugContent := pna.formatOVNDebugInfo(debugInfo)
		if err := ioutil.WriteFile(debugFilePath, []byte(debugContent), 0644); err != nil {
			klog.Warningf("Failed to write OVN debug info to file: %v", err)
		} else {
			klog.Infof("Successfully wrote OVN debug info to %s", debugFilePath)
		}
	}

	return nil
}

func (pna *podNetworkAvalibility) namespaceDeleted(ctx context.Context) (bool, error) {
	_, err := pna.kubeClient.CoreV1().Namespaces().Get(ctx, pna.namespaceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		klog.Errorf("Error checking for deleted namespace: %s, %s", pna.namespaceName, err.Error())
		return false, err
	}

	return false, nil
}

func (pna *podNetworkAvalibility) Cleanup(ctx context.Context) error {
	if len(pna.namespaceName) > 0 && pna.kubeClient != nil {
		if err := pna.kubeClient.CoreV1().Namespaces().Delete(ctx, pna.namespaceName, metav1.DeleteOptions{}); err != nil {
			return err
		}

		startTime := time.Now()
		err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 20*time.Minute, true, pna.namespaceDeleted)
		if err != nil {
			return err
		}

		klog.Infof("Deleting namespace: %s took %.2f seconds", pna.namespaceName, time.Now().Sub(startTime).Seconds())

	}
	return nil
}

// collectOVNDebugInfo collects debugging information from OVN pods
func (pna *podNetworkAvalibility) collectOVNDebugInfo(ctx context.Context) ([]OVNPodDebugInfo, error) {
	var debugInfos []OVNPodDebugInfo

	// Get all OVN pods from openshift-ovn-kubernetes namespace
	ovnPods, err := pna.kubeClient.CoreV1().Pods("openshift-ovn-kubernetes").List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-node",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list OVN pods: %v", err)
	}

	if len(ovnPods.Items) == 0 {
		klog.Warningf("No OVN pods found in openshift-ovn-kubernetes namespace")
		return debugInfos, nil
	}

	// Find pods with highest restart count
	highestRestartPod := pna.findPodWithHighestRestartCount(ovnPods.Items)
	if highestRestartPod != nil {
		debugInfo, err := pna.collectDebugInfoForPod(ctx, *highestRestartPod, "highest restart count")
		if err != nil {
			klog.Warningf("Failed to collect debug info for highest restart pod %s: %v", highestRestartPod.Name, err)
		} else {
			debugInfos = append(debugInfos, *debugInfo)
		}
	}

	// Find pods with abnormally high CPU or memory usage
	highResourcePods := pna.findPodsWithHighResourceUsage(ctx, ovnPods.Items)
	for _, pod := range highResourcePods {
		debugInfo, err := pna.collectDebugInfoForPod(ctx, pod, "high resource usage")
		if err != nil {
			klog.Warningf("Failed to collect debug info for high resource pod %s: %v", pod.Name, err)
		} else {
			debugInfos = append(debugInfos, *debugInfo)
		}
	}

	// TODO: Add logic to identify pods involved in failures based on test results
	// For now, we'll collect from any failed pods
	failedPods := pna.findFailedPods(ovnPods.Items)
	for _, pod := range failedPods {
		debugInfo, err := pna.collectDebugInfoForPod(ctx, pod, "pod failure")
		if err != nil {
			klog.Warningf("Failed to collect debug info for failed pod %s: %v", pod.Name, err)
		} else {
			debugInfos = append(debugInfos, *debugInfo)
		}
	}

	return debugInfos, nil
}

// findPodWithHighestRestartCount finds the pod with the highest restart count
func (pna *podNetworkAvalibility) findPodWithHighestRestartCount(pods []corev1.Pod) *corev1.Pod {
	if len(pods) == 0 {
		return nil
	}

	var maxRestartPod *corev1.Pod
	maxRestarts := int32(0)

	for i := range pods {
		pod := &pods[i]
		totalRestarts := int32(0)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}

		if totalRestarts > maxRestarts {
			maxRestarts = totalRestarts
			maxRestartPod = pod
		}
	}

	// Only return if there are actual restarts
	if maxRestarts > 0 {
		return maxRestartPod
	}
	return nil
}

// findPodsWithHighResourceUsage finds pods with abnormally high CPU or memory usage
func (pna *podNetworkAvalibility) findPodsWithHighResourceUsage(ctx context.Context, pods []corev1.Pod) []corev1.Pod {
	var highResourcePods []corev1.Pod

	// Define thresholds for high resource usage
	cpuThreshold := resource.MustParse("500m")   // 0.5 CPU cores
	memoryThreshold := resource.MustParse("1Gi") // 1GB memory

	for _, pod := range pods {
		// Check if pod has high resource requests/limits or is showing signs of resource pressure
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					if cpu.Cmp(cpuThreshold) > 0 {
						highResourcePods = append(highResourcePods, pod)
						break
					}
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					if memory.Cmp(memoryThreshold) > 0 {
						highResourcePods = append(highResourcePods, pod)
						break
					}
				}
			}
		}

		// Also check for pods that are in resource-related failure states
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
				if strings.Contains(condition.Reason, "OutOfMemory") ||
					strings.Contains(condition.Reason, "OutOfCpu") ||
					strings.Contains(condition.Message, "memory") ||
					strings.Contains(condition.Message, "cpu") {
					highResourcePods = append(highResourcePods, pod)
					break
				}
			}
		}
	}

	return highResourcePods
}

// findFailedPods finds pods that are in failed states
func (pna *podNetworkAvalibility) findFailedPods(pods []corev1.Pod) []corev1.Pod {
	var failedPods []corev1.Pod

	for _, pod := range pods {
		// Check if pod is in failed phase
		if pod.Status.Phase == corev1.PodFailed {
			failedPods = append(failedPods, pod)
			continue
		}

		// Check for failed containers
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil &&
				(containerStatus.State.Waiting.Reason == "CrashLoopBackOff" ||
					containerStatus.State.Waiting.Reason == "ImagePullBackOff" ||
					containerStatus.State.Waiting.Reason == "ErrImagePull") {
				failedPods = append(failedPods, pod)
				break
			}
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
				failedPods = append(failedPods, pod)
				break
			}
		}

		// Check pod conditions for failures
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
				if strings.Contains(condition.Reason, "Failed") ||
					strings.Contains(condition.Reason, "Error") {
					failedPods = append(failedPods, pod)
					break
				}
			}
		}
	}

	return failedPods
}

// collectDebugInfoForPod collects debugging information for a specific pod
func (pna *podNetworkAvalibility) collectDebugInfoForPod(ctx context.Context, pod corev1.Pod, reason string) (*OVNPodDebugInfo, error) {
	// Calculate total restart count
	totalRestarts := int32(0)
	for _, containerStatus := range pod.Status.ContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}

	// Find the appropriate container (ovnkube-node or ovnkube-controller)
	containerName := ""
	for _, container := range pod.Spec.Containers {
		if container.Name == "ovnkube-node" || container.Name == "ovnkube-controller" {
			containerName = container.Name
			break
		}
	}
	if containerName == "" {
		return nil, fmt.Errorf("no suitable OVN container found in pod %s", pod.Name)
	}

	// Collect ovs-dpctl dump-flows output
	ovsDpctlOutput, err := pna.execInPod(ctx, pod.Namespace, pod.Name, containerName, []string{"ovs-dpctl", "dump-flows"})
	if err != nil {
		klog.Warningf("Failed to collect ovs-dpctl dump-flows from pod %s: %v", pod.Name, err)
		ovsDpctlOutput = fmt.Sprintf("Failed to collect ovs-dpctl output: %v", err)
	}

	// Get resource usage information (basic info from pod status)
	cpuUsage := "N/A"
	memoryUsage := "N/A"

	// Try to get resource usage from pod status
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					cpuUsage = fmt.Sprintf("Requested: %s", cpu.String())
				}
				if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					memoryUsage = fmt.Sprintf("Requested: %s", memory.String())
				}
			}
			break
		}
	}

	debugInfo := &OVNPodDebugInfo{
		PodName:        pod.Name,
		NodeName:       pod.Spec.NodeName,
		RestartCount:   totalRestarts,
		CPUUsage:       cpuUsage,
		MemoryUsage:    memoryUsage,
		Reason:         reason,
		OVSDPCTLOutput: ovsDpctlOutput,
		ContainerName:  containerName,
	}

	return debugInfo, nil
}

// execInPod executes a command in a pod and returns the output
func (pna *podNetworkAvalibility) execInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, error) {
	output, err := util.ExecInPodWithResult(
		pna.kubeClient.CoreV1(),
		pna.adminRESTConfig,
		namespace,
		podName,
		containerName,
		command,
	)
	return output, err
}

// formatOVNDebugInfo formats the debug information into a readable string
func (pna *podNetworkAvalibility) formatOVNDebugInfo(debugInfos []OVNPodDebugInfo) string {
	var result strings.Builder

	result.WriteString("=== OVN Pod Debug Information ===\n")
	result.WriteString(fmt.Sprintf("Collected at: %s\n", time.Now().Format(time.RFC3339)))
	result.WriteString(fmt.Sprintf("Total pods analyzed: %d\n\n", len(debugInfos)))

	for i, info := range debugInfos {
		result.WriteString(fmt.Sprintf("--- Pod %d: %s ---\n", i+1, info.PodName))
		result.WriteString(fmt.Sprintf("Node: %s\n", info.NodeName))
		result.WriteString(fmt.Sprintf("Container: %s\n", info.ContainerName))
		result.WriteString(fmt.Sprintf("Reason for selection: %s\n", info.Reason))
		result.WriteString(fmt.Sprintf("Restart count: %d\n", info.RestartCount))
		result.WriteString(fmt.Sprintf("CPU usage: %s\n", info.CPUUsage))
		result.WriteString(fmt.Sprintf("Memory usage: %s\n", info.MemoryUsage))
		result.WriteString("\n--- ovs-dpctl dump-flows output ---\n")
		result.WriteString(info.OVSDPCTLOutput)
		result.WriteString("\n\n")
	}

	return result.String()
}
