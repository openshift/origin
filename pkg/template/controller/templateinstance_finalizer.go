package controller

import (
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrs "k8s.io/apimachinery/pkg/util/errors"
	//	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	//	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	//	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	//	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/utils/clock"

	"github.com/golang/glog"

	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	//	"github.com/openshift/origin/pkg/bulk"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
	restutil "github.com/openshift/origin/pkg/util/rest"
)

// TemplateInstanceFinalizerController watches for new TemplateInstance objects and
// instantiates the template contained within, using parameters read from a
// linked Secret object.  The TemplateInstanceFinalizerController instantiates objects
// using its own service account, first verifying that the requester also has
// permissions to instantiate.
type TemplateInstanceFinalizerController struct {
	restmapper        meta.RESTMapper
	dynamicRestMapper *discovery.DeferredDiscoveryRESTMapper
	config            *rest.Config
	jsonConfig        *rest.Config
	templateClient    templateclient.Interface

	// FIXME: Remove then cient when the build configs are able to report the
	//				status of the last build.
	buildClient buildclient.Interface

	kc kclientsetinternal.Interface

	lister   templatelister.TemplateInstanceLister
	informer cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface

	readinessLimiter workqueue.RateLimiter

	clock clock.Clock
}

// NewTemplateInstanceFinalizerController returns a new TemplateInstanceFinalizerController.
func NewTemplateInstanceFinalizerController(dynamicRestMapper *discovery.DeferredDiscoveryRESTMapper, config *rest.Config, kc kclientsetinternal.Interface, buildClient buildclient.Interface, templateClient templateclient.Interface, informer internalversion.TemplateInstanceInformer) *TemplateInstanceFinalizerController {
	c := &TemplateInstanceFinalizerController{
		restmapper:       restutil.DefaultMultiRESTMapper(),
		config:           config,
		kc:               kc,
		templateClient:   templateClient,
		buildClient:      buildClient,
		lister:           informer.Lister(),
		informer:         informer.Informer(),
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "openshift_template_instance_controller"),
		readinessLimiter: workqueue.NewItemFastSlowRateLimiter(5*time.Second, 20*time.Second, 200),
		clock:            clock.RealClock{},
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t := obj.(*templateapi.TemplateInstance)
			if t.DeletionTimestamp != nil {
				c.enqueue(t)
			}
		},
		UpdateFunc: func(_, obj interface{}) {
			t := obj.(*templateapi.TemplateInstance)
			if t.DeletionTimestamp != nil {
				c.enqueue(t)
			}
		},
	})

	c.jsonConfig = rest.CopyConfig(c.config)
	c.jsonConfig.ContentConfig = dynamic.ContentConfig()

	return c
}

// getTemplateInstance returns the TemplateInstance from the shared informer,
// given its key (dequeued from c.queue).
func (c *TemplateInstanceFinalizerController) getTemplateInstance(key string) (*templateapi.TemplateInstance, error) {
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

	needsFinalizing := false
	for _, v := range templateInstance.Finalizers {
		if v == templateapi.TemplateInstanceFinalizer {
			needsFinalizing = true
			break
		}
	}
	if !needsFinalizing {
		return nil
	}

	glog.V(4).Infof("TemplateInstanceFinalizer controller: syncing %s", key)

	templatePtr := &templateInstance.Spec.Template
	template := templatePtr.DeepCopy()

	errs := runtime.DecodeList(template.Objects, legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: "", Version: "v1"}), unstructured.UnstructuredJSONScheme)
	if len(errs) > 0 {
		return kerrs.NewAggregate(errs)
	}

	errs = []error{}
	background := metav1.DeletePropagationBackground
	deleteOpts := &metav1.DeleteOptions{PropagationPolicy: &background}
	clientPool := dynamic.NewDynamicClientPool(c.jsonConfig)
	for _, o := range templateInstance.Status.Objects {
		gv, err := schema.ParseGroupVersion(o.Ref.APIVersion)
		if err != nil {
			errs = append(errs, fmt.Errorf("error deleting object %v: %v", o, err))
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   gv.Group,
			Version: gv.Version,
			Kind:    o.Ref.Kind,
		}
		client, err := clientPool.ClientForGroupVersionKind(gvk)
		if err != nil {
			errs = append(errs, fmt.Errorf("error deleting object %v: %v", o, err))
			continue
		}
		apiResource := &metav1.APIResource{Name: strings.ToLower(o.Ref.Kind) + "s"}
		err = client.Resource(apiResource, o.Ref.Namespace).Delete(o.Ref.Name, deleteOpts)
		if err != nil {
			errs = append(errs, fmt.Errorf("error deleting object %v: %v", o, err))
			continue
		}

		/*
			meta, _ := meta.Accessor(template.Objects[o.Index])
			// template object's name+namespace may have been generated or parameterized, so we need
			// the actual value from the object we created
			meta.SetName(o.Ref.Name)
			meta.SetNamespace(o.Ref.Namespace)
		*/
	}
	if len(errs) > 0 {
		return kerrs.NewAggregate(errs)
	}
	/*
		bulk := bulk.Bulk{
			Mapper: &resource.Mapper{
				RESTMapper:   c.restmapper,
				ObjectTyper:  legacyscheme.Scheme,
				ClientMapper: bulk.ClientMapperFromConfig(c.config),
			},
			DynamicMapper: &resource.Mapper{
				RESTMapper:   c.dynamicRestMapper,
				ObjectTyper:  discovery.NewUnstructuredObjectTyper(nil),
				ClientMapper: bulk.ClientMapperFromConfig(c.jsonConfig),
			},

			Op: func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {

				helper := resource.NewHelper(info.Client, info.Mapping)
				s := metav1.DeletePropagationForeground
				deleteErr := helper.DeleteWithOptions(info.Namespace, info.Name, &metav1.DeleteOptions{PropagationPolicy: &s})

				if deleteErr != nil {
					return nil, deleteErr
				}
				return nil, nil
			},
		}

		// We do not SAR check the deletion, should we?  When this was done via GC there was
		// no SAR checking so this retains compatibility w/ that behavior.
		errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
		if len(errs) > 0 {
			return utilerrors.NewAggregate(errs)
		}
	*/
	newFinalizers := []string{}
	for _, v := range templateInstance.Finalizers {
		if v == templateapi.TemplateInstanceFinalizer {
			continue
		}
		newFinalizers = append(newFinalizers, v)
	}
	templateInstance.Finalizers = newFinalizers

	_, err = c.templateClient.Template().TemplateInstances(templateInstance.Namespace).UpdateStatus(templateInstance)
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

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return
	}

	glog.V(2).Infof("Starting TemplateInstanceFinalizer controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	go wait.Until(c.dynamicRestMapper.Reset, 30*time.Second, stopCh)

	<-stopCh
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
func (c *TemplateInstanceFinalizerController) enqueue(templateInstance *templateapi.TemplateInstance) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.Add(key)
}

// enqueueAfter adds a TemplateInstance to c.queue after a duration.
func (c *TemplateInstanceFinalizerController) enqueueAfter(templateInstance *templateapi.TemplateInstance, duration time.Duration) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.AddAfter(key, duration)
}
