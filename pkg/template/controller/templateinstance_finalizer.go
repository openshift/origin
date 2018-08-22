package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrs "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/utils/clock"

	templatev1 "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned"
	templateinformer "github.com/openshift/client-go/template/informers/externalversions/template/v1"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"
)

// TemplateInstanceFinalizerController watches for new TemplateInstance objects and
// instantiates the template contained within, using parameters read from a
// linked Secret object.  The TemplateInstanceFinalizerController instantiates objects
// using its own service account, first verifying that the requester also has
// permissions to instantiate.
type TemplateInstanceFinalizerController struct {
	dynamicRestMapper meta.RESTMapper
	client            dynamic.Interface
	templateClient    templateclient.Interface

	lister         templatelister.TemplateInstanceLister
	informerSynced func() bool

	queue workqueue.RateLimitingInterface

	readinessLimiter workqueue.RateLimiter

	clock clock.Clock

	recorder record.EventRecorder
}

// NewTemplateInstanceFinalizerController returns a new TemplateInstanceFinalizerController.
func NewTemplateInstanceFinalizerController(dynamicRestMapper meta.RESTMapper, dynamicClient dynamic.Interface, templateClient templateclient.Interface, informer templateinformer.TemplateInstanceInformer) *TemplateInstanceFinalizerController {
	c := &TemplateInstanceFinalizerController{
		dynamicRestMapper: dynamicRestMapper,
		templateClient:    templateClient,
		client:            dynamicClient,
		lister:            informer.Lister(),
		informerSynced:    informer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "openshift_template_instance_finalizer_controller"),
		readinessLimiter:  workqueue.NewItemFastSlowRateLimiter(5*time.Second, 20*time.Second, 200),
		clock:             clock.RealClock{},
		recorder:          record.NewBroadcaster().NewRecorder(legacyscheme.Scheme, corev1.EventSource{Component: "template-instance-finalizer-controller"}),
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t := obj.(*templatev1.TemplateInstance)
			if t.DeletionTimestamp != nil {
				c.enqueue(t)
			}
		},
		UpdateFunc: func(_, obj interface{}) {
			t := obj.(*templatev1.TemplateInstance)
			if t.DeletionTimestamp != nil {
				c.enqueue(t)
			}
		},
	})

	return c
}

// getTemplateInstance returns the TemplateInstance from the shared informer,
// given its key (dequeued from c.queue).
func (c *TemplateInstanceFinalizerController) getTemplateInstance(key string) (*templatev1.TemplateInstance, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return c.lister.TemplateInstances(namespace).Get(name)
}

// sync is the actual controller worker function.
func (c *TemplateInstanceFinalizerController) sync(key string) error {
	templateInstance, err := c.getTemplateInstance(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if templateInstance.DeletionTimestamp == nil {
		return nil
	}

	needsFinalizing := false
	for _, v := range templateInstance.Finalizers {
		if v == TemplateInstanceFinalizer {
			needsFinalizing = true
			break
		}
	}
	if !needsFinalizing {
		return nil
	}

	glog.V(4).Infof("TemplateInstanceFinalizer controller: syncing %s", key)

	errs := []error{}
	foreground := metav1.DeletePropagationForeground
	deleteOpts := &metav1.DeleteOptions{PropagationPolicy: &foreground}
	for _, o := range templateInstance.Status.Objects {
		glog.V(5).Infof("attempting to delete object: %#v", o)

		gv, err := schema.ParseGroupVersion(o.Ref.APIVersion)
		if err != nil {
			errs = append(errs, fmt.Errorf("error parsing group version %s for object %#v: %v", o.Ref.APIVersion, o, err))
			continue
		}
		gk := schema.GroupKind{
			Group: gv.Group,
			Kind:  o.Ref.Kind,
		}

		// if a resource type is removed, the template instance finalizer will
		// never be able to clean up the template instance since it won't be
		// able to map+delete all child resources that were previously created.
		mapping, err := c.dynamicRestMapper.RESTMapping(gk, gv.Version)
		if err != nil || mapping == nil {
			errs = append(errs, fmt.Errorf("error mapping object %#v: %v", o, err))
			continue
		}

		namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
		namespace := ""
		if namespaced {
			namespace = o.Ref.Namespace
		}

		err = c.client.Resource(mapping.Resource).Namespace(namespace).Delete(o.Ref.Name, deleteOpts)
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error deleting object %#v with mapping %#v: %v", o, mapping, err))
			continue
		}
	}
	if len(errs) > 0 {
		err = kerrs.NewAggregate(errs)
		c.recorder.Eventf(templateInstance, "FinalizerError", "DeletionFailure", err.Error())
		return err
	}

	templateInstanceCopy := templateInstance.DeepCopy()

	newFinalizers := []string{}
	for _, v := range templateInstanceCopy.Finalizers {
		if v == TemplateInstanceFinalizer {
			continue
		}
		newFinalizers = append(newFinalizers, v)
	}
	templateInstanceCopy.Finalizers = newFinalizers

	_, err = c.templateClient.TemplateV1().TemplateInstances(templateInstanceCopy.Namespace).UpdateStatus(templateInstanceCopy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstanceFinalizer update failed: %v", err))
		return err
	}

	return nil
}

// Run runs the controller until stopCh is closed, with as many workers as
// specified.
func (c *TemplateInstanceFinalizerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.V(2).Infof("TemplateInstanceFinalizer controller waiting for cache sync")
	if !cache.WaitForCacheSync(stopCh, c.informerSynced) {
		return
	}

	glog.Infof("Starting TemplateInstanceFinalizer controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.V(2).Infof("Stopping TemplateInstanceFinalizer controller")
}

// runWorker repeatedly calls processNextWorkItem until the latter wants to
// exit.
func (c *TemplateInstanceFinalizerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem reads from the queue and calls the sync worker function.
// It returns false only when the queue is closed.
func (c *TemplateInstanceFinalizerController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil { // for example, success, or the TemplateInstance has gone away
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("TemplateInstanceFinalizer %v failed with: %v", key, err))
	c.queue.AddRateLimited(key) // avoid hot looping

	return true
}

// enqueue adds a TemplateInstance to c.queue.  This function is called on the
// shared informer goroutine.
func (c *TemplateInstanceFinalizerController) enqueue(templateInstance *templatev1.TemplateInstance) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.Add(key)
}
