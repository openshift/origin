package prune

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/prune/bindata"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// PruneController is a controller that watches static installer pod revision statuses and spawns
// a pruner pod to delete old revision resources from disk
type PruneController struct {
	targetNamespace, podResourcePrefix string
	// command is the string to use for the pruning pod command
	command []string
	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	// prunerPodImageFn returns the image name for the pruning pod
	prunerPodImageFn func() string
	// ownerRefsFn sets the ownerrefs on the pruner pod
	ownerRefsFn func(revision int32) ([]metav1.OwnerReference, error)

	operatorConfigClient v1helpers.StaticPodOperatorClient

	configMapGetter corev1client.ConfigMapsGetter
	secretGetter    corev1client.SecretsGetter
	podGetter       corev1client.PodsGetter
	eventRecorder   events.Recorder
}

const (
	pruneControllerWorkQueueKey = "key"
	statusConfigMapName         = "revision-status-"
	defaultRevisionLimit        = int32(5)
)

// NewPruneController creates a new pruning controller
func NewPruneController(
	targetNamespace string,
	podResourcePrefix string,
	command []string,
	configMapGetter corev1client.ConfigMapsGetter,
	secretGetter corev1client.SecretsGetter,
	podGetter corev1client.PodsGetter,
	operatorConfigClient v1helpers.StaticPodOperatorClient,
	eventRecorder events.Recorder,
) *PruneController {
	c := &PruneController{
		targetNamespace:   targetNamespace,
		podResourcePrefix: podResourcePrefix,
		command:           command,

		operatorConfigClient: operatorConfigClient,

		configMapGetter: configMapGetter,
		secretGetter:    secretGetter,
		podGetter:       podGetter,
		eventRecorder:   eventRecorder.WithComponentSuffix("prune-controller"),

		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PruneController"),
		prunerPodImageFn: getPrunerPodImageFromEnv,
	}

	c.ownerRefsFn = c.setOwnerRefs
	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())

	return c
}

func getRevisionLimits(operatorSpec *operatorv1.StaticPodOperatorSpec) (int32, int32) {
	failedRevisionLimit := defaultRevisionLimit
	succeededRevisionLimit := defaultRevisionLimit
	if operatorSpec.FailedRevisionLimit != 0 {
		failedRevisionLimit = operatorSpec.FailedRevisionLimit
	}
	if operatorSpec.SucceededRevisionLimit != 0 {
		succeededRevisionLimit = operatorSpec.SucceededRevisionLimit
	}
	return failedRevisionLimit, succeededRevisionLimit
}

func (c *PruneController) excludedRevisionHistory(operatorStatus *operatorv1.StaticPodOperatorStatus, failedRevisionLimit, succeededRevisionLimit int32) ([]int, error) {
	var succeededRevisions, failedRevisions, inProgressRevisions, unknownStatusRevisions []int

	configMaps, err := c.configMapGetter.ConfigMaps(c.targetNamespace).List(metav1.ListOptions{})
	if err != nil {
		return []int{}, err
	}
	for _, configMap := range configMaps.Items {
		if !strings.HasPrefix(configMap.Name, statusConfigMapName) {
			continue
		}

		if revision, ok := configMap.Data["revision"]; ok {
			revisionNumber, err := strconv.Atoi(revision)
			if err != nil {
				return []int{}, err
			}
			switch configMap.Data["status"] {
			case string(corev1.PodSucceeded):
				succeededRevisions = append(succeededRevisions, revisionNumber)
			case string(corev1.PodFailed):
				failedRevisions = append(failedRevisions, revisionNumber)

			case "InProgress":
				// we always protect inprogress
				inProgressRevisions = append(inProgressRevisions, revisionNumber)

			default:
				// protect things you don't understand
				unknownStatusRevisions = append(unknownStatusRevisions, revisionNumber)
				c.eventRecorder.Event("UnknownRevisionStatus", fmt.Sprintf("unknown status for revision %d: %v", revisionNumber, configMap.Data["status"]))
			}
		}
	}

	// Return early if nothing to prune
	if len(succeededRevisions)+len(failedRevisions) == 0 {
		glog.V(2).Info("no revision IDs currently eligible to prune")
		return []int{}, nil
	}

	// Get list of protected IDs
	protectedSucceededRevisions := protectedRevisions(succeededRevisions, int(succeededRevisionLimit))
	protectedFailedRevisions := protectedRevisions(failedRevisions, int(failedRevisionLimit))

	excludedRevisions := make([]int, 0, len(protectedSucceededRevisions)+len(protectedFailedRevisions)+len(inProgressRevisions)+len(unknownStatusRevisions))
	excludedRevisions = append(excludedRevisions, protectedSucceededRevisions...)
	excludedRevisions = append(excludedRevisions, protectedFailedRevisions...)
	excludedRevisions = append(excludedRevisions, inProgressRevisions...)
	excludedRevisions = append(excludedRevisions, unknownStatusRevisions...)
	sort.Ints(excludedRevisions)

	// There should always be at least 1 excluded ID, otherwise we'll delete the current revision
	if len(excludedRevisions) == 0 {
		return []int{}, fmt.Errorf("need at least 1 excluded ID for revision pruning")
	}
	return excludedRevisions, nil
}

func (c *PruneController) pruneDiskResources(operatorStatus *operatorv1.StaticPodOperatorStatus, excludedRevisions []int, maxEligibleRevision int) error {
	// Run pruning pod on each node and pin it to that node
	for _, nodeStatus := range operatorStatus.NodeStatuses {
		// Use the highest value between CurrentRevision and LastFailedRevision
		// Because CurrentRevision only updates on successful installs and we still prune on an unsuccessful install
		if err := c.ensurePrunePod(nodeStatus.NodeName, maxEligibleRevision, excludedRevisions, max(nodeStatus.LastFailedRevision, nodeStatus.CurrentRevision)); err != nil {
			return err
		}
	}
	return nil
}

func (c *PruneController) pruneAPIResources(excludedRevisions []int, maxEligibleRevision int) error {
	protectedRevisions := sets.NewInt(excludedRevisions...)
	statusConfigMaps, err := c.configMapGetter.ConfigMaps(c.targetNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cm := range statusConfigMaps.Items {
		if !strings.HasPrefix(cm.Name, statusConfigMapName) {
			continue
		}

		revision, err := strconv.Atoi(cm.Data["revision"])
		if err != nil {
			return fmt.Errorf("unexpected error converting revision to int: %+v", err)
		}

		if protectedRevisions.Has(revision) {
			continue
		}
		if revision > maxEligibleRevision {
			continue
		}
		if err := c.configMapGetter.ConfigMaps(c.targetNamespace).Delete(cm.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func protectedRevisions(revisions []int, revisionLimit int) []int {
	sort.Ints(revisions)
	if len(revisions) == 0 {
		return revisions
	}
	startKey := 0
	// We use -1 = unlimited revisions, so protect all. Limit shouldn't ever be literally 0 either
	if revisionLimit > 0 && len(revisions) > revisionLimit {
		startKey = len(revisions) - revisionLimit
	}
	return revisions[startKey:]
}

func (c *PruneController) ensurePrunePod(nodeName string, maxEligibleRevision int, protectedRevisions []int, revision int32) error {
	if revision == 0 {
		return nil
	}
	pod := resourceread.ReadPodV1OrDie(bindata.MustAsset(filepath.Join("pkg/operator/staticpod/controller/prune", "manifests/pruner-pod.yaml")))

	pod.Name = getPrunerPodName(nodeName, revision)
	pod.Namespace = c.targetNamespace
	pod.Spec.NodeName = nodeName
	pod.Spec.Containers[0].Image = c.prunerPodImageFn()
	pod.Spec.Containers[0].Command = c.command
	pod.Spec.Containers[0].Args = append(pod.Spec.Containers[0].Args,
		fmt.Sprintf("-v=%d", 4),
		fmt.Sprintf("--max-eligible-revision=%d", maxEligibleRevision),
		fmt.Sprintf("--protected-revisions=%s", revisionsToString(protectedRevisions)),
		fmt.Sprintf("--resource-dir=%s", "/etc/kubernetes/static-pod-resources"),
		fmt.Sprintf("--static-pod-name=%s", c.podResourcePrefix),
	)

	ownerRefs, err := c.ownerRefsFn(revision)
	if err != nil {
		return fmt.Errorf("unable to set pruner pod ownerrefs: %+v", err)
	}
	pod.OwnerReferences = ownerRefs

	_, _, err = resourceapply.ApplyPod(c.podGetter, c.eventRecorder, pod)
	return err
}

func (c *PruneController) setOwnerRefs(revision int32) ([]metav1.OwnerReference, error) {
	ownerReferences := []metav1.OwnerReference{}
	statusConfigMap, err := c.configMapGetter.ConfigMaps(c.targetNamespace).Get(fmt.Sprintf("revision-status-%d", revision), metav1.GetOptions{})
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

func getPrunerPodName(nodeName string, revision int32) string {
	return fmt.Sprintf("revision-pruner-%d-%s", revision, nodeName)
}

func revisionsToString(revisions []int) string {
	values := []string{}
	for _, id := range revisions {
		value := strconv.Itoa(id)
		values = append(values, value)
	}
	return strings.Join(values, ",")
}

func getPrunerPodImageFromEnv() string {
	return os.Getenv("OPERATOR_IMAGE")
}

func (c *PruneController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting PruneController")
	defer glog.Infof("Shutting down PruneController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *PruneController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *PruneController) processNextWorkItem() bool {
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

func (c *PruneController) sync() error {
	glog.V(5).Info("Syncing revision pruner")
	operatorSpec, operatorStatus, _, err := c.operatorConfigClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}
	failedLimit, succeededLimit := getRevisionLimits(operatorSpec)

	excludedRevisions, err := c.excludedRevisionHistory(operatorStatus, failedLimit, succeededLimit)
	if err != nil {
		return err
	}
	// if no IDs are excluded, then there is nothing to prune
	if len(excludedRevisions) == 0 {
		glog.Info("No excluded revisions to prune, skipping")
		return nil
	}

	errs := []error{}
	if diskErr := c.pruneDiskResources(operatorStatus, excludedRevisions, excludedRevisions[len(excludedRevisions)-1]); diskErr != nil {
		errs = append(errs, diskErr)
	}
	if apiErr := c.pruneAPIResources(excludedRevisions, excludedRevisions[len(excludedRevisions)-1]); apiErr != nil {
		errs = append(errs, apiErr)
	}
	return v1helpers.NewMultiLineAggregate(errs)
}

// eventHandler queues the operator to check spec and status
func (c *PruneController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
	}
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
