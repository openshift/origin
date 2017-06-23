package controller

import (
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/config/cmd"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	"github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

// TemplateInstanceController watches for new TemplateInstance objects and
// instantiates the template contained within, using parameters read from a
// linked Secret object.  The TemplateInstanceController instantiates objects
// using its own service account, first verifying that the requester also has
// permissions to instantiate.
type TemplateInstanceController struct {
	config         *rest.Config
	oc             client.Interface
	kc             kclientsetinternal.Interface
	templateclient internalversiontemplate.TemplateInterface

	lister   templatelister.TemplateInstanceLister
	informer cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface
}

// NewTemplateInstanceController returns a new TemplateInstanceController.
func NewTemplateInstanceController(config *rest.Config, oc client.Interface, kc kclientsetinternal.Interface, templateclient internalversiontemplate.TemplateInterface, informer internalversion.TemplateInstanceInformer) *TemplateInstanceController {
	c := &TemplateInstanceController{
		config:         config,
		oc:             oc,
		kc:             kc,
		templateclient: templateclient,
		lister:         informer.Lister(),
		informer:       informer.Informer(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "TemplateInstanceController"),
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	return c
}

// getTemplateInstance returns the TemplateInstance from the shared informer,
// given its key (dequeued from c.queue).
func (c *TemplateInstanceController) getTemplateInstance(key string) (*templateapi.TemplateInstance, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return c.lister.TemplateInstances(namespace).Get(name)
}

// copyTemplateInstance returns a deep copy of a TemplateInstance object.
func (c *TemplateInstanceController) copyTemplateInstance(templateInstance *templateapi.TemplateInstance) (*templateapi.TemplateInstance, error) {
	templateInstanceCopy, err := kapi.Scheme.DeepCopy(templateInstance)
	if err != nil {
		return nil, err
	}

	return templateInstanceCopy.(*templateapi.TemplateInstance), nil
}

// copyTemplate returns a deep copy of a Template object.
func (c *TemplateInstanceController) copyTemplate(template *templateapi.Template) (*templateapi.Template, error) {
	templateCopy, err := kapi.Scheme.DeepCopy(template)
	if err != nil {
		return nil, err
	}

	return templateCopy.(*templateapi.Template), nil
}

// sync is the actual controller worker function.
func (c *TemplateInstanceController) sync(key string) error {
	templateInstance, err := c.getTemplateInstance(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == templateapi.TemplateInstanceReady && condition.Status == kapi.ConditionTrue ||
			condition.Type == templateapi.TemplateInstanceInstantiateFailure && condition.Status == kapi.ConditionTrue {
			return nil
		}
	}

	glog.V(4).Infof("TemplateInstance controller: syncing %s", key)

	templateInstance, err = c.copyTemplateInstance(templateInstance)
	if err != nil {
		return err
	}

	instantiateErr := c.instantiate(templateInstance)
	if instantiateErr != nil {
		glog.V(4).Infof("TemplateInstance controller: instantiate %s returned %v", key, instantiateErr)
	}

	templateInstance.Status.Conditions = templateapi.FilterTemplateInstanceCondition(templateInstance.Status.Conditions, templateapi.TemplateInstanceReady)
	templateInstance.Status.Conditions = templateapi.FilterTemplateInstanceCondition(templateInstance.Status.Conditions, templateapi.TemplateInstanceInstantiateFailure)

	if instantiateErr == nil {
		templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, templateapi.TemplateInstanceCondition{
			Type:               templateapi.TemplateInstanceReady,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Created",
		})

	} else {
		templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, templateapi.TemplateInstanceCondition{
			Type:               templateapi.TemplateInstanceInstantiateFailure,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Failed",
			Message:            instantiateErr.Error(),
		})
	}

	_, err = c.templateclient.TemplateInstances(templateInstance.Namespace).UpdateStatus(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstance status update failed: %v", err))
	}

	return err
}

// Run runs the controller until stopCh is closed, with as many workers as
// specified.
func (c *TemplateInstanceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return
	}

	glog.V(2).Infof("Starting TemplateInstance controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// runWorker repeatedly calls processNextWorkItem until the latter wants to
// exit.
func (c *TemplateInstanceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem reads from the queue and calls the sync worker function.
// It returns false only when the queue is closed.
func (c *TemplateInstanceController) processNextWorkItem() bool {
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

	utilruntime.HandleError(fmt.Errorf("TemplateInstance %v failed with: %v", key, err))
	c.queue.AddRateLimited(key) // avoid hot looping

	return true
}

// enqueue adds a TemplateInstance to c.queue.  This function is called on the
// shared informer goroutine.
func (c *TemplateInstanceController) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

// instantiate instantiates the objects contained in a TemplateInstance.  Any
// parameters for instantiation are contained in the Secret linked to the
// TemplateInstance.
func (c *TemplateInstanceController) instantiate(templateInstance *templateapi.TemplateInstance) error {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return fmt.Errorf("spec.requester.username not set")
	}

	u := &user.DefaultInfo{Name: templateInstance.Spec.Requester.Username}

	var secret *kapi.Secret
	if templateInstance.Spec.Secret != nil {
		if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: templateInstance.Namespace,
			Verb:      "get",
			Group:     kapi.GroupName,
			Resource:  "secrets",
			Name:      templateInstance.Spec.Secret.Name,
		}); err != nil {
			return err
		}

		s, err := c.kc.Core().Secrets(templateInstance.Namespace).Get(templateInstance.Spec.Secret.Name, metav1.GetOptions{})
		secret = s
		if err != nil {
			return err
		}
	}

	template, err := c.copyTemplate(&templateInstance.Spec.Template)
	if err != nil {
		return err
	}

	// We label all objects we create - this is needed by the template service
	// broker.
	if template.ObjectLabels == nil {
		template.ObjectLabels = make(map[string]string)
	}
	template.ObjectLabels[templateapi.TemplateInstanceLabel] = templateInstance.Name

	if secret != nil {
		for i, param := range template.Parameters {
			if value, ok := secret.Data[param.Name]; ok {
				template.Parameters[i].Value = string(value)
				template.Parameters[i].Generate = ""
			}
		}
	}

	if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: templateInstance.Namespace,
		Verb:      "create",
		Group:     templateapi.GroupName,
		Resource:  "templateconfigs",
		Name:      template.Name,
	}); err != nil {
		return err
	}

	glog.V(4).Infof("TemplateInstance controller: creating TemplateConfig for %s/%s", templateInstance.Namespace, templateInstance.Name)

	template, err = c.oc.TemplateConfigs(templateInstance.Namespace).Create(template)
	if err != nil {
		return err
	}

	errs := runtime.DecodeList(template.Objects, kapi.Codecs.UniversalDecoder())
	if len(errs) > 0 {
		return errs[0]
	}

	// We add an OwnerReference to all objects we create - this is also needed
	// by the template service broker for cleanup.
	for _, obj := range template.Objects {
		meta, _ := meta.Accessor(obj)
		ref := meta.GetOwnerReferences()
		ref = append(ref, metav1.OwnerReference{
			APIVersion: templateapiv1.SchemeGroupVersion.String(),
			Kind:       "TemplateInstance",
			Name:       templateInstance.Name,
			UID:        templateInstance.UID,
		})
		meta.SetOwnerReferences(ref)
	}

	bulk := cmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:   client.DefaultMultiRESTMapper(),
			ObjectTyper:  kapi.Scheme,
			ClientMapper: cmd.ClientMapperFromConfig(c.config),
		},
		Op: func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
			if len(info.Namespace) > 0 {
				namespace = info.Namespace
			}
			if namespace == "" {
				return nil, errors.New("namespace was empty")
			}
			if info.Mapping.Resource == "" {
				return nil, errors.New("resource was empty")
			}
			if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
				Namespace: namespace,
				Verb:      "create",
				Group:     info.Mapping.GroupVersionKind.Group,
				Resource:  info.Mapping.Resource,
				Name:      info.Name,
			}); err != nil {
				return nil, err
			}
			return obj, nil
		},
	}

	// First, do all the SARs to ensure the requester actually has permissions
	// to create.
	glog.V(4).Infof("TemplateInstance controller: running SARs for %s/%s", templateInstance.Namespace, templateInstance.Name)

	errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}

	bulk.Op = func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
		// as cmd.Create, but be tolerant to the existence of objects that we
		// created before.
		helper := resource.NewHelper(info.Client, info.Mapping)
		if len(info.Namespace) > 0 {
			namespace = info.Namespace
		}
		createObj, createErr := helper.Create(namespace, false, obj)
		if kerrors.IsAlreadyExists(createErr) {
			obj, err := helper.Get(namespace, info.Name, false)
			if err != nil {
				return createObj, createErr
			}

			meta, err := meta.Accessor(obj)
			if err != nil {
				return createObj, createErr
			}

			if meta.GetLabels()[templateapi.TemplateInstanceLabel] == templateInstance.Name {
				return obj, nil
			}
		}

		return createObj, createErr
	}

	// Second, create the objects, being tolerant if they already exist and are
	// labelled as having previously been created by us.
	glog.V(4).Infof("TemplateInstance controller: creating objects for %s/%s", templateInstance.Namespace, templateInstance.Name)

	errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}
