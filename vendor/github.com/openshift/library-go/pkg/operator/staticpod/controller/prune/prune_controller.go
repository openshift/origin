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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/prune/bindata"
)

// PruneController is a controller that watches static installer pod revision statuses and spawns
// a pruner pod to delete old revision resources from disk
type PruneController struct {
	targetNamespace, podResourcePrefix          string
	failedRevisionLimit, succeededRevisionLimit int

	// command is the string to use for the pruning pod command
	command []string
	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
	// prunerPodImageFn returns the image name for the pruning pod
	prunerPodImageFn func() string

	operatorConfigClient common.OperatorClient

	kubeClient    kubernetes.Interface
	eventRecorder events.Recorder
}

const (
	pruneControllerWorkQueueKey = "key"
	statusConfigMapName         = "revision-status-"
)

// NewPruneController creates a new pruning controller
func NewPruneController(
	targetNamespace string,
	podResourcePrefix string,
	command []string,
	kubeClient kubernetes.Interface,
	operatorConfigClient common.OperatorClient,
	eventRecorder events.Recorder,
) *PruneController {
	c := &PruneController{
		targetNamespace:        targetNamespace,
		podResourcePrefix:      podResourcePrefix,
		command:                command,
		failedRevisionLimit:    5,
		succeededRevisionLimit: 5,

		operatorConfigClient: operatorConfigClient,
		kubeClient:           kubeClient,
		eventRecorder:        eventRecorder,

		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PruneController"),
		prunerPodImageFn: getPrunerPodImageFromEnv,
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *PruneController) pruneRevisionHistory(operatorStatus *operatorv1.StaticPodOperatorStatus) error {
	var succeededRevisionIDs, failedRevisionIDs []int

	configMaps, err := c.kubeClient.CoreV1().ConfigMaps(c.targetNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, configMap := range configMaps.Items {
		if !strings.HasPrefix(configMap.Name, statusConfigMapName) {
			continue
		}

		if revision, ok := configMap.Data["revision"]; ok {
			revisionID, err := strconv.Atoi(revision)
			if err != nil {
				return err
			}
			switch configMap.Data["phase"] {
			case string(corev1.PodSucceeded):
				succeededRevisionIDs = append(succeededRevisionIDs, revisionID)
			case string(corev1.PodFailed):
				failedRevisionIDs = append(failedRevisionIDs, revisionID)
			default:
				return fmt.Errorf("unknown pod status phase for revision %d: %v", revisionID, configMap.Data["phase"])
			}
		}
	}

	// Return early if nothing to prune
	if len(succeededRevisionIDs)+len(failedRevisionIDs) == 0 {
		return nil
	}

	// Get list of protected IDs
	protectedSucceededRevisionIDs := protectedIDs(succeededRevisionIDs, c.succeededRevisionLimit)
	protectedFailedRevisionIDs := protectedIDs(failedRevisionIDs, c.failedRevisionLimit)

	excludedIDs := make([]int, 0, len(protectedSucceededRevisionIDs)+len(protectedFailedRevisionIDs))
	excludedIDs = append(excludedIDs, protectedSucceededRevisionIDs...)
	excludedIDs = append(excludedIDs, protectedFailedRevisionIDs...)
	sort.Ints(excludedIDs)

	// Run pruning pod on each node and pin it to that node
	for _, nodeStatus := range operatorStatus.NodeStatuses {
		if err := c.ensurePrunePod(nodeStatus.NodeName, excludedIDs[len(excludedIDs)-1], excludedIDs, nodeStatus.TargetRevision); err != nil {
			return err
		}
	}
	return nil
}

func protectedIDs(revisionIDs []int, revisionLimit int) []int {
	sort.Ints(revisionIDs)
	if len(revisionIDs) == 0 {
		return revisionIDs
	}
	return revisionIDs[protectedRevisionKeyToStart(len(revisionIDs), revisionLimit):]
}

func protectedRevisionKeyToStart(length, limit int) int {
	// 0 = default = unlimited revisions (ie, protect everything)
	if limit == 0 || length < limit {
		return 0
	}
	return length - limit
}

func (c *PruneController) ensurePrunePod(nodeName string, maxEligibleRevision int, protectedRevisions []int, revision int32) error {
	pod := resourceread.ReadPodV1OrDie(bindata.MustAsset(filepath.Join("pkg/operator/staticpod/controller/prune", "manifests/pruner-pod.yaml")))

	pod.Name = getPrunerPodName(nodeName, revision)
	pod.Namespace = c.targetNamespace
	pod.Spec.NodeName = nodeName
	pod.Spec.Containers[0].Image = c.prunerPodImageFn()
	pod.Spec.Containers[0].Command = c.command
	pod.Spec.Containers[0].Args = append(pod.Spec.Containers[0].Args,
		fmt.Sprintf("-v=%d", 4),
		fmt.Sprintf("--max-eligible-id=%d", maxEligibleRevision),
		fmt.Sprintf("--protected-ids=%s", revisionsToString(protectedRevisions)),
		fmt.Sprintf("--resource-dir=%s", "/etc/kubernetes/static-pod-resources"),
		fmt.Sprintf("--static-pod-name=%s", c.podResourcePrefix),
	)

	_, _, err := resourceapply.ApplyPod(c.kubeClient.CoreV1(), c.eventRecorder, pod)
	return err
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
	_, operatorStatus, _, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}

	return c.pruneRevisionHistory(operatorStatus)
}

// eventHandler queues the operator to check spec and status
func (c *PruneController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(pruneControllerWorkQueueKey) },
	}
}
