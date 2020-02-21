package nsfinalizer

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1lister "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type finalizerController struct {
	name          string
	namespaceName string

	namespaceGetter v1.NamespacesGetter
	podLister       corev1listers.PodLister
	dsLister        appsv1lister.DaemonSetLister
	eventRecorder   events.Recorder

	preRunHasSynced []cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

// NewFinalizerController is here because
// When running an aggregated API on the platform, you delete the namespace hosting the aggregated API. Doing that the
// namespace controller starts by doing complete discovery and then deleting all objects, but pods have a grace period,
// so it deletes the rest and requeues. The ns controller starts again and does a complete discovery and.... fails. The
// failure means it refuses to complete the cleanup. Now, we don't actually want to delete the resoruces from our
// aggregated API, only the server plus config if we remove the apiservices to unstick it, GC will start cleaning
// everything. For now, we can unbork 4.0, but clearing the finalizer after the pod and daemonset we created are gone.
func NewFinalizerController(
	namespaceName string,
	kubeInformersForTargetNamespace kubeinformers.SharedInformerFactory,
	namespaceGetter v1.NamespacesGetter,
	eventRecorder events.Recorder,
) *finalizerController {
	fullname := "NamespaceFinalizerController_" + namespaceName
	c := &finalizerController{
		name:          fullname,
		namespaceName: namespaceName,

		namespaceGetter: namespaceGetter,
		podLister:       kubeInformersForTargetNamespace.Core().V1().Pods().Lister(),
		dsLister:        kubeInformersForTargetNamespace.Apps().V1().DaemonSets().Lister(),
		eventRecorder:   eventRecorder.WithComponentSuffix("finalizer-controller"),

		preRunHasSynced: []cache.InformerSynced{
			kubeInformersForTargetNamespace.Core().V1().Pods().Informer().HasSynced,
			kubeInformersForTargetNamespace.Apps().V1().DaemonSets().Informer().HasSynced,
		},
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fullname),
	}

	kubeInformersForTargetNamespace.Core().V1().Pods().Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Apps().V1().DaemonSets().Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c finalizerController) sync() error {
	ns, err := c.namespaceGetter.Namespaces().Get(c.namespaceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if ns.DeletionTimestamp == nil {
		return nil
	}

	// allow one minute of grace for most things to terminate.
	// TODO now that we have conditions, we may be able to check specific conditions
	deletedMoreThanAMinute := ns.DeletionTimestamp.Time.Add(1 * time.Minute).Before(time.Now())
	if !deletedMoreThanAMinute {
		c.queue.AddAfter(c.namespaceName, 1*time.Minute)
		return nil
	}

	pods, err := c.podLister.Pods(c.namespaceName).List(labels.Everything())
	if err != nil {
		return err
	}
	if len(pods) > 0 {
		return nil
	}
	dses, err := c.dsLister.DaemonSets(c.namespaceName).List(labels.Everything())
	if err != nil {
		return err
	}
	if len(dses) > 0 {
		return nil
	}

	newFinalizers := []corev1.FinalizerName{}
	for _, curr := range ns.Spec.Finalizers {
		if curr == corev1.FinalizerKubernetes {
			continue
		}
		newFinalizers = append(newFinalizers, curr)
	}
	if reflect.DeepEqual(newFinalizers, ns.Spec.Finalizers) {
		return nil
	}
	ns.Spec.Finalizers = newFinalizers

	c.eventRecorder.Event("NamespaceFinalization", fmt.Sprintf("clearing namespace finalizer on %q", c.namespaceName))
	_, err = c.namespaceGetter.Namespaces().Finalize(ns)
	return err
}

// Run starts the openshift-apiserver and blocks until stopCh is closed.
func (c *finalizerController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v", c.name)
	defer klog.Infof("Shutting down %v", c.name)

	if !cache.WaitForCacheSync(ctx.Done(), c.preRunHasSynced...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// always kick at least once in case we started after the namespace was cleared
	c.queue.Add(c.namespaceName)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, ctx.Done())

	<-ctx.Done()
}

func (c *finalizerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *finalizerController) processNextWorkItem() bool {
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
func (c *finalizerController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(c.namespaceName) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(c.namespaceName) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(c.namespaceName) },
	}
}
