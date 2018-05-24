package support

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/runtime"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type lifecyclePodHook struct {
	podClient typedcorev1.PodInterface
	podLister listersv1.PodLister

	rcName      string
	rcNamespace string
	hookPodName string

	deployerStarted   chan struct{}
	deployerStartedAt *time.Time

	hookPodStarted   chan struct{}
	hookPodStartedAt *time.Time

	hookType string

	// hookPodRestarts is the pod container restart counter.
	hookPodRestarts int32

	// hookCompleteChan signals when the hook execution is complete
	hookCompleteChan chan struct{}

	// hookFailureError logs the pod status message as error we can report to users and store in status annotation
	hookFailureError error

	// logStreamComplete indicates that the hook pod container log streaming was already started or done
	logStreamCompleted bool

	// outputWriterMutex is used to synchronize writing to output (log messages vs. streaming logs can race)
	outputWriterMutex sync.Mutex
	out               io.Writer

	workqueue workqueue.RateLimitingInterface
	sync.RWMutex
}

// HookPodExists is returned when the hook pod already exists.
var HookPodExists = fmt.Errorf("the hook pod already exists")

func (h *lifecyclePodHook) run(hook *appsapi.LifecycleHook, rc *corev1.ReplicationController, deployerStartTime time.Time) error {
	config, err := appsutil.DecodeDeploymentConfig(rc, legacyscheme.Codecs.UniversalDecoder())
	if err != nil {
		return fmt.Errorf("failed to decode deployment config: %v", err)
	}

	hookPodSpec, err := createHookPodManifest(hook, rc, &config.Spec.Strategy, h.hookType, deployerStartTime)
	if err != nil {
		return fmt.Errorf("failed to create hook pod manifest: %v", err)
	}

	if _, err := h.podClient.Create(hookPodSpec); err != nil {
		if kerrors.IsAlreadyExists(err) {
			// The lifecycle hook pod already exists, which means we already ran it. This usually happen when a custom strategy
			// is used and the openshift-deploy command kicks the hook execution again...
			return HookPodExists
		}
		return fmt.Errorf("failed to create hook pod: %v", err)
	}

	h.logProgress("Running lifecycle hook pod ...")
	return nil
}

func (h *lifecyclePodHook) runWorker() {
	for h.processNextWorkItem() {
	}
}

func (h *lifecyclePodHook) processNextWorkItem() bool {
	obj, shutdown := h.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer h.workqueue.Done(obj)
		key, ok := obj.(string)
		if !ok {
			h.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := h.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		h.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (h *lifecyclePodHook) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	// We only care about deployer pod and hook pod
	switch objMeta.GetName() {
	case appsutil.DeployerPodNameForDeployment(h.rcName):
		h.workqueue.AddRateLimited(key)
	case h.hookPodName:
		h.workqueue.AddRateLimited(key)
	}
}

func (h *lifecyclePodHook) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	pod, err := h.podLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("pod '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	switch pod.Name {
	case appsutil.DeployerPodNameForDeployment(h.rcName):
		h.ensureDeployerPodStarted(pod)
	case h.hookPodName:
		h.syncHookPod(pod)
	}

	return nil
}

func (h *lifecyclePodHook) ensureDeployerPodStarted(pod *corev1.Pod) {
	if h.deployerStartedAt != nil {
		return
	}
	if pod.Status.StartTime != nil {
		h.Lock()
		h.deployerStartedAt = &pod.Status.StartTime.Time
		if h.deployerStarted != nil {
			close(h.deployerStarted)
		}
		h.Unlock()
	}
}

func (h *lifecyclePodHook) ensureHookPodStarted(pod *corev1.Pod) {
	if h.hookPodStartedAt != nil {
		return
	}
	if pod.Status.StartTime != nil {
		h.Lock()
		h.hookPodStartedAt = &pod.Status.StartTime.Time
		if h.hookPodStarted != nil {
			close(h.hookPodStarted)
		}
		h.Unlock()
	}
}

func (h *lifecyclePodHook) syncHookPod(pod *corev1.Pod) {
	// Notify RunHook about startedTime being set on the hook pod
	h.ensureHookPodStarted(pod)

	// Handle hook pod container restarts where we reset the log streaming so we can
	// restart the stream from new container.
	if len(pod.Status.ContainerStatuses) > 0 {
		currentRestartCount := pod.Status.ContainerStatuses[0].RestartCount
		h.Lock()
		if currentRestartCount > h.hookPodRestarts {
			h.logProgress(fmt.Sprintf("Retrying lifecycle hook pod (retry #%d)...", currentRestartCount))
			h.resetStreamingLogs()
		}
		h.hookPodRestarts = currentRestartCount
		h.Unlock()
	}

	// Handle different phases of hook pod
	switch pod.Status.Phase {
	case corev1.PodRunning:
		go func() {
			h.streamHookLogs()
		}()
	case corev1.PodFailed:
		go func() {
			h.streamHookLogs()
			h.logProgress("Lifecycle hook failed")
			h.setHookErrorFromPod(pod)
			close(h.hookCompleteChan)
		}()
	case corev1.PodSucceeded:
		go func() {
			h.streamHookLogs()
			h.logProgress("Lifecycle hook succeeded")
			close(h.hookCompleteChan)
		}()
	}
}

// getHookError returns an error from the hook pod
func (h *lifecyclePodHook) getHookError() error {
	h.RLock()
	defer h.RUnlock()
	return h.hookFailureError
}

// setHookErrorFromPod sets the error from the pod
func (h *lifecyclePodHook) setHookErrorFromPod(pod *corev1.Pod) {
	h.Lock()
	defer h.Unlock()
	h.hookFailureError = fmt.Errorf("%s", pod.Status.Message)
}

// resetStreamingLogs resets the log streaming state, allowing to restart the log streaming when needed.
// For example when the container running in hook restarts.
func (h *lifecyclePodHook) resetStreamingLogs() {
	h.outputWriterMutex.Lock()
	defer h.outputWriterMutex.Unlock()
	h.logStreamCompleted = false
}

func (h *lifecyclePodHook) streamHookLogs() {
	// Take the output write mutex so nothing else will attempt to write to output until the log streaming finish
	h.outputWriterMutex.Lock()
	defer h.outputWriterMutex.Unlock()

	if h.logStreamCompleted {
		return
	}
	h.logStreamCompleted = true

	logStream, err := streamFn(h.podClient, h.hookPodName)
	if err != nil || logStream == nil {
		if logStream == nil {
			err = fmt.Errorf("log stream is empty")
		}
		glog.Errorf("Warning: Unable to get logs from hook pod %s: %v", h.hookPodName, err)
		// We will retry when pod is completed
		h.logStreamCompleted = false
		return
	}

	defer logStream.Close()
	if _, err := io.Copy(h.out, logStream); err != nil {
		// We will retry when pod is completed
		glog.Errorf("Warning: Unable to stream logs from hook pod %s: %v", h.hookPodName, err)
		h.logStreamCompleted = false
		return
	}
}

func (h *lifecyclePodHook) error(message string) error {
	return fmt.Errorf("ERROR %s-hook: %s", h.hookType, message)
}

func (h *lifecyclePodHook) logProgress(message string) {
	h.outputWriterMutex.Lock()
	defer h.outputWriterMutex.Unlock()
	fmt.Fprintf(h.out, "--> %s: %s\n", h.hookType, message)
}

// streamFn encapsulates the streaming of logs so we can mock this in unit tests
var streamFn = func(client typedcorev1.PodInterface, hookPodName string) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container:  "lifecycle",
		Follow:     true,
		Timestamps: false,
	}
	return client.GetLogs(hookPodName, opts).Stream()
}

// waitForDeployerPodStarted waits until the deployer pod has the startedTime set by kubelet or timeouts.
func (h *lifecyclePodHook) waitForDeployerPodStarted(timeout time.Duration, startChan chan struct{}) (*time.Time, error) {
	h.workqueue.Add(h.rcNamespace + "/" + appsutil.DeployerPodNameForDeployment(h.rcName))
	select {
	case <-startChan:
		return h.deployerStartedAt, nil
	case <-time.After(timeout):
		close(startChan)
		return nil, DeployerPodTimeoutError
	}
}

// waitForHookPodStarted waits until the hook pod has the startedTime set by kubelet or timeouts.
func (h *lifecyclePodHook) waitForHookPodStarted(timeout time.Duration, startChan chan struct{}) error {
	select {
	case <-startChan:
		return nil
	case <-time.After(timeout):
		close(startChan)
		return HookPodTimeoutError
	}
}
