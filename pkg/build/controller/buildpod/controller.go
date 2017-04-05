package buildpod

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
)

const (
	// We must avoid processing build pods until the build and pod stores have synced.
	// If they haven't synced, to avoid a hot loop, we'll wait this long between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
	maxRetries            = 15
	maxNotFoundRetries    = 5
)

// BuildPodController watches pods running builds and manages the build state
type BuildPodController struct {
	buildUpdater buildclient.BuildUpdater
	secretClient kcoreclient.SecretsGetter
	podClient    kcoreclient.PodsGetter

	queue workqueue.RateLimitingInterface

	buildStore oscache.StoreToBuildLister
	podStore   cache.StoreToPodLister

	buildStoreSynced func() bool
	podStoreSynced   func() bool

	runPolicies []policy.RunPolicy
}

// NewBuildPodController creates a new BuildPodController.
func NewBuildPodController(buildInformer, podInformer cache.SharedIndexInformer, kc kclientset.Interface, oc osclient.Interface) *BuildPodController {
	buildListerUpdater := buildclient.NewOSClientBuildClient(oc)
	c := &BuildPodController{
		buildUpdater: buildListerUpdater,
		secretClient: kc.Core(), // TODO: Replace with cache client
		podClient:    kc.Core(),
		queue:        workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	c.runPolicies = policy.GetAllRunPolicies(buildListerUpdater, buildListerUpdater)

	c.buildStore.Indexer = buildInformer.GetIndexer()
	c.podStore.Indexer = podInformer.GetIndexer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePod,
		DeleteFunc: c.deletePod,
	})
	buildInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addBuild,
		UpdateFunc: c.updateBuild,
	})

	c.buildStoreSynced = buildInformer.HasSynced
	c.podStoreSynced = podInformer.HasSynced

	return c
}

type podNotFoundKey struct {
	Namespace string
	PodName   string
	BuildName string
}

// Run begins watching and syncing.
func (c *BuildPodController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// Wait for the build store to sync before starting any work in this controller.
	if !cache.WaitForCacheSync(stopCh, c.buildStoreSynced, c.podStoreSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down build pod controller")
}

// HandlePod updates the state of the build based on the pod state
func (bc *BuildPodController) HandlePod(pod *kapi.Pod) error {
	glog.V(5).Infof("Handling update of build pod %s/%s", pod.Namespace, pod.Name)
	build, exists, err := bc.getBuildForPod(pod)
	if err != nil {
		glog.V(4).Infof("Error getting build for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return err
	}
	if !exists || build == nil {
		glog.V(5).Infof("No build found for pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}

	nextStatus := build.Status.Phase
	currentReason := build.Status.Reason

	if build.Status.Phase != buildapi.BuildPhaseFailed {
		switch pod.Status.Phase {
		case kapi.PodPending:
			build.Status.Reason = ""
			build.Status.Message = ""
			nextStatus = buildapi.BuildPhasePending
			if secret := build.Spec.Output.PushSecret; secret != nil && currentReason != buildapi.StatusReasonMissingPushSecret {
				if _, err := bc.secretClient.Secrets(build.Namespace).Get(secret.Name); err != nil && errors.IsNotFound(err) {
					build.Status.Reason = buildapi.StatusReasonMissingPushSecret
					build.Status.Message = buildapi.StatusMessageMissingPushSecret
					glog.V(4).Infof("Setting reason for pending build to %q due to missing secret %s/%s", build.Status.Reason, build.Namespace, secret.Name)
				}
			}

		case kapi.PodRunning:
			// The pod's still running
			build.Status.Reason = ""
			build.Status.Message = ""
			nextStatus = buildapi.BuildPhaseRunning

		case kapi.PodSucceeded:
			build.Status.Reason = ""
			build.Status.Message = ""
			// Check the exit codes of all the containers in the pod
			nextStatus = buildapi.BuildPhaseComplete
			if len(pod.Status.ContainerStatuses) == 0 {
				// no containers in the pod means something went badly wrong, so the build
				// should be failed.
				glog.V(2).Infof("Failing build %s/%s because the pod has no containers", build.Namespace, build.Name)
				nextStatus = buildapi.BuildPhaseFailed
				if build.Status.Reason == "" {
					build.Status.Reason = buildapi.StatusReasonBuildPodDeleted
					build.Status.Message = buildapi.StatusMessageBuildPodDeleted
				}
			} else {
				for _, info := range pod.Status.ContainerStatuses {
					if info.State.Terminated != nil && info.State.Terminated.ExitCode != 0 {
						nextStatus = buildapi.BuildPhaseFailed
						break
					}
				}
			}

		case kapi.PodFailed:
			nextStatus = buildapi.BuildPhaseFailed
			if build.Status.Reason == "" {
				build.Status.Reason = buildapi.StatusReasonGenericBuildFailed
				build.Status.Message = buildapi.StatusMessageGenericBuildFailed
			}

		default:
			build.Status.Reason = ""
			build.Status.Message = ""
		}
	}
	// Update the build object when it progress to a next state or the reason for
	// the current state changed.
	if (!common.HasBuildPodNameAnnotation(build) || build.Status.Phase != nextStatus || build.Status.Phase == buildapi.BuildPhaseFailed) && !buildutil.IsBuildComplete(build) {
		common.SetBuildPodNameAnnotation(build, pod.Name)
		reason := ""
		if len(build.Status.Reason) > 0 {
			reason = " (" + string(build.Status.Reason) + ")"
		}
		glog.V(4).Infof("Updating build %s/%s status %s -> %s%s", build.Namespace, build.Name, build.Status.Phase, nextStatus, reason)
		build.Status.Phase = nextStatus

		if buildutil.IsBuildComplete(build) {
			common.SetBuildCompletionTimeAndDuration(build)
		}
		if build.Status.Phase == buildapi.BuildPhaseRunning {
			now := unversioned.Now()
			build.Status.StartTimestamp = &now
		}
		if err := bc.buildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("failed to update build %s/%s: %v", build.Namespace, build.Name, err)
		}
		glog.V(4).Infof("Build %s/%s status was updated to %s", build.Namespace, build.Name, build.Status.Phase)

		if buildutil.IsBuildComplete(build) {
			common.HandleBuildCompletion(build, bc.runPolicies)
		}
	}
	return nil
}

// HandleBuildPodDeletion sets the status of a build to error if the build pod has been deleted
func (bc *BuildPodController) HandleBuildPodDeletion(pod *kapi.Pod) error {
	glog.V(4).Infof("Handling deletion of build pod %s/%s", pod.Namespace, pod.Name)
	build, exists, err := bc.getBuildForPod(pod)
	if err != nil {
		glog.V(4).Infof("Error getting build for pod %s/%s", pod.Namespace, pod.Name)
		return err
	}
	if !exists || build == nil {
		glog.V(5).Infof("No build found for deleted pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}

	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		glog.V(4).Infof("Build %s/%s is a pipeline build, ignoring", build.Namespace, build.Name)
		return nil
	}
	// If build was cancelled, we'll leave HandleBuild to update the build
	if build.Status.Cancelled {
		glog.V(4).Infof("Cancelation for build %s/%s was already triggered, ignoring", build.Namespace, build.Name)
		return nil
	}

	if buildutil.IsBuildComplete(build) {
		glog.V(4).Infof("Pod was deleted but build %s/%s is already completed, so no need to update it.", build.Namespace, build.Name)
		return nil
	}

	nextStatus := buildapi.BuildPhaseError
	if build.Status.Phase != nextStatus {
		glog.V(4).Infof("Updating build %s/%s status %s -> %s", build.Namespace, build.Name, build.Status.Phase, nextStatus)
		build.Status.Phase = nextStatus
		build.Status.Reason = buildapi.StatusReasonBuildPodDeleted
		build.Status.Message = buildapi.StatusMessageBuildPodDeleted
		common.SetBuildCompletionTimeAndDuration(build)
		if err := bc.buildUpdater.Update(build.Namespace, build); err != nil {
			return fmt.Errorf("Failed to update build %s/%s: %v", build.Namespace, build.Name, err)
		}
	}
	return nil
}

func (bc *BuildPodController) worker() {
	for {
		if quit := bc.work(); quit {
			return
		}
	}
}

func (bc *BuildPodController) work() bool {
	key, quit := bc.queue.Get()
	if quit {
		return true
	}
	defer bc.queue.Done(key)

	if notFoundKey, ok := key.(podNotFoundKey); ok {
		bc.handlePodNotFound(notFoundKey)
		return true
	}

	pod, err := bc.getPodByKey(key.(string))
	if err != nil {
		utilruntime.HandleError(err)
	}

	if pod == nil {
		return false
	}

	err = bc.HandlePod(pod)
	bc.handleError(err, key, pod)

	return false
}

func podForNotFoundKey(key podNotFoundKey) *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Namespace = key.Namespace
	pod.Name = key.PodName
	pod.Annotations = map[string]string{buildapi.BuildAnnotation: key.BuildName}
	return pod
}

func (bc *BuildPodController) handlePodNotFound(key podNotFoundKey) {
	_, err := bc.podStore.Pods(key.Namespace).Get(key.PodName)
	if err == nil {
		glog.V(4).Infof("Found missing pod %s/%s\n", key.Namespace, key.PodName)
		bc.queue.Forget(key)
		return
	}

	// If number of retries has not been exceeded, requeue and attempt to retrieve from cache again
	if bc.queue.NumRequeues(key) < maxNotFoundRetries {
		glog.V(4).Infof("Failed to retrieve build pod %s/%s: %v. Retrying it.", key.Namespace, key.PodName, err)
		bc.queue.AddRateLimited(key)
		return
	}

	// Once the maximum number of retries has been exceeded, try retrieving it from the API server directly
	bc.queue.Forget(key)
	_, err = bc.podClient.Pods(key.Namespace).Get(key.PodName)
	if err != nil && errors.IsNotFound(err) {
		// If the pod is still not found, handle it as a deletion event
		err = bc.HandleBuildPodDeletion(podForNotFoundKey(key))
	}
	utilruntime.HandleError(err)
}

func (bc *BuildPodController) getPodByKey(key string) (*kapi.Pod, error) {
	obj, exists, err := bc.podStore.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve pod %q from store: %v", key, err)
		bc.queue.AddRateLimited(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Pod %q has been deleted", key)
		return nil, nil
	}

	return obj.(*kapi.Pod), nil
}

func (bc *BuildPodController) updatePod(old, cur interface{}) {
	// A periodic relist will send update events for all known pods.
	curPod := cur.(*kapi.Pod)
	oldPod := old.(*kapi.Pod)
	if curPod.ResourceVersion == oldPod.ResourceVersion {
		return
	}
	if isBuildPod(curPod) {
		bc.enqueuePod(curPod)
	}
}

func (bc *BuildPodController) deletePod(obj interface{}) {
	pod, ok := obj.(*kapi.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone: %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*kapi.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod: %+v", obj))
			return
		}
	}
	if isBuildPod(pod) {
		err := bc.HandleBuildPodDeletion(pod)
		utilruntime.HandleError(err)
	}
}

func (bc *BuildPodController) addBuild(obj interface{}) {
	build := obj.(*buildapi.Build)
	bc.checkBuildPodDeletion(build)
}

func (bc *BuildPodController) updateBuild(old, cur interface{}) {
	build := cur.(*buildapi.Build)
	bc.checkBuildPodDeletion(build)
}

func (bc *BuildPodController) checkBuildPodDeletion(build *buildapi.Build) {
	switch {
	case buildutil.IsBuildComplete(build):
		glog.V(5).Infof("checkBuildPodDeletion: ignoring build %s/%s because it is complete", build.Namespace, build.Name)
		return
	case build.Status.Phase == buildapi.BuildPhaseNew:
		glog.V(5).Infof("checkBuildPodDeletion: ignoring build %s/%s because it is new", build.Namespace, build.Name)
		return
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		glog.V(5).Infof("checkBuildPodDeletion: ignoring build %s/%s because it is a pipeline build", build.Namespace, build.Name)
		return
	}
	_, err := bc.podStore.Pods(build.Namespace).Get(buildapi.GetBuildPodName(build))

	// The only error that can currently be returned is a NotFound error, but adding a check
	// here just in case that changes in the future
	if err != nil && errors.IsNotFound(err) {
		// If the pod is not found, enqueue a pod not found event. The reason is that
		// the cache may not have been populated at the time. With the pod not found key,
		// we will keep trying to find the pod a fixed number of times. If at that point, the pod
		// is not found by directly accessing the API server, then we will mark the build
		// as failed.
		bc.enqueuePodNotFound(build.Namespace, buildapi.GetBuildPodName(build), build.Name)
	}
	if err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *BuildPodController) enqueuePod(pod *kapi.Pod) {
	key, err := kcontroller.KeyFunc(pod)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", pod, err))
		return
	}
	c.queue.Add(key)
}

func (c *BuildPodController) enqueuePodNotFound(namespace, name, buildName string) {
	key := podNotFoundKey{
		Namespace: namespace,
		PodName:   name,
		BuildName: buildName,
	}
	c.queue.Add(key)
}

func (bc *BuildPodController) handleError(err error, key interface{}, pod *kapi.Pod) {
	if err == nil {
		bc.queue.Forget(key)
		return
	}

	if strategy.IsFatal(err) {
		glog.V(2).Infof("Will not retry fatal error for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		bc.queue.Forget(key)
		return
	}

	if bc.queue.NumRequeues(key) < maxRetries {
		glog.V(4).Infof("Retrying pod %s/%s: %v", pod.Namespace, pod.Name, err)
		bc.queue.AddRateLimited(key)
		return
	}

	glog.V(2).Infof("Giving up retrying pod %s/%s: %v", pod.Namespace, pod.Name, err)
	bc.queue.Forget(key)
}

func isBuildPod(pod *kapi.Pod) bool {
	return len(buildutil.GetBuildName(pod)) > 0
}

func (bc *BuildPodController) getBuildForPod(pod *kapi.Pod) (*buildapi.Build, bool, error) {
	if !isBuildPod(pod) {
		return nil, false, fmt.Errorf("cannot get build for pod (%s/%s): pod is not a build pod", pod.Namespace, pod.Name)
	}
	build, err := bc.buildStore.Builds(pod.Namespace).Get(buildutil.GetBuildName(pod))
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err == nil {
		build, err = buildutil.BuildDeepCopy(build)
	}
	return build, true, err
}
