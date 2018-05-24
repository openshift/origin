package support

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/apihelpers"
	"k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	imageclientv1 "github.com/openshift/client-go/image/clientset/versioned"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

const (
	// DeploymentHookResultAnnotation indicates the result of the lifecycle hook executing during rollout. The value for this annotation
	// can either be 'success' in case the hook finished successfully or the status message from the failed hook pod in case the hook
	// failed to run. The error will be reported regardless of failure policy.
	// This annotation should be used in CI systems or tests so they don't have to rely on deployer pod logs.
	DeploymentHookResultAnnotation = "openshift.io/deployment-hook-result"

	// DeploymentHookResultSuccess indicates the lifecycle hook succeeded without error.
	DeploymentHookResultSuccess = "success"

	DeploymentHookTypePre  = "pre"
	DeploymentHookTypeMid  = "mid"
	DeploymentHookTypePost = "post"
)

type HookExecutor interface {
	RunHook(hook *appsapi.LifecycleHook, controller *corev1.ReplicationController, stage string, timeout time.Duration) error
}

// LifecycleHookController operates the execution and lifecycle of deployment lifecycle hooks.
type LifecycleHookController struct {
	podClient   coreclient.PodsGetter
	kubeClient  kubernetes.Interface
	imageClient imageclientv1.Interface

	recorder record.EventRecorder
	out      io.Writer
}

var (
	HookPodTimeoutError     = fmt.Errorf("timeout while waiting for the hook pod to start")
	DeployerPodTimeoutError = fmt.Errorf("timeout while waiting for the deployer pod to start")
)

func NewLifecycleHookController(kubeclientset kubernetes.Interface, imageClient imageclientv1.Interface, out io.Writer) *LifecycleHookController {
	c := &LifecycleHookController{
		kubeClient:  kubeclientset,
		imageClient: imageClient,
		out:         out,
	}

	eventBroadcaster := record.NewBroadcaster()
	if glog.V(5) {
		eventBroadcaster.StartLogging(glog.Infof)
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})

	c.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "deployer"})

	return c
}

func (c *LifecycleHookController) RunHook(hook *appsapi.LifecycleHook, rc *corev1.ReplicationController, hookType string, timeout time.Duration) error {
	if len(hook.TagImages) > 0 {
		h := &lifecycleTagHook{
			imageClient: c.imageClient,
			hookType:    hookType,
			out:         c.out,
		}
		return h.run(hook, rc)
	}

	h := &lifecyclePodHook{
		rcName:            rc.Name,
		rcNamespace:       rc.Namespace,
		podClient:         c.kubeClient.CoreV1().Pods(rc.Namespace),
		hookPodName:       apihelpers.GetPodName(rc.Name, hookType),
		out:               c.out,
		hookType:          hookType,
		deployerStarted:   make(chan struct{}),
		hookPodStarted:    make(chan struct{}),
		hookCompleteChan:  make(chan struct{}),
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pods"),
		outputWriterMutex: sync.Mutex{},
	}

	// Start new filtered informer for all deployer managed pods
	informer := coreinformers.NewFilteredPodInformer(c.kubeClient, rc.Namespace, 60*time.Second, cache.Indexers{}, func(options *metav1.ListOptions) {
		options.LabelSelector = labels.SelectorFromSet(map[string]string{appsapi.DeployerPodForDeploymentLabel: rc.Name}).String()
	})

	h.podLister = v1.NewPodLister(informer.GetIndexer())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    h.enqueue,
		UpdateFunc: func(old, new interface{}) { h.enqueue(new) },
	})

	stopChan := make(chan struct{})
	defer close(stopChan)

	go informer.Run(stopChan)
	defer h.workqueue.ShutDown()

	// TODO: Should probably add timeout to prevent this hanging when the caches cannot be synced
	if !cache.WaitForCacheSync(stopChan, informer.HasSynced) {
		return h.error("wait for cache sync failed")
	}

	// Run just one worker as we want to avoid parallel processing
	go wait.Until(h.runWorker, time.Second, stopChan)

	// Block until we know the the deployer pod has started
	deployerStartTime, err := h.waitForDeployerPodStarted(timeout, h.deployerStarted)
	if err != nil {
		return h.error(err.Error())
	}

	// We need deployerStartTime so we can set the ActiveDeadlineSeconds for the hook pod which should not account
	// for the time deployer pod took to start.
	if err := h.run(hook, rc, *deployerStartTime); err != nil {
		if err == HookPodExists {
			return nil
		}
		return h.error(err.Error())
	}

	// Block until we know the the hook pod has started
	if err := h.waitForHookPodStarted(timeout, h.hookPodStarted); err != nil {
		return h.error(err.Error())
	}

	// Wait until the hook pod reaches the final state
	select {
	case <-h.hookCompleteChan:
	case <-time.After(timeout):
		return HookPodTimeoutError
	}

	// If hook failed, report the result into replication controller regardless of the failure policy.
	defer func() {
		if err := c.recordHookResult(rc, h); err != nil {
			glog.Errorf("Failed to record hook status back to replication controller: %v", err)
		}
	}()

	if err := h.getHookError(); err == nil {
		eventMessage := fmt.Sprintf("The %s-hook for rollout %s/%s completed successfully", hookType, rc.Namespace, rc.Name)
		c.recorder.Eventf(rc, corev1.EventTypeNormal, "Completed", eventMessage)
		return nil
	}

	switch hook.FailurePolicy {
	case appsapi.LifecycleHookFailurePolicyAbort, appsapi.LifecycleHookFailurePolicyRetry:
		// If the hook and the policy is abort on failure, report the failure and error out
		eventMessage := fmt.Sprintf("The %s-hook failed: %v, rollout of %s/%s will now abort", hookType, h.hookFailureError, rc.Namespace, rc.Name)
		c.recorder.Eventf(rc, corev1.EventTypeWarning, "Failed", eventMessage)
		return fmt.Errorf("the %s hook failed: %v, aborting rollout of %s/%s", hookType, h.hookFailureError, rc.Namespace, rc.Name)
	case appsapi.LifecycleHookFailurePolicyIgnore:
		// If hook failed, but policy is ignore failures, report the failure but succeed the hook execution
		eventMessage := fmt.Sprintf("The %s-hook failed: %v (ignore), rollout of %s/%s will continue", hookType, h.hookFailureError, rc.Namespace, rc.Name)
		c.recorder.Eventf(rc, corev1.EventTypeWarning, "Failed", eventMessage)
		return nil
	default:
		return h.hookFailureError
	}
}

// recordHookResult records the status of the lifecycle hooks back to the replication controller
// This should be used as an alternative to parsing deployer logs in tests.
func (c *LifecycleHookController) recordHookResult(rc *corev1.ReplicationController, hook *lifecyclePodHook) error {
	// Retry fast here as this will likely conflict with the controller making updates to RC
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r, err := c.kubeClient.CoreV1().ReplicationControllers(rc.Namespace).Get(rc.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		rCopy := r.DeepCopy()
		if rCopy.Annotations == nil {
			rCopy.Annotations = map[string]string{}
		}
		status := DeploymentHookResultSuccess
		if err := hook.getHookError(); err != nil {
			status = fmt.Sprintf("failed: %v", err)
		}
		rCopy.Annotations[fmt.Sprintf("%s.%s", DeploymentHookResultAnnotation, hook.hookType)] = status
		_, err = c.kubeClient.CoreV1().ReplicationControllers(rc.Namespace).Update(rCopy)
		return err
	})
}
