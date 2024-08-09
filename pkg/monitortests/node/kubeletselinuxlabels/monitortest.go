package kubeletselinuxlabels

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/statetracker"
	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	msgPhaseDrain    = "drained node"
	msgPhaseOSUpdate = "updated operating system"
	msgPhaseReboot   = "rebooted and kubelet started"
	testName         = "[sig-node][kubelet] selinux labels on kubelet process should always be kubelet_t"
)

var (
	//go:embed *.yaml
	yamls embed.FS

	namespace *corev1.Namespace
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
}

type selinuxLabelWatcher struct {
	kubeClient    *kubernetes.Clientset
	namespaceName string
}

// This test was added to detect that selinux labels for the kubelet process
// are always kubelet_t.
// We notice that in cases of node disruption (restarts/starting) we were seeing that the labels
// regressed so we want to monitor them throughout all tests.
func NewSelinuxLabelWatcher() monitortestframework.MonitorTest {
	return &selinuxLabelWatcher{}
}

func (lw *selinuxLabelWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	lw.kubeClient = kubeClient
	nodes, err := lw.kubeClient.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return err
	}
	actualNamespace, err := lw.kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, v1.CreateOptions{})
	if err != nil {
		return err
	}
	lw.namespaceName = actualNamespace.Name

	for i, val := range nodes.Items {
		podWithNodeName := selinuxPodSpec(fmt.Sprintf("label-%d", i), actualNamespace.Name, val.Name)
		_, err := lw.kubeClient.CoreV1().Pods(lw.namespaceName).Create(ctx, podWithNodeName, v1.CreateOptions{})
		if err != nil {
			klog.InfoS("Failed to create pods", "Namespace", lw.namespaceName, "Name", val.Name)
			return err
		}
	}

	// we need to have the pods ready
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 400*time.Second, true, lw.allPodsStarted)
	if err != nil {
		return err
	}
	return nil
}

func (lw *selinuxLabelWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*selinuxLabelWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	var intervals monitorapi.Intervals
	nodeStateTracker := statetracker.NewStateTracker(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, beginning)
	locatorToMessageAnnotations := map[string]map[string]string{}

	for _, event := range startingIntervals {
		// TODO: dangerous assumptions here without using interval source, we ended up picking up container
		// ready events because they have a node in the locator, and a reason of "Ready".
		// Once the reasons marked "not ported" in the comments below are ported, we could filter here on
		// event.Source to ensure we only look at what we intend.
		node, ok := monitorapi.NodeFromLocator(event.Locator)
		if !ok {
			continue
		}
		reason := monitorapi.ReasonFrom(event.Message)
		if len(reason) == 0 {
			continue
		}

		roles := monitorapi.GetNodeRoles(event)

		nodeLocator := monitorapi.NewLocator().NodeFromName(node)
		nodeLocatorKey := nodeLocator.OldLocator()
		if _, ok := locatorToMessageAnnotations[nodeLocatorKey]; !ok {
			locatorToMessageAnnotations[nodeLocatorKey] = map[string]string{}
		}
		locatorToMessageAnnotations[nodeLocatorKey][string(monitorapi.AnnotationRoles)] = roles

		drainState := statetracker.State("Drain", "NodeUpdatePhases", monitorapi.NodeUpdateReason)
		osUpdateState := statetracker.State("OperatingSystemUpdate", "NodeUpdatePhases", monitorapi.NodeUpdateReason)
		rebootState := statetracker.State("Reboot", "NodeUpdatePhases", monitorapi.NodeUpdateReason)

		switch reason {
		case "Reboot":
			// Not ported, so we don't have a Source to check
			mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseDrain).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Drain")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
				event.From)...)

			osUpdateMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseOSUpdate).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "OperatingSystemUpdate")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, osUpdateMB),
				event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, rebootState, event.From)
		case "Starting":
			// Not ported, so we don't have a Source to check
			mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseDrain).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Drain")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
				event.From)...)

			osUpdateMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseOSUpdate).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "OperatingSystemUpdate")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, osUpdateMB),
				event.From)...)

			rebootMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseReboot).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Reboot")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, rebootState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, rebootMB),
				event.From)...)
		}
	}
	// Close all node intervals left hanging open:
	intervals = append(intervals, nodeStateTracker.CloseAllIntervals(locatorToMessageAnnotations, end)...)

	return intervals, nil
}

func (lw *selinuxLabelWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {

	podsList, err := lw.kubeClient.CoreV1().Pods(lw.namespaceName).List(ctx, v1.ListOptions{})
	if err != nil {
		return []*junitapi.JUnitTestCase{{Name: testName, SystemErr: err.Error()}}, err
	}
	for _, val := range podsList.Items {
		if !exutil.CheckPodIsRunning(val) {
			return []*junitapi.JUnitTestCase{{Name: testName, SystemErr: "selinux label not matching expected"}}, fmt.Errorf("selinux label not matching")
		}
	}
	return []*junitapi.JUnitTestCase{{Name: testName, SystemOut: "kubelet selinux labels match expected"}}, nil
}

func (*selinuxLabelWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (lw *selinuxLabelWatcher) Cleanup(ctx context.Context) error {

	if len(lw.namespaceName) > 0 && lw.kubeClient != nil {
		if err := lw.kubeClient.CoreV1().Namespaces().Delete(ctx, lw.namespaceName, v1.DeleteOptions{}); err != nil {
			return err
		}

		err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 20*time.Minute, true, lw.namespaceDeleted)
		if err != nil {
			return err
		}
	}
	return nil
}

func (lw *selinuxLabelWatcher) namespaceDeleted(ctx context.Context) (bool, error) {
	_, err := lw.kubeClient.CoreV1().Namespaces().Get(ctx, lw.namespaceName, v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		klog.Errorf("Error checking for deleted namespace: %s, %s", lw.namespaceName, err.Error())
		return false, err
	}

	return false, nil
}

func (lw *selinuxLabelWatcher) allPodsStarted(ctx context.Context) (bool, error) {
	pods, err := lw.kubeClient.CoreV1().Pods(lw.namespaceName).List(ctx, v1.ListOptions{})
	if err != nil {
		klog.Errorf("Error checking for pods: %s, %s", lw.namespaceName, err.Error())
		return false, err
	}
	for _, val := range pods.Items {
		if !exutil.CheckPodIsReady(val) {
			return false, fmt.Errorf("pod %s/%s is not ready with status %+v", val.Namespace, val.Name, val.Status.ContainerStatuses)
		}
	}

	return true, nil
}
