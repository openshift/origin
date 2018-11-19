package installer

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const installerControllerWorkQueueKey = "key"

type InstallerController struct {
	targetNamespace, staticPodName string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []string
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []string
	// command is the string to use for the installer pod command
	command []string

	operatorConfigClient common.OperatorClient

	kubeClient kubernetes.Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	// installerPodImageFn returns the image name for the installer pod
	installerPodImageFn func() string
}

// staticPodState is the status of a static pod that has been installed to a node.
type staticPodState int

const (
	// staticPodStatePending means that the installed static pod is not up yet.
	staticPodStatePending = staticPodState(iota)
	// staticPodStateReady means that the installed static pod is ready.
	staticPodStateReady
	// staticPodStateFailed means that the static pod installation of a node has failed.
	staticPodStateFailed
)

const revisionLabel = "revision"

func NewInstallerController(
	targetNamespace, staticPodName string,
	configMaps []string,
	secrets []string,
	command []string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient common.OperatorClient,
	kubeClient kubernetes.Interface,
) *InstallerController {
	c := &InstallerController{
		targetNamespace: targetNamespace,
		staticPodName:   staticPodName,
		configMaps:      configMaps,
		secrets:         secrets,
		command:         command,

		operatorConfigClient: operatorConfigClient,
		kubeClient:           kubeClient,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "InstallerController"),

		installerPodImageFn: getInstallerPodImageFromEnv,
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Pods().Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *InstallerController) getStaticPodState(nodeName string) (state staticPodState, revision string, errors []string, err error) {
	pod, err := c.kubeClient.CoreV1().Pods(c.targetNamespace).Get(mirrorPodNameForNode(c.staticPodName, nodeName), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return staticPodStatePending, "", nil, nil
		}
		return staticPodStatePending, "", nil, err
	}
	switch pod.Status.Phase {
	case corev1.PodRunning, corev1.PodSucceeded:
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return staticPodStateReady, pod.Labels[revisionLabel], nil, nil
			}
		}
	case corev1.PodFailed:
		return staticPodStateFailed, pod.Labels[revisionLabel], []string{pod.Status.Message}, nil
	}

	return staticPodStatePending, "", nil, nil
}

func (c *InstallerController) nodeToStartRevisionWith(nodes []operatorv1.NodeStatus) (int, error) {
	// find upgrading node as this will be the first to start new revision (to minimize number of down nodes)
	startNode := 0
	foundUpgradingNode := false
	for i := range nodes {
		if nodes[i].TargetRevision != 0 {
			startNode = i
			foundUpgradingNode = true
			break
		}
	}

	// otherwise try to find a node that is not ready regarding its currently reported revision
	if !foundUpgradingNode {
		for i := range nodes {
			currNodeState := &nodes[i]
			state, revision, _, err := c.getStaticPodState(currNodeState.NodeName)
			if err != nil {
				return 0, err
			}
			if state != staticPodStateReady || revision != strconv.Itoa(int(currNodeState.CurrentRevision)) {
				startNode = i
				break
			}
		}
	}

	return startNode, nil
}

// manageInstallationPods takes care of creating content for the static pods to install.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c *InstallerController) manageInstallationPods(operatorSpec *operatorv1.OperatorSpec, originalOperatorStatus *operatorv1.StaticPodOperatorStatus, resourceVersion string) (bool, error) {
	operatorStatus := originalOperatorStatus.DeepCopy()

	if len(operatorStatus.NodeStatuses) == 0 {
		return false, nil
	}

	// start with node which is in worst state (instead of terminating healthy pods first)
	startNode, err := c.nodeToStartRevisionWith(operatorStatus.NodeStatuses)
	if err != nil {
		return true, err
	}

	for l := 0; l < len(operatorStatus.NodeStatuses); l++ {
		i := (startNode + l) % len(operatorStatus.NodeStatuses)

		var currNodeState *operatorv1.NodeStatus
		var prevNodeState *operatorv1.NodeStatus
		currNodeState = &operatorStatus.NodeStatuses[i]
		if l > 0 {
			prev := (startNode + l - 1) % len(operatorStatus.NodeStatuses)
			prevNodeState = &operatorStatus.NodeStatuses[prev]
		}

		// if we are in a transition, check to see if our installer pod completed
		if currNodeState.TargetRevision > currNodeState.CurrentRevision {
			if err := c.ensureInstallerPod(currNodeState.NodeName, operatorSpec, currNodeState.TargetRevision); err != nil {
				return true, err
			}

			pendingNewRevision := operatorStatus.LatestAvailableRevision > currNodeState.TargetRevision
			newCurrNodeState, err := c.newNodeStateForInstallInProgress(currNodeState, pendingNewRevision)
			if err != nil {
				return true, err
			}

			// if we make a change to this status, we want to write it out to the API before we commence work on the next node.
			// it's an extra write/read, but it makes the state debuggable from outside this process
			if !equality.Semantic.DeepEqual(newCurrNodeState, currNodeState) {
				glog.Infof("%q moving to %v", currNodeState.NodeName, spew.Sdump(*newCurrNodeState))
				operatorStatus.NodeStatuses[i] = *newCurrNodeState
				if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
					_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
					return false, updateError
				}
			} else {
				glog.V(2).Infof("%q is in transition to %d, but has not made progress", currNodeState.NodeName, currNodeState.TargetRevision)
			}

			break
		}

		revisionToStart := c.getRevisionToStart(currNodeState, prevNodeState, operatorStatus)
		if revisionToStart == 0 {
			glog.V(4).Infof("%q does not need update", currNodeState.NodeName)
			continue
		}
		glog.Infof("%q needs new revision %d", currNodeState.NodeName, revisionToStart)

		newCurrNodeState := currNodeState.DeepCopy()
		newCurrNodeState.TargetRevision = revisionToStart
		newCurrNodeState.LastFailedRevisionErrors = nil

		// if we make a change to this status, we want to write it out to the API before we commence work on the next node.
		// it's an extra write/read, but it makes the state debuggable from outside this process
		if !equality.Semantic.DeepEqual(newCurrNodeState, currNodeState) {
			glog.Infof("%q moving to %v", currNodeState.NodeName, spew.Sdump(*newCurrNodeState))
			operatorStatus.NodeStatuses[i] = *newCurrNodeState
			if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
				_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
				return false, updateError
			}
		}
		break
	}

	v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
		Type:   "InstallerControllerFailing",
		Status: operatorv1.ConditionFalse,
	})
	if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
		_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
		if updateError != nil {
			return true, updateError
		}
	}

	return false, nil
}

// newNodeStateForInstallInProgress returns the new NodeState or error
func (c *InstallerController) newNodeStateForInstallInProgress(currNodeState *operatorv1.NodeStatus, newRevisionPending bool) (*operatorv1.NodeStatus, error) {
	ret := currNodeState.DeepCopy()
	installerPod, err := c.kubeClient.CoreV1().Pods(c.targetNamespace).Get(getInstallerPodName(currNodeState.TargetRevision, currNodeState.NodeName), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ret.LastFailedRevision = currNodeState.TargetRevision
		ret.TargetRevision = currNodeState.CurrentRevision
		ret.LastFailedRevisionErrors = []string{err.Error()}
		return ret, nil
	}
	if err != nil {
		return nil, err
	}

	failed := false
	errors := []string{}

	switch installerPod.Status.Phase {
	case corev1.PodSucceeded:
		if newRevisionPending {
			// stop early, don't wait for ready static pod because a new revision is waiting
			failed = true
			errors = append(errors, "static pod has been installed, but is not ready while new revision is pending")
			break
		}

		state, revision, failedErrors, err := c.getStaticPodState(currNodeState.NodeName)
		if err != nil {
			return nil, err
		}

		if revision != strconv.Itoa(int(currNodeState.TargetRevision)) {
			// new updated pod to be launched
			break
		}

		switch state {
		case staticPodStateFailed:
			failed = true
			errors = failedErrors

		case staticPodStateReady:
			ret.CurrentRevision = currNodeState.TargetRevision
			ret.TargetRevision = 0
			ret.LastFailedRevision = 0
			ret.LastFailedRevisionErrors = nil
			return ret, nil
		}

	case corev1.PodFailed:
		failed = true
		for _, containerStatus := range installerPod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil && len(containerStatus.State.Terminated.Message) > 0 {
				errors = append(errors, fmt.Sprintf("%s: %s", containerStatus.Name, containerStatus.State.Terminated.Message))
			}
		}
	}

	if failed {
		ret.LastFailedRevision = currNodeState.TargetRevision
		ret.TargetRevision = 0
		if len(errors) == 0 {
			errors = append(errors, "no detailed termination message, see `oc get -n %q pods/%q -oyaml`", installerPod.Namespace, installerPod.Name)
		}
		ret.LastFailedRevisionErrors = errors
		return ret, nil
	}

	return ret, nil
}

// getRevisionToStart returns the revision we need to start or zero if none
func (c *InstallerController) getRevisionToStart(currNodeState, prevNodeState *operatorv1.NodeStatus, operatorStatus *operatorv1.StaticPodOperatorStatus) int32 {
	if prevNodeState == nil {
		currentAtLatest := currNodeState.CurrentRevision == operatorStatus.LatestAvailableRevision
		failedAtLatest := currNodeState.LastFailedRevision == operatorStatus.LatestAvailableRevision
		if !currentAtLatest && !failedAtLatest {
			return operatorStatus.LatestAvailableRevision
		}
		return 0
	}

	prevFinished := prevNodeState.TargetRevision == 0
	prevInTransition := prevNodeState.CurrentRevision != prevNodeState.TargetRevision
	if prevInTransition && !prevFinished {
		return 0
	}

	prevAhead := prevNodeState.CurrentRevision > currNodeState.CurrentRevision
	failedAtPrev := currNodeState.LastFailedRevision == prevNodeState.CurrentRevision
	if prevAhead && !failedAtPrev {
		return prevNodeState.CurrentRevision
	}

	return 0
}

func getInstallerPodName(revision int32, nodeName string) string {
	return fmt.Sprintf("installer-%d-%s", revision, nodeName)
}

// ensureInstallerPod creates the installer pod with the secrets required to if it does not exist already
func (c *InstallerController) ensureInstallerPod(nodeName string, operatorSpec *operatorv1.OperatorSpec, revision int32) error {
	required := resourceread.ReadPodV1OrDie([]byte(installerPod))
	required.Name = getInstallerPodName(revision, nodeName)
	required.Namespace = c.targetNamespace
	required.Spec.NodeName = nodeName
	required.Spec.Containers[0].Image = c.installerPodImageFn()
	required.Spec.Containers[0].Command = c.command
	required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args,
		fmt.Sprintf("-v=%d", 4),
		fmt.Sprintf("--revision=%d", revision),
		fmt.Sprintf("--namespace=%s", c.targetNamespace),
		fmt.Sprintf("--pod=%s", c.configMaps[0]),
		fmt.Sprintf("--resource-dir=%s", "/etc/kubernetes/static-pod-resources"),
		fmt.Sprintf("--pod-manifest-dir=%s", "/etc/kubernetes/manifests"),
	)
	for _, name := range c.configMaps {
		required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args, fmt.Sprintf("--configmaps=%s", name))
	}
	for _, name := range c.secrets {
		required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args, fmt.Sprintf("--secrets=%s", name))
	}

	if _, err := c.kubeClient.CoreV1().Pods(c.targetNamespace).Create(required); err != nil && !apierrors.IsAlreadyExists(err) {
		glog.Errorf("failed to create pod on node %q for %s: %v", nodeName, resourceread.WritePodV1OrDie(required), err)
		return err
	}

	return nil
}

func getInstallerPodImageFromEnv() string {
	return os.Getenv("OPERATOR_IMAGE")
}

func (c InstallerController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	switch operatorSpec.ManagementState {
	case operatorv1.Unmanaged:
		return nil
	case operatorv1.Removed:
		// TODO probably just fail.  Static pod managers can't be removed.
		return nil
	}
	requeue, syncErr := c.manageInstallationPods(operatorSpec, operatorStatus, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request")
	}
	err = syncErr

	if err != nil {
		v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  "StatusUpdateError",
			Message: err.Error(),
		})
		if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
			if _, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus); updateError != nil {
				glog.Error(updateError)
			}
		}
		return err
	}

	return nil
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *InstallerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting InstallerController")
	defer glog.Infof("Shutting down InstallerController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *InstallerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *InstallerController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *InstallerController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
	}
}

func mirrorPodNameForNode(staticPodName, nodeName string) string {
	return staticPodName + "-" + nodeName
}

const installerPod = `apiVersion: v1
kind: Pod
metadata:
  namespace: <namespace>
  name: installer-<revision>-<nodeName>
  labels:
    app: installer
spec:
  serviceAccountName: installer-sa
  containers:
  - name: installer
    image: ${IMAGE}
    imagePullPolicy: Always
    securityContext:
      privileged: true
      runAsUser: 0
    terminationMessagePolicy: FallbackToLogsOnError
    volumeMounts:
    - mountPath: /etc/kubernetes/
      name: kubelet-dir
  restartPolicy: Never
  securityContext:
    runAsUser: 0
  volumes:
  - hostPath:
      path: /etc/kubernetes/
    name: kubelet-dir
`
