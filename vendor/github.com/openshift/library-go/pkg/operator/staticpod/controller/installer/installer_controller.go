package installer

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/installer/bindata"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	operatorStatusInstallerControllerFailing = "InstallerControllerFailing"
	nodeInstallerFailing                     = "NodeInstallerFailing"
	installerControllerWorkQueueKey          = "key"
	manifestDir                              = "pkg/operator/staticpod/controller/installer"
	manifestInstallerPodPath                 = "manifests/installer-pod.yaml"

	hostResourceDirDir = "/etc/kubernetes/static-pod-resources"
	hostPodManifestDir = "/etc/kubernetes/manifests"

	revisionLabel       = "revision"
	statusConfigMapName = "revision-status"
)

// InstallerController is a controller that watches the currentRevision and targetRevision fields for each node and spawn
// installer pods to update the static pods on the master nodes.
type InstallerController struct {
	targetNamespace, staticPodName string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []revision.RevisionResource
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []revision.RevisionResource
	// command is the string to use for the installer pod command
	command []string

	// these are copied separately at the beginning to a fixed location
	certConfigMaps []revision.RevisionResource
	certSecrets    []revision.RevisionResource
	certDir        string

	operatorConfigClient v1helpers.StaticPodOperatorClient

	configMapsGetter corev1client.ConfigMapsGetter
	podsGetter       corev1client.PodsGetter

	eventRecorder events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	// installerPodImageFn returns the image name for the installer pod
	installerPodImageFn func() string
	// ownerRefsFn sets the ownerrefs on the pruner pod
	ownerRefsFn func(revision int32) ([]metav1.OwnerReference, error)

	installerPodMutationFns []InstallerPodMutationFunc
}

// InstallerPodMutationFunc is a function that has a chance at changing the installer pod before it is created
type InstallerPodMutationFunc func(pod *corev1.Pod, nodeName string, operatorSpec *operatorv1.StaticPodOperatorSpec, revision int32) error

func (o *InstallerController) WithInstallerPodMutationFn(installerPodMutationFn InstallerPodMutationFunc) *InstallerController {
	o.installerPodMutationFns = append(o.installerPodMutationFns, installerPodMutationFn)
	return o
}

func (o *InstallerController) WithCerts(certDir string, certConfigMaps, certSecrets []revision.RevisionResource) *InstallerController {
	o.certDir = certDir
	o.certConfigMaps = certConfigMaps
	o.certSecrets = certSecrets
	return o
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

// NewInstallerController creates a new installer controller.
func NewInstallerController(
	targetNamespace, staticPodName string,
	configMaps []revision.RevisionResource,
	secrets []revision.RevisionResource,
	command []string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient v1helpers.StaticPodOperatorClient,
	configMapsGetter corev1client.ConfigMapsGetter,
	podsGetter corev1client.PodsGetter,
	eventRecorder events.Recorder,
) *InstallerController {
	c := &InstallerController{
		targetNamespace: targetNamespace,
		staticPodName:   staticPodName,
		configMaps:      configMaps,
		secrets:         secrets,
		command:         command,

		operatorConfigClient: operatorConfigClient,
		configMapsGetter:     configMapsGetter,
		podsGetter:           podsGetter,
		eventRecorder:        eventRecorder.WithComponentSuffix("installer-controller"),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "InstallerController"),

		installerPodImageFn: getInstallerPodImageFromEnv,
	}

	c.ownerRefsFn = c.setOwnerRefs
	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Pods().Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *InstallerController) getStaticPodState(nodeName string) (state staticPodState, revision string, errors []string, err error) {
	pod, err := c.podsGetter.Pods(c.targetNamespace).Get(mirrorPodNameForNode(c.staticPodName, nodeName), metav1.GetOptions{})
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

// nodeToStartRevisionWith returns a node index i and guarantees for every node < i that it is
// - not updating
// - ready
// - at the revision claimed in CurrentRevision.
func nodeToStartRevisionWith(getStaticPodState func(nodeName string) (state staticPodState, revision string, errors []string, err error), nodes []operatorv1.NodeStatus) (int, error) {
	if len(nodes) == 0 {
		return 0, fmt.Errorf("nodes array cannot be empty")
	}

	// find upgrading node as this will be the first to start new revision (to minimize number of down nodes)
	for i := range nodes {
		if nodes[i].TargetRevision != 0 {
			return i, nil
		}
	}

	// otherwise try to find a node that is not ready. Take the oldest one.
	oldestNotReadyRevisionNode := -1
	oldestNotReadyRevision := math.MaxInt32
	for i := range nodes {
		currNodeState := &nodes[i]
		state, revision, _, err := getStaticPodState(currNodeState.NodeName)
		if err != nil && apierrors.IsNotFound(err) {
			return i, nil
		}
		if err != nil {
			return 0, err
		}
		revisionNum, err := strconv.Atoi(revision)
		if err != nil {
			return i, nil
		}
		if state != staticPodStateReady && revisionNum < oldestNotReadyRevision {
			oldestNotReadyRevisionNode = i
			oldestNotReadyRevision = revisionNum
		}
	}
	if oldestNotReadyRevisionNode >= 0 {
		return oldestNotReadyRevisionNode, nil
	}

	// find a node that is has the wrong revision. Take the oldest one.
	oldestPodRevisionNode := -1
	oldestPodRevision := math.MaxInt32
	for i := range nodes {
		currNodeState := &nodes[i]
		_, revision, _, err := getStaticPodState(currNodeState.NodeName)
		if err != nil {
			return 0, err
		}
		revisionNum, err := strconv.Atoi(revision)
		if err != nil {
			return i, nil
		}
		if revisionNum != int(currNodeState.CurrentRevision) && revisionNum < oldestPodRevision {
			oldestPodRevisionNode = i
			oldestPodRevision = revisionNum
		}
	}
	if oldestPodRevisionNode >= 0 {
		return oldestPodRevisionNode, nil
	}

	// last but not least, choose the one with the older current revision. This will imply that failed installer pods will be retried.
	oldestCurrentRevisionNode := -1
	oldestCurrentRevision := int32(math.MaxInt32)
	for i := range nodes {
		currNodeState := &nodes[i]
		if currNodeState.CurrentRevision < oldestCurrentRevision {
			oldestCurrentRevisionNode = i
			oldestCurrentRevision = currNodeState.CurrentRevision
		}
	}
	if oldestCurrentRevisionNode >= 0 {
		return oldestCurrentRevisionNode, nil
	}

	return 0, nil
}

// manageInstallationPods takes care of creating content for the static pods to install.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c *InstallerController) manageInstallationPods(operatorSpec *operatorv1.StaticPodOperatorSpec, originalOperatorStatus *operatorv1.StaticPodOperatorStatus, resourceVersion string) (bool, error) {
	operatorStatus := originalOperatorStatus.DeepCopy()

	if len(operatorStatus.NodeStatuses) == 0 {
		return false, nil
	}

	// stop on first deployment failure of the latest revision (excluding OOM, that never sets LatestAvailableRevision).
	for _, s := range operatorStatus.NodeStatuses {
		if s.LastFailedRevision == operatorStatus.LatestAvailableRevision {
			return false, nil
		}
	}

	// start with node which is in worst state (instead of terminating healthy pods first)
	startNode, err := nodeToStartRevisionWith(c.getStaticPodState, operatorStatus.NodeStatuses)
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

		// if we are in a transition, check to see whether our installer pod completed
		if currNodeState.TargetRevision > currNodeState.CurrentRevision {
			if err := c.ensureInstallerPod(currNodeState.NodeName, operatorSpec, currNodeState.TargetRevision); err != nil {
				c.eventRecorder.Warningf("InstallerPodFailed", "Failed to create installer pod for revision %d on node %q: %v",
					currNodeState.TargetRevision, currNodeState.NodeName, err)
				return true, err
			}

			pendingNewRevision := operatorStatus.LatestAvailableRevision > currNodeState.TargetRevision
			newCurrNodeState, oom, err := c.newNodeStateForInstallInProgress(currNodeState, pendingNewRevision)
			if err != nil {
				return true, err
			}

			// if we make a change to this status, we want to write it out to the API before we commence work on the next node.
			// it's an extra write/read, but it makes the state debuggable from outside this process
			if !equality.Semantic.DeepEqual(newCurrNodeState, currNodeState) {
				glog.Infof("%q moving to %v", currNodeState.NodeName, spew.Sdump(*newCurrNodeState))
				newOperatorStatus, updated, updateError := v1helpers.UpdateStaticPodStatus(c.operatorConfigClient, setNodeStatusFn(newCurrNodeState), setAvailableProgressingNodeInstallerFailingConditions)
				if updateError != nil {
					return false, updateError
				} else if updated && currNodeState.CurrentRevision != newCurrNodeState.CurrentRevision {
					c.eventRecorder.Eventf("NodeCurrentRevisionChanged", "Updated node %q from revision %d to %d", currNodeState.NodeName,
						currNodeState.CurrentRevision, newCurrNodeState.CurrentRevision)
				}
				if err := c.updateRevisionStatus(newOperatorStatus); err != nil {
					glog.Errorf("error updating revision status configmap: %v", err)
				}
				return false, nil
			} else {
				glog.V(2).Infof("%q is in transition to %d, but has not made progress", currNodeState.NodeName, currNodeState.TargetRevision)
			}
			if !oom {
				break
			}

			// OOM is special. We want to retry the installer pod by falling through here. Also we don't set LastFailedRevision.
			glog.V(2).Infof("Retrying %q for revision %d because it was OOM killed", currNodeState.NodeName, currNodeState.TargetRevision)
			installerPodName := getInstallerPodName(currNodeState.TargetRevision, currNodeState.NodeName)
			if err := c.podsGetter.Pods(c.targetNamespace).Delete(installerPodName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return true, err
			}
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
			if _, updated, updateError := v1helpers.UpdateStaticPodStatus(c.operatorConfigClient, setNodeStatusFn(newCurrNodeState), setAvailableProgressingNodeInstallerFailingConditions); updateError != nil {
				return false, updateError
			} else if updated && currNodeState.TargetRevision != newCurrNodeState.TargetRevision && newCurrNodeState.TargetRevision != 0 {
				c.eventRecorder.Eventf("NodeTargetRevisionChanged", "Updating node %q from revision %d to %d", currNodeState.NodeName,
					currNodeState.CurrentRevision, newCurrNodeState.TargetRevision)
			}

			return false, nil
		}
		break
	}

	return false, nil
}

func (c *InstallerController) updateRevisionStatus(operatorStatus *operatorv1.StaticPodOperatorStatus) error {
	failedRevisions := make(map[int32]struct{})
	currentRevisions := make(map[int32]struct{})
	for _, nodeState := range operatorStatus.NodeStatuses {
		failedRevisions[nodeState.LastFailedRevision] = struct{}{}
		currentRevisions[nodeState.CurrentRevision] = struct{}{}
	}
	delete(failedRevisions, 0)

	// If all current revisions point to the same revision, then mark it successful
	if len(currentRevisions) == 1 {
		err := c.updateConfigMapForRevision(currentRevisions, string(corev1.PodSucceeded))
		if err != nil {
			return err
		}
	}
	return c.updateConfigMapForRevision(failedRevisions, string(corev1.PodFailed))
}

func (c *InstallerController) updateConfigMapForRevision(currentRevisions map[int32]struct{}, status string) error {
	for currentRevision := range currentRevisions {
		statusConfigMap, err := c.configMapsGetter.ConfigMaps(c.targetNamespace).Get(statusConfigMapNameForRevision(currentRevision), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			glog.Infof("%s configmap not found, skipping update revision status", statusConfigMapNameForRevision(currentRevision))
			continue
		}
		if err != nil {
			return err
		}
		statusConfigMap.Data["status"] = status
		_, _, err = resourceapply.ApplyConfigMap(c.configMapsGetter, c.eventRecorder, statusConfigMap)
		if err != nil {
			return err
		}
	}
	return nil
}

func setNodeStatusFn(status *operatorv1.NodeStatus) v1helpers.UpdateStaticPodStatusFunc {
	return func(operatorStatus *operatorv1.StaticPodOperatorStatus) error {
		for i := range operatorStatus.NodeStatuses {
			if operatorStatus.NodeStatuses[i].NodeName == status.NodeName {
				operatorStatus.NodeStatuses[i] = *status
				break
			}
		}
		return nil
	}
}

// setAvailableProgressingConditions sets the Available and Progressing conditions
func setAvailableProgressingNodeInstallerFailingConditions(newStatus *operatorv1.StaticPodOperatorStatus) error {
	// Available means that we have at least one pod at the latest level
	numAvailable := 0
	numAtLatestRevision := 0
	numProgressing := 0
	counts := map[int32]int{}
	failingCount := map[int32]int{}
	failing := map[int32][]string{}
	for _, currNodeStatus := range newStatus.NodeStatuses {
		if currNodeStatus.CurrentRevision != 0 {
			numAvailable++
		}

		// keep track of failures so that we can report failing status
		if currNodeStatus.LastFailedRevision != 0 {
			existing := failingCount[currNodeStatus.CurrentRevision]
			failingCount[currNodeStatus.CurrentRevision] = existing + 1
			failing[currNodeStatus.LastFailedRevision] = append(failing[currNodeStatus.LastFailedRevision], currNodeStatus.LastFailedRevisionErrors...)
		}

		existing := counts[currNodeStatus.CurrentRevision]
		counts[currNodeStatus.CurrentRevision] = existing + 1

		if newStatus.LatestAvailableRevision == currNodeStatus.CurrentRevision {
			numAtLatestRevision += 1
		} else {
			numProgressing += 1
		}
	}

	revisionStrings := []string{}
	for revision, count := range counts {
		revisionStrings = append(revisionStrings, fmt.Sprintf("%d nodes are at revision %d", count, revision))
	}
	revisionDescription := strings.Join(revisionStrings, "; ")

	if numAvailable > 0 {
		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeAvailable,
			Status:  operatorv1.ConditionTrue,
			Message: fmt.Sprintf("%d nodes are active; %s", numAvailable, revisionDescription),
		})
	} else {
		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeAvailable,
			Status:  operatorv1.ConditionFalse,
			Reason:  "ZeroNodesActive",
			Message: fmt.Sprintf("%d nodes are active; %s", numAvailable, revisionDescription),
		})
	}

	// Progressing means that the any node is not at the latest available revision
	if numProgressing > 0 {
		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeProgressing,
			Status:  operatorv1.ConditionTrue,
			Message: fmt.Sprintf("%s", revisionDescription),
		})
	} else {
		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeProgressing,
			Status:  operatorv1.ConditionFalse,
			Reason:  "AllNodesAtLatestRevision",
			Message: fmt.Sprintf("%s", revisionDescription),
		})
	}

	if len(failing) > 0 {
		failingStrings := []string{}
		for failingRevision, errorStrings := range failing {
			failingStrings = append(failingStrings, fmt.Sprintf("%d nodes are failing on revision %d:\n%v", failingCount[failingRevision], failingRevision, strings.Join(errorStrings, "\n")))
		}
		failingDescription := strings.Join(failingStrings, "; ")

		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:    nodeInstallerFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  "InstallerPodFailed",
			Message: failingDescription,
		})
	} else {
		v1helpers.SetOperatorCondition(&newStatus.Conditions, operatorv1.OperatorCondition{
			Type:   nodeInstallerFailing,
			Status: operatorv1.ConditionFalse,
		})
	}

	return nil
}

// newNodeStateForInstallInProgress returns the new NodeState, whether it was killed by OOM or an error
func (c *InstallerController) newNodeStateForInstallInProgress(currNodeState *operatorv1.NodeStatus, newRevisionPending bool) (status *operatorv1.NodeStatus, oom bool, err error) {
	ret := currNodeState.DeepCopy()
	installerPod, err := c.podsGetter.Pods(c.targetNamespace).Get(getInstallerPodName(currNodeState.TargetRevision, currNodeState.NodeName), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ret.LastFailedRevision = currNodeState.TargetRevision
		ret.TargetRevision = currNodeState.CurrentRevision
		ret.LastFailedRevisionErrors = []string{err.Error()}
		return ret, false, nil
	}
	if err != nil {
		return nil, false, err
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
			return nil, false, err
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
			return ret, false, nil
		}

	case corev1.PodFailed:
		failed = true
		for _, containerStatus := range installerPod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil && len(containerStatus.State.Terminated.Message) > 0 {
				errors = append(errors, fmt.Sprintf("%s: %s", containerStatus.Name, containerStatus.State.Terminated.Message))
				if containerStatus.State.Terminated.Reason == "OOMKilled" {
					// do not set LastFailedRevision
					return currNodeState, true, nil
				}
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
		return ret, false, nil
	}

	return ret, false, nil
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
func (c *InstallerController) ensureInstallerPod(nodeName string, operatorSpec *operatorv1.StaticPodOperatorSpec, revision int32) error {
	pod := resourceread.ReadPodV1OrDie(bindata.MustAsset(filepath.Join(manifestDir, manifestInstallerPodPath)))

	pod.Namespace = c.targetNamespace
	pod.Name = getInstallerPodName(revision, nodeName)
	pod.Spec.NodeName = nodeName
	pod.Spec.Containers[0].Image = c.installerPodImageFn()
	pod.Spec.Containers[0].Command = c.command

	ownerRefs, err := c.ownerRefsFn(revision)
	if err != nil {
		return fmt.Errorf("unable to set installer pod ownerrefs: %+v", err)
	}
	pod.OwnerReferences = ownerRefs

	if c.configMaps[0].Optional {
		return fmt.Errorf("pod configmap %s is required, cannot be optional", c.configMaps[0].Name)
	}
	args := []string{
		"-v=4", // TODO: Make this configurable?
		fmt.Sprintf("--revision=%d", revision),
		fmt.Sprintf("--namespace=%s", pod.Namespace),
		fmt.Sprintf("--pod=%s", c.configMaps[0].Name),
		fmt.Sprintf("--resource-dir=%s", hostResourceDirDir),
		fmt.Sprintf("--pod-manifest-dir=%s", hostPodManifestDir),
	}
	for _, cm := range c.configMaps {
		if cm.Optional {
			args = append(args, fmt.Sprintf("--optional-configmaps=%s", cm.Name))
		} else {
			args = append(args, fmt.Sprintf("--configmaps=%s", cm.Name))
		}
	}
	for _, s := range c.secrets {
		if s.Optional {
			args = append(args, fmt.Sprintf("--optional-secrets=%s", s.Name))
		} else {
			args = append(args, fmt.Sprintf("--secrets=%s", s.Name))
		}
	}
	if len(c.certDir) > 0 {
		args = append(args, fmt.Sprintf("--cert-dir=%s", filepath.Join(hostResourceDirDir, c.certDir)))
		for _, cm := range c.certConfigMaps {
			if cm.Optional {
				args = append(args, fmt.Sprintf("--optional-cert-configmaps=%s", cm.Name))
			} else {
				args = append(args, fmt.Sprintf("--cert-configmaps=%s", cm.Name))
			}
		}
		for _, s := range c.certSecrets {
			if s.Optional {
				args = append(args, fmt.Sprintf("--optional-cert-secrets=%s", s.Name))
			} else {
				args = append(args, fmt.Sprintf("--cert-secrets=%s", s.Name))
			}
		}
	}

	pod.Spec.Containers[0].Args = args

	// Some owners need to change aspects of the pod.  Things like arguments for instance
	for _, fn := range c.installerPodMutationFns {
		if err := fn(pod, nodeName, operatorSpec, revision); err != nil {
			return err
		}
	}

	_, _, err = resourceapply.ApplyPod(c.podsGetter, c.eventRecorder, pod)
	return err
}

func (c *InstallerController) setOwnerRefs(revision int32) ([]metav1.OwnerReference, error) {
	ownerReferences := []metav1.OwnerReference{}
	statusConfigMap, err := c.configMapsGetter.ConfigMaps(c.targetNamespace).Get(fmt.Sprintf("revision-status-%d", revision), metav1.GetOptions{})
	if err == nil {
		ownerReferences = append(ownerReferences, metav1.OwnerReference{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       statusConfigMap.Name,
			UID:        statusConfigMap.UID,
		})
	}
	return ownerReferences, err
}

func getInstallerPodImageFromEnv() string {
	return os.Getenv("OPERATOR_IMAGE")
}

func (c InstallerController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	requeue, syncErr := c.manageInstallationPods(operatorSpec, operatorStatus, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request")
	}
	err = syncErr

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   operatorStatusInstallerControllerFailing,
		Status: operatorv1.ConditionFalse,
	}
	if err != nil {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, _, updateError := v1helpers.UpdateStaticPodStatus(c.operatorConfigClient, v1helpers.UpdateStaticPodConditionFn(cond), setAvailableProgressingNodeInstallerFailingConditions); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
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

func statusConfigMapNameForRevision(revision int32) string {
	return fmt.Sprintf("%s-%d", statusConfigMapName, revision)
}
